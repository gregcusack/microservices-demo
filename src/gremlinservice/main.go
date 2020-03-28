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
