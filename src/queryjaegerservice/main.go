package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"go.uber.org/zap"
	"log"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	istio "istio.io/client-go/pkg/clientset/versioned"
)

var (
	addr       string
	kubeconfig string
	incluster  bool
	logger *zap.Logger
	sugar *zap.SugaredLogger
)

func init() {
	flag.StringVar(&addr, "addr", "cs1380.cs.brown.edu:5000", "addr for jaeger-query")
	flag.BoolVar(&incluster, "incluster", false, "toggle if executable is in k8s cluster")
	if home := homeDir(); home != "" {
		flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}

	logger, _ = zap.NewProduction()
	sugar = logger.Sugar()
}

func setupk8s() (*istio.Clientset, error) {
	if !incluster {
		return setupOutCluster(kubeconfig)
	}
	return setupInCluster()
}

func main() {
	flag.Parse()

	defer logger.Sync()

	sugar.Info("Setting up k8s...")
	ic, err := setupk8s()
	if err != nil {
		glog.Errorf("k8s error: %v\n", err.Error())
	}

	// Dial conn to jaeger query
	sugar.Info("Dialing jaeger query address...")
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}

	csvDir := filepath.Join("data", "csv")

	// 1. Based on csv data, choose services that will have fault injection.
	// 	  For now, only most frequent service will be fault injected
	records, err := readCSV(filepath.Join(csvDir, "services"))
	if err != nil {
		log.Fatal(err)
	}

	row := records[0]
	faultSvc := row["service"]

	// 2. Find upstream services for to-be fault injected services. This includes
	// 	  all upstream services of those who are immediately upstream of to-be fault
	//	  injected service, and so on.
	records, err = readCSV(filepath.Join(csvDir, "edges"))
	if err != nil {
		log.Fatal(err)
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

	fmt.Printf("upstream svcs: %v\n", upstreamSvcs)

	// 3. Get traces for upstream services before fault injection for last 30 seconds
	sugar.Info("Querying chunks...")
	chunks, err := queryChunks(cc, upstreamSvcs, time.Now().Add(-30*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	// 4. Measure stats for upstream services' traces
	beforeNodes, err := measureSuccessRate(chunks)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(beforeNodes)

	// 5. Apply fault injection yaml
	sugar.Info("Appyling fault injection...")
	if err := applyFaultInjection(ic, faultSvc); err != nil {
		log.Fatal(err)
	}

	// 6. Wait 30 seconds
	sugar.Info("Waiting 30 seconds for experiment to run...")
	time.Sleep(30 * time.Second)

	// 7. Measure traces for upstream services after fault injection for last 30 seconds
	sugar.Info("Querying chunks...")
	chunks, err = queryChunks(cc, upstreamSvcs, time.Now().Add(-30*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	// 8. Remove fault injection
	sugar.Info("Deleting fault injection...")
	if err := deleteFaultInjection(ic, faultSvc); err != nil {
		log.Fatal(err)
	}

	// 9. Analyze results
	afterNodes, err := measureSuccessRate(chunks)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(afterNodes)
}
