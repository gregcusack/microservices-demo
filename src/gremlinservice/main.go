package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/ishell"
	"github.com/fatih/color"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"go.uber.org/zap"
	st "google.golang.org/grpc/status"
)

var (
	jaegerAddr string
	faultAddr  string

	zLogger *zap.Logger
	sugar   *zap.SugaredLogger
)

func init() {
	flag.StringVar(&jaegerAddr, "jaeger", "cs1380.cs.brown.edu:5000", "address of jaeger service")
	flag.StringVar(&faultAddr, "fault", "cs1380.cs.brown.edu:5000", "address of fault service")

	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()
}

func main() {
	flag.Parse()

	defer zLogger.Sync()

	// Create jaeger service client
	jc := NewJaegerClient(jaegerAddr)

	// Create fault service client
	fc := NewFaultServiceClient(faultAddr)

	// Create kiali service client
	kc := NewKialiClient()

	// Create client shell
	shell := ishell.New()

	shell.AddCmd(&ishell.Cmd{
		Name: "analyze",
		Func: func(c *ishell.Context) {
			dirs, err := readDir(filepath.Join("data", "chunks"))
			if err != nil {
				c.Err(err)
				return
			}
			dates := func() (res []string) {
				for _, dir := range dirs {
					i, err := strconv.ParseInt(dir, 10, 64)
					if err != nil {
						sugar.Fatal(err)
					}
					res = append(res, time.Unix(i, 0).String())
				}
				return
			}()

			index := c.MultiChoice(dates, "Choose experiment to analyze:")
			id := dirs[index]

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
		},
		Help: "analyzes the last experiment results",
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "start",
		Func: func(c *ishell.Context) {
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
		},
		Help: "starts fault injection experiment",
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "query",
		Func: func(c *ishell.Context) {
			if len(c.Args) != 1 {
				c.Println("Usage: query <service>")
				return
			}
			svc := c.Args[0]
			resp, err := jc.QueryOperations(svc)
			if err != nil {
				c.Err(err)
				return
			}

			var ops []string

			// Get all Recv operations
			for _, op := range resp.GetOperationNames() {
				if strings.Contains(strings.ToLower(op), "recv") {
					ops = append(ops, op)
				}
			}

			parentDir := filepath.Join("data", "traces", strconv.FormatInt(time.Now().Unix(), 10))

			for _, op := range ops {
				sugar.Infof("Getting traces for svc: %v, operation: %v", svc, op)
				traces, err := jc.QueryTraces(svc, op, time.Now().Add(-30*time.Second))
				if err != nil {
					c.Err(err)
					return
				}

				op = strings.Replace(op, "/", "_", -1)
				childDir := filepath.Join(parentDir, op)
				if err := os.MkdirAll(childDir, 0755); err != nil {
					c.Err(err)
					return
				}

				for traceID, spans := range traces {
					chunk := &api_v2.SpansResponseChunk{Spans: spans}
					if err := writeChunksToFile(chunk, filepath.Join(childDir, traceID)); err != nil {
						c.Err(err)
						return
					}
				}
			}
		},
		Help: "Queries a services operations",
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "dag",
		Func: func(c *ishell.Context) {
			path := filepath.Join("data", "traces")
			names, err := readDir(path)
			if err != nil {
				c.Err(err)
				return
			}
			index := c.MultiChoice(names, "Choose a time to examine:")
			t := names[index]
			path = filepath.Join(path, t)
			names, err = readDir(path)
			if err != nil {
				c.Err(err)
				return
			}

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
					s.WriteString(d.exportDag(processed, vLabels, eLabels))
					processed++
				}
			}

			s.WriteString("t # -1\n")

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

			sugar.Infof("Beginning subgraph mining...")

			input := filepath.Join("data", "graphs", t)
			output := filepath.Join("data", "graphs", fmt.Sprintf("%v_result", t))

			cmd := exec.Command(
				"sh",
				"mine.sh",
				input,
				output,
			)
			cmd.CombinedOutput()

			sugar.Infof("Finished subgraph mining")
		},
		Help: "Creates dag out of trace",
	})

	// print shell help
	fmt.Print(shell.HelpText())

	// Start shell
	shell.Run()
}

func readDir(path string) (names []string, err error) {
	var dirs []os.FileInfo
	dirs, err = ioutil.ReadDir(path)
	if err != nil {
		return
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].IsDir() != dirs[j].IsDir() {
			return dirs[i].IsDir()
		}
		return dirs[i].Name() > dirs[j].Name()
	})

	for _, d := range dirs {
		names = append(names, d.Name())
	}

	return
}
