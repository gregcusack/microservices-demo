package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"go.uber.org/atomic"
	st "google.golang.org/grpc/status"
)

// Query operations of frontend service
func queryFrontend(path string) error {
	resp, err := jc.QueryOperations("frontend")
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
	sugar.Infof("Querying traces for operations: %v", ops)

	// Get and save at most 50 traces for last minute for each operation
	for _, op := range ops {
		traces, err := jc.QueryTraces("frontend", op, time.Now().Add(-60*time.Second), 50)
		if err != nil {
			return err
		}

		if len(traces) == 0 {
			continue
		}

		opPath := filepath.Join(path, "traces", strings.ReplaceAll(op, "/", "_"))

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

// Perform subgraph mining
func chooseFaultSvc(path string) (string, error) {
	sugar.Infof("Starting subgraph mining")
	cmd := exec.Command(
		"sh",
		"mine.sh",
		filepath.Join(path, "traces.data"),
		filepath.Join(path, "traces.result"),
	)

	if err := cmd.Start(); err != nil {
		return "", err
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	cancel := atomic.NewBool(true)

	time.AfterFunc(10 * time.Second, func() {
		if cancel.Load() {
			syscall.Kill(-pgid, 15)
		}
	})

	cmd.Wait()
	
	cancel.Store(false)
	sugar.Infof("Finished subgraph mining")

	subgraphs, err := parseDags(path)
	if err != nil {
		return "", err
	}

	testGraph := subgraphs[0]

	sugar.Infof("top sub-graph: %v", testGraph)

	var faultSvc string
	for _, v := range testGraph.vertices {
		if v.label != "frontend" {
			faultSvc = v.label
			break
		}
	}

	if faultSvc == "" {
		return "", errors.New("could not find a svc to apply fault to")
	}

	sugar.Infof("fault service: %v", faultSvc)
	return faultSvc, nil
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

// Get traces for upstream services for last 30 seconds
func measureSuccess(id string, status status, upstreamSvcs []string) (Graph, error) {
	chunks, err := jc.QueryChunks(id, status, upstreamSvcs, time.Now().Add(-30*time.Second))
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
		color.Red("%#v", graph)
	}

	return graph, nil
}

// Apply fault injection
func applyFault(faultSvc string) error {
	sugar.Info("Applying fault injection...")
	if err := fc.ApplyFault(faultSvc); err != nil {
		s := st.Convert(err)
		if !strings.Contains(s.Message(), "already exists") {
			return err
		}
		if err := fc.DeleteFault(faultSvc); err != nil {
			return err
		}
		if err := fc.ApplyFault(faultSvc); err != nil {
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
	sugar.Info("Stats after fault injection:")
	color.Blue("%#v", after)

	sugar.Info("Delta stats:")
	color.Red("%#v", CalculateDeltas(before, after))
}