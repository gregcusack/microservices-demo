package main

import (
	"flag"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
)

var (
	addr string
)

func init() {
	flag.StringVar(&addr, "addr", "cs1380.cs.brown.edu:5000", "addr for jaeger-query")
}

func main() {
	flag.Parse()

	// Dial conn to jaeger query
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

	upstreamSvcsMap := make(map[string]struct{}, 0)
	for _, row := range records {
		if row["end"] == faultSvc {
			upstreamSvcsMap[row["start"]] = struct{}{}
		}
	}

	var upstreamSvcs []string
	for svc := range upstreamSvcsMap {
		upstreamSvcs = append(upstreamSvcs, svc)
	}

	// 3. Get traces for upstream services before fault injection for last 30 seconds
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

	// 4. Create fault injection yaml
	filename, err := createFaultInjection(faultSvc)
	if err != nil {
		log.Fatal(err)
	}

	// 5. Apply fault injection yaml
	cmd := exec.Command("kubectl", "apply", "-f", filename)
	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		log.Fatal(err)
	}

	// 6. Wait 30 seconds
	fmt.Println("waiting 30 seconds for experiment to run...")
	time.Sleep(30 * time.Second)

	// 7. Measure traces for upstream services after fault injection for last 30 seconds
	_, err = queryChunks(cc, upstreamSvcs, time.Now().Add(-30*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	// 8. Remove fault injection
	cmd = exec.Command("kubectl", "delete", "-f", filename)
	out, err = cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		log.Fatal(err)
	}

	// 9. Analyze results

}
