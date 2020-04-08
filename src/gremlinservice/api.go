package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/ishell"
	"github.com/fatih/color"
	st "google.golang.org/grpc/status"
)

func analyze(c *ishell.Context) {
	// Choose chunks folder to analyze
	dirs, err := readDir(filepath.Join("data", "chunks"))
	if err != nil {
		c.Err(err)
		return
	}

	index := c.MultiChoice(dirs, "Choose experiment to analyze:")
	id := dirs[index]

	// Reply all chunks from an experiment
	before, after, err := replayChunks(filepath.Join("data", "chunks", id))
	if err != nil {
		c.Err(err)
		return
	}

	beforeGraph, err := MeasureSuccessRate(before)
	if err != nil {
		c.Err(err)
		return
	}

	// Print the differences
	c.Println("Before fault injection:")
	color.Green("%#v", beforeGraph)

	afterGraph, err := MeasureSuccessRate(after)
	if err != nil {
		c.Err(err)
		return
	}

	c.Println("After fault injection:")
	color.Blue("%#v", afterGraph)

	deltaGraph := CalculateDeltas(beforeGraph, afterGraph)

	c.Println("Deltas:")
	color.Red("%#v", deltaGraph)
}

func start(c *ishell.Context) {
	// Get grpc rates of all service to start experiment with
	rates, err := kc.GetAllTrafficRates()
	if err != nil {
		c.Err(err)
		return
	}

	// Format rates
	var svcs []string
	for svc, rate := range rates {
		svcs = append(svcs, fmt.Sprintf("%v: %.2f req/s", svc, rate))
	}

	index := c.MultiChoice(svcs, "Choose service to start experiment with:")
	faultSvc := strings.Split(svcs[index], ":")[0]

	// Create experiment id (just use unix timestamp for now)
	id := strconv.FormatInt(time.Now().Unix(), 10)

	// 2. Find upstream services for to-be fault injected services. This includes
	// 	  all upstream services of those who are immediately upstream of to-be fault
	//	  injected service, and so on.
	mesh, err := kc.GetMeshOverview()
	if err != nil {
		sugar.Fatal(err)
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

	// 3. Get traces for upstream services before fault injection for last 30 seconds
	chunks, err := jc.QueryChunks(id, Before, upstreamSvcs, time.Now().Add(-30*time.Second))
	if err != nil {
		sugar.Fatal(err)
	}

	// 4. Measure stats for upstream services' traces
	beforeNodes, err := MeasureSuccessRate(chunks)
	if err != nil {
		sugar.Fatal(err)
	}
	sugar.Info("Stats before fault injection:")
	color.Green("%#v", beforeNodes)

	// 5. Apply fault injection
	sugar.Info("Applying fault injection...")
	if err := fc.ApplyFault(faultSvc); err != nil {
		s := st.Convert(err)
		if !strings.Contains(s.Message(), "already exists") {
			sugar.Fatal(err)
		}
		if err := fc.DeleteFault(faultSvc); err != nil {
			sugar.Fatal(err)
		}
		if err := fc.ApplyFault(faultSvc); err != nil {
			sugar.Fatal(err)
		}
	}

	// 6. Wait 30 seconds
	sugar.Info("Waiting 30 seconds for experiment to run...")
	time.Sleep(30 * time.Second)

	// 7. Measure traces for upstream services after fault injection for last 30 seconds
	chunks, err = jc.QueryChunks(id, After, upstreamSvcs, time.Now().Add(-30*time.Second))
	if err != nil {
		sugar.Fatal(err)
	}

	// 8. Delete fault injection
	sugar.Info("Deleting fault injection...")
	if err := fc.DeleteFault(faultSvc); err != nil {
		sugar.Fatal(err)
	}

	// 9. Analyze results
	afterNodes, err := MeasureSuccessRate(chunks)
	if err != nil {
		sugar.Fatal(err)
	}
	sugar.Info("Stats after fault injection:")
	color.Blue("%#v", afterNodes)

	sugar.Info("Delta stats:")
	color.Red("%#v", CalculateDeltas(beforeNodes, afterNodes))
}

func mine(c *ishell.Context) {
	path := filepath.Join("data", "traces")
	names, err := readDir(path)
	if err != nil {
		c.Err(err)
		return
	}

	// Choose a traces from a timeslot
	index := c.MultiChoice(names, "Choose a time to examine:")
	t := names[index]
	path = filepath.Join(path, t)
	names, err = readDir(path)
	if err != nil {
		c.Err(err)
		return
	}

	// Create all vertices and edges for each trace
	// Need to keep global map of vertex labels and edge labels
	vLabels := make(map[string]int, 0)
	eLabels := make(map[string]int, 0)
	s := strings.Builder{}
	processed := 0

	for _, name := range names {
		files, err := readDir(filepath.Join(path, name))
		if err != nil {
			c.Err(err)
			return
		}
		for _, file := range files {
			trace, err := readChunk(filepath.Join(path, name, file))
			if err != nil {
				c.Err(err)
				return
			}
			d := traceToDag(trace.GetSpans())
			result, _ := exportDag(d, processed, vLabels, eLabels)
			s.WriteString(result)
			processed++
		}
	}

	s.WriteString("t # -1\n")

	// Write all vertices and edges to a file
	if err := os.MkdirAll(filepath.Join("data", "graphs"), 0755); err != nil {
		c.Err(err)
		return
	}
	if err := ioutil.WriteFile(filepath.Join("data", "graphs", t), []byte(s.String()), 0644); err != nil {
		c.Err(err)
		return
	}

	var data []byte
	if data, err = json.Marshal(vLabels); err != nil {
		c.Err(err)
		return
	}
	if err := ioutil.WriteFile(filepath.Join("data", "graphs", fmt.Sprintf("%v_vlabels", t)), data, 0644); err != nil {
		c.Err(err)
		return
	}
	if data, err = json.Marshal(eLabels); err != nil {
		c.Err(err)
		return
	}
	if err := ioutil.WriteFile(filepath.Join("data", "graphs", fmt.Sprintf("%v_elabels", t)), data, 0644); err != nil {
		c.Err(err)
		return
	}
	sugar.Infof("Exported traces successfully")

	// Begin subgraph mining
	sugar.Infof("Beginning subgraph mining...")

	input := filepath.Join("data", "graphs", t)
	output := filepath.Join("data", "graphs", fmt.Sprintf("%v_result", t))

	// Create python process that executes subgraph mining
	cmd := exec.Command(
		"sh",
		"mine.sh",
		input,
		output,
	)

	if _, err := cmd.CombinedOutput(); err != nil {
		sugar.Error(err)
	}

	sugar.Infof("Finished subgraph mining")
}

func experiment(c *ishell.Context) {
	// Create experiment id and experiment folder
	id := strconv.FormatInt(time.Now().Unix(), 10)
	path := filepath.Join("data", "experiments", id)
	if err := os.MkdirAll(path, 0755); err != nil {
		c.Err(err)
		return
	}
	sugar.Infof("Starting experiment: %v", id)

	// Query operations of frontend service
	resp, err := jc.QueryOperations("frontend")
	if err != nil {
		c.Err(err)
		return
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
			c.Err(err)
			return
		}

		if len(traces) == 0 {
			continue
		}

		opPath := filepath.Join(path, "traces", strings.ReplaceAll(op, "/", "_"))

		if err := os.MkdirAll(opPath, 0755); err != nil {
			c.Err(err)
			return
		}

		for traceID, trace := range traces {
			if err := writeTraceToFile(trace, filepath.Join(opPath, traceID)); err != nil {
				c.Err(err)
				return
			}
		}

		sugar.Infof("Saved traces for %v", op)
	}
	sugar.Infof("Saved all traces")

	// Create graph file from traces
	if err := tracesToDags(path); err != nil {
		c.Err(err)
		return
	}
	sugar.Infof("Exported traces to graph data")

	// Perform subgraph mining
	sugar.Infof("Starting subgraph mining")
	cmd := exec.Command(
		"sh",
		"mine.sh",
		filepath.Join(path, "traces.data"),
		filepath.Join(path, "traces.result"),
	)

	cmd.CombinedOutput()
	sugar.Infof("Finished subgraph mining")
}