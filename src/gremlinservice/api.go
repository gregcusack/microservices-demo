package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/ishell"
	"github.com/fatih/color"
)

func analyze(c *ishell.Context) {
	// Choose chunks folder to analyze
	dirs, err := readDir(filepath.Join("data", "experiments"))
	if err != nil {
		c.Err(err)
		return
	}

	index := c.MultiChoice(dirs, "Choose experiment to analyze:")
	id := dirs[index]

	// Reply all chunks from an experiment
	before, after, err := replayChunks(filepath.Join("data", "experiments", id))
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

	// Get upstream services of fault svc
	upstreamSvcs, err := getUpstreamSvcs(faultSvc)
	if err != nil {
		c.Err(err)
		return
	}

	// Get percentage error rate
	percent := getFailureRate(c)

	// Measure success rate
	beforeGraph, err := measureSuccess(id, Before, upstreamSvcs)
	if err != nil {
		c.Err(err)
		return
	}
	// Apply fault injection
	if err := applyFault(faultSvc, "/", percent); err != nil {
		c.Err(err)
		return
	}
	// Wait 30 seconds
	sugar.Info("Waiting 30 seconds for experiment to run...")
	time.Sleep(30 * time.Second)

	// Measure success rate
	afterGraph, err := measureSuccess(id, After, upstreamSvcs)
	if err != nil {
		c.Err(err)
		return
	}
	// Delete fault injection
	if err := deleteFault(faultSvc); err != nil {
		c.Err(err)
		return
	}
	// Measure delta
	analyzeResults(beforeGraph, afterGraph)
}

func continueExperiment(c *ishell.Context) {
	path := filepath.Join("data", "experiments")

	names, err := readDir(path)
	if err != nil {
		c.Err(err)
		return
	}
	// Choose an experiment
	index := c.MultiChoice(names, "Choose an experiment to continue:")
	id := names[index]
	path = filepath.Join(path, id)
	// Mine workflows from graph data
	if err := mineWorkflows(path); err != nil {
		c.Err(err)
		return
	}
	// Choose fault service
	faultSvc, faultUri, err := chooseFault(path, c)
	if err != nil {
		c.Err(err)
		return
	}
	// Get upstream services of fault svc
	upstreamSvcs, err := getUpstreamSvcs(faultSvc)
	if err != nil {
		c.Err(err)
		return
	}
	// Get fault percentage
	percent := getFailureRate(c)
	// Measure success rate
	beforeGraph, err := measureSuccess(path, Before, upstreamSvcs)
	if err != nil {
		c.Err(err)
		return
	}
	// Apply fault injection
	if err := applyFault(faultSvc, faultUri, percent); err != nil {
		c.Err(err)
		return
	}
	// Wait 30 seconds
	sugar.Info("Waiting 30 seconds for experiment to run...")
	time.Sleep(30 * time.Second)
	// Measure success rate
	afterGraph, err := measureSuccess(path, After, upstreamSvcs)
	if err != nil {
		c.Err(err)
		return
	}
	// Delete fault injection
	if err := deleteFault(faultSvc); err != nil {
		c.Err(err)
		return
	}
	// Measure delta
	analyzeResults(beforeGraph, afterGraph)
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
	if err := queryService(filepath.Join(path, "traces"), "frontend"); err != nil {
		c.Err(err)
		return
	}
	// Create graph file from traces
	if err := convertTracesToDags(path); err != nil {
		c.Err(err)
		return
	}
	// Mine workflows from graph data
	if err := mineWorkflows(path); err != nil {
		c.Err(err)
		return
	}
	// Choose fault service
	faultSvc, faultUri, err := chooseFault(path, c)
	if err != nil {
		c.Err(err)
		return
	}
	// Get upstream services of fault svc
	upstreamSvcs, err := getUpstreamSvcs(faultSvc)
	if err != nil {
		c.Err(err)
		return
	}
	// Get fault percentage
	percent := getFailureRate(c)
	// Measure success rate
	beforeGraph, err := measureSuccess(path, Before, upstreamSvcs)
	if err != nil {
		c.Err(err)
		return
	}
	// Apply fault injection
	if err := applyFault(faultSvc, faultUri, percent); err != nil {
		c.Err(err)
		return
	}
	// Wait 30 seconds
	sugar.Info("Waiting 30 seconds for experiment to run...")
	time.Sleep(30 * time.Second)
	// Measure success rate
	afterGraph, err := measureSuccess(path, After, upstreamSvcs)
	if err != nil {
		c.Err(err)
		return
	}
	// Delete fault injection
	if err := deleteFault(faultSvc); err != nil {
		c.Err(err)
		return
	}
	// Measure delta
	analyzeResults(beforeGraph, afterGraph)
}
