package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/ishell"
	"github.com/fatih/color"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
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

	// Create client shell
	shell := ishell.New()

	shell.AddCmd(&ishell.Cmd{
		Name: "analyze",
		Func: func(c *ishell.Context) {
			dirs, err := readDir()
			if err != nil {
				c.Err(err)
				return
			}
			dates := func() (res []string) {
				for _, dir := range dirs {
					i, err := strconv.ParseInt(dir.Name(), 10, 64)
					if err != nil {
						sugar.Fatal(err)
					}
					res = append(res, time.Unix(i, 0).String())
				}
				return
			}()

			index := c.MultiChoice(dates, "Choose experiment to analyze:")
			id := dirs[index].Name()

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
			// Create experiment id (just use unix timestamp for now)
			id := strconv.FormatInt(time.Now().Unix(), 10)

			// 1. Based on csv data, choose services that will have fault injection.
			// 	  For now, only most frequent service will be fault injected
			records, err := readCSV(filepath.Join("csv", "services"))
			if err != nil {
				sugar.Fatal(err)
			}

			row := records[0]
			faultSvc := row["service"]

			// 2. Find upstream services for to-be fault injected services. This includes
			// 	  all upstream services of those who are immediately upstream of to-be fault
			//	  injected service, and so on.
			records, err = readCSV(filepath.Join("csv", "edges"))
			if err != nil {
				sugar.Fatal(err)
			}

			// Create reverse graph of microservice mesh
			mesh := make(map[string]map[string]struct{}, 0)
			for _, row := range records {
				start := row["start"]
				end := row["end"]

				if _, ok := mesh[end]; !ok {
					mesh[end] = make(map[string]struct{}, 0)
				}

				mesh[end][start] = struct{}{}
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
				st := status.Convert(err)
				if !strings.Contains(st.Message(), "already exists") {
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
		},
		Help: "starts fault injection experiment",
	})

	// print shell help
	fmt.Print(shell.HelpText())

	// Start shell
	shell.Run()
}

func readDir() (dirs []os.FileInfo, err error) {
	dirs, err = ioutil.ReadDir(filepath.Join("data", "chunks"))
	if err != nil {
		return
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].IsDir() != dirs[j].IsDir() {
			return dirs[i].IsDir()
		}
		return dirs[i].Name() > dirs[j].Name()
	})
	return
}
