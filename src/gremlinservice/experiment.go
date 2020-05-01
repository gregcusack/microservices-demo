package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/abiosoft/ishell"

	"github.com/fatih/color"
	"go.uber.org/atomic"
	st "google.golang.org/grpc/status"
)

// Query traces from all Recv operations of a service
func queryService(path, svc string) error {
	resp, err := jc.QueryOperations(svc)
	if err != nil {
		return err
	}

	// Get all Recv operations
	var ops []string
	for _, op := range resp.GetOperationNames() {
		if strings.Contains(strings.ToLower(op), "recv") {
			ops = append(ops, op)
		}
	}

	// Get and save at most 50 traces for last minute for each operation
	for _, op := range ops {
		traces, err := jc.QueryTraces(svc, op, time.Now().Add(-60*time.Second), 30)
		if err != nil {
			return err
		}

		if len(traces) == 0 {
			continue
		}

		opPath := filepath.Join(path, strings.ReplaceAll(op, "/", "_"))

		if err := os.MkdirAll(opPath, 0755); err != nil {
			return err
		}

		for traceID, trace := range traces {
			if err := writeTraceToFile(trace, filepath.Join(opPath, traceID)); err != nil {
				return err
			}
		}

		sugar.Infof("Saved traces for %v", op)
	}
	sugar.Infof("Saved all traces")

	return nil
}

// Create graph file from traces
func convertTracesToDags(path string) error {
	if err := tracesToDags(path); err != nil {
		return err
	}
	sugar.Infof("Exported traces to graph data")
	return nil
}

// Perform subgraph mining on graph data
func mineWorkflows(path string) error {
	sugar.Infof("Starting subgraph mining")
	cmd := exec.Command(
		"sh",
		"mine.sh",
		filepath.Join(path, "traces.data"),
		filepath.Join(path, "traces.result"),
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return err
	}
	cancel := atomic.NewBool(true)

	time.AfterFunc(10*time.Second, func() {
		if cancel.Load() {
			syscall.Kill(-pgid, 15)
		}
	})

	cmd.Wait()

	cancel.Store(false)
	sugar.Infof("Finished subgraph mining")

	return nil
}

// Perform subgraph mining
func chooseFault(path string, c *ishell.Context) (svc, uri string, err error) {
	subgraphs, err := parseDags(path)
	if err != nil {
		return "", "", err
	}
	if len(subgraphs) == 0 {
		return "", "", errors.New("No subgraphs found")
	}

	var options []string
	for _, g := range subgraphs {
		options = append(options, g.GoString())
	}
	index := c.MultiChoice(options, "Select workflow to test:")
	subgraph := subgraphs[index]
	index = c.MultiChoice([]string{"service", "request"}, "Select granularity of experiment:")
	options = make([]string, 0)
	if index == 0 {
		for _, v := range subgraph.vertices {
			if v.label == "frontend" {
				continue
			}
			options = append(options, v.label)
		}
		index = c.MultiChoice(options, "Select service to test:")
		sugar.Infof("fault service: %v", options[index])
		return options[index], "/", nil
	}

	var svcs []string

	for _, e := range subgraph.edges {
		arr := strings.Split(e.label, ".")
		label := fmt.Sprintf("/%v/%v", strings.Join(arr[1:len(arr)-1], "."), arr[len(arr)-1])
		options = append(options, label)
		svcs = append(svcs, strings.ToLower(arr[2]))
	}

	index = c.MultiChoice(options, "Select request to test:")
	sugar.Infof("fault request: %v", options[index])
	return svcs[index], options[index], nil
}

// Find upstream services for to-be fault injected services. This includes
// all upstream services of those who are immediately upstream of to-be fault
// injected service, and so on.
func getUpstreamSvcs(faultSvc string) ([]string, error) {
	mesh, err := kc.GetMeshOverview()
	if err != nil {
		return nil, err
	}

	// Get all upstream services of to-be fault injected service using dfs
	upstreamSvcsMap := make(map[string]struct{}, 0)

	var stack []string
	for node := range mesh[faultSvc] {
		stack = append(stack, node)
	}

	var node string
	for len(stack) > 0 {
		node, stack = stack[0], stack[1:]
		upstreamSvcsMap[node] = struct{}{}
		for n := range mesh[node] {
			stack = append(stack, n)
		}
	}

	var upstreamSvcs []string
	for svc := range upstreamSvcsMap {
		upstreamSvcs = append(upstreamSvcs, svc)
	}

	return upstreamSvcs, nil
}

func getFailureRate(c *ishell.Context) float64 {
	c.Print("Enter failure rate: ")
	in := c.ReadLine()
	rate, err := strconv.ParseFloat(in, 64)
	for err != nil {
		c.Println("Invalid failure rate: ", err)
		return getFailureRate(c)
	}
	c.Printf("Failure rate: %v%%\n", rate)
	return rate
}

// Get traces for upstream services for last 30 seconds
func measureSuccess(path string, status status, upstreamSvcs []string) (Graph, error) {
	chunks, err := jc.QueryChunks(path, status, upstreamSvcs, time.Now().Add(-30*time.Second))
	if err != nil {
		return Graph{}, err
	}

	// Measure stats for upstream services' traces
	graph, err := MeasureSuccessRate(chunks)
	if err != nil {
		return Graph{}, err
	}
	if status == Before {
		sugar.Info("Stats before fault injection:")
		color.Green("%#v", graph)
	} else {
		sugar.Info("Stats after fault injection:")
		color.Blue("%#v", graph)
	}

	return graph, nil
}

// Apply fault injection
func applyFault(svc, uri string, percent float64) error {
	sugar.Info("Applying fault injection...")
	if err := fc.ApplyFault(svc, uri, percent); err != nil {
		s := st.Convert(err)
		if !strings.Contains(s.Message(), "already exists") {
			return err
		}
		if err := fc.DeleteFault(svc); err != nil {
			return err
		}
		if err := fc.ApplyFault(svc, uri, percent); err != nil {
			return err
		}
	}
	return nil
}

// Delete fault injection
func deleteFault(faultSvc string) error {
	sugar.Info("Deleting fault injection...")
	return fc.DeleteFault(faultSvc)
}

func analyzeResults(before, after Graph) {
	sugar.Info("Delta stats:")
	color.Red("%#v", CalculateDeltas(before, after))
}
