package faultservice

import (
	"context"
	"fmt"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"net"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	zLogger *zap.Logger
	sugar   *zap.SugaredLogger
)

func init() {
	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()
}

// Run starts the gRPC server
func Run(port, queryAddr, kubeconfig string) {
	sugar.Infof("starting grpc server at :%s", port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		panic(err)
	}

	srv := grpc.NewServer()
	svc := newServer(queryAddr, kubeconfig)

	RegisterFaultServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)
	go srv.Serve(lis)
	select {}
}

type server struct {
	ic *IstioClient
	jc *JaegerClient
}

func newServer(queryAddr, kubeconfig string) *server {
	sugar.Info("Setting up istio client...")
	ic, err := NewIstioClient(kubeconfig)
	if err != nil {
		sugar.Infof("Error setting up istio client: %v\n", err)
	}
	sugar.Info("Setting up jaeger client...")
	jc, err := NewJaegerClient(queryAddr)
	if err != nil {
		sugar.Infof("Error setting up jaeger client: %v\n", err)
		panic(err)
	}

	return &server{ic: ic, jc: jc}
}

func (s *server) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (s *server) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (s *server) Experiment(req *EmptyMsg, experimentServer FaultService_ExperimentServer) error {
	csvDir := filepath.Join("data", "csv")
	if err := os.MkdirAll(csvDir, 0755); err != nil {
		return err
	}

	// 1. Based on csv data, choose services that will have fault injection.
	// 	  For now, only most frequent service will be fault injected
	records, err := readCSV(filepath.Join(csvDir, "services"))
	if err != nil {
		return err
	}

	row := records[0]
	faultSvc := row["service"]
	experimentServer.Send(&InfoMsg{Info: fmt.Sprintf("Fault svc: %v", faultSvc)})

	// 2. Find upstream services for to-be fault injected services. This includes
	// 	  all upstream services of those who are immediately upstream of to-be fault
	//	  injected service, and so on.
	records, err = readCSV(filepath.Join(csvDir, "edges"))
	if err != nil {
		return err
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

	experimentServer.Send(&InfoMsg{Info: fmt.Sprintf("upstream svcs: %v\n", upstreamSvcs)})

	// 3. Get traces for upstream services before fault injection for last 30 seconds
	experimentServer.Send(&InfoMsg{Info: "Querying chunks before fault injection..."})
	chunks, err := s.jc.QueryChunks(upstreamSvcs, time.Now().Add(-30*time.Second))
	if err != nil {
		return err
	}

	// 4. Measure stats for upstream services' traces
	beforeNodes, err := MeasureSuccessRate(chunks)
	if err != nil {
		return err
	}

	experimentServer.Send(&InfoMsg{Info: fmt.Sprintf("Before fault injection: %#v", beforeNodes)})

	// 5. Apply fault injection yaml
	experimentServer.Send(&InfoMsg{Info: "Appyling fault injection..."})
	if err := s.ic.ApplyFaultInjection(faultSvc); err != nil {
		return err
	}

	// 6. Wait 30 seconds
	experimentServer.Send(&InfoMsg{Info: "Waiting 30 seconds for experiment to run..."})
	time.Sleep(30 * time.Second)

	// 7. Measure traces for upstream services after fault injection for last 30 seconds
	experimentServer.Send(&InfoMsg{Info: "Querying chunks after fault injection..."})
	chunks, err = s.jc.QueryChunks(upstreamSvcs, time.Now().Add(-30*time.Second))
	if err != nil {
		return err
	}

	// 8. Remove fault injection
	experimentServer.Send(&InfoMsg{Info: "Deleting fault injection..."})
	if err := s.ic.DeleteFaultInjection(faultSvc); err != nil {
		return err
	}

	// 9. Analyze results
	afterNodes, err := MeasureSuccessRate(chunks)
	if err != nil {
		return err
	}

	experimentServer.Send(&InfoMsg{Info: fmt.Sprintf("After fault injection: %#v", afterNodes)})

	return nil
}
