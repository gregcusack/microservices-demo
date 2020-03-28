// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"

	pb "github.com/triplewy/microservices-demo/src/recommendationservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"contrib.go.opencensus.io/exporter/jaeger"
	mapset "github.com/deckarep/golang-set"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	port        string
	catalogAddr string
	cc          *grpc.ClientConn

	zLogger *zap.Logger
	sugar   *zap.SugaredLogger
)

func init() {
	port = "8080"
	catalogAddr = "localhost:3550"

	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	if os.Getenv("PRODUCT_CATALOG_SERVICE_ADDR") != "" {
		catalogAddr = os.Getenv("PRODUCT_CATALOG_SERVICE_ADDR")
	}

	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()
}

func main() {
	initTracing()
	flag.Parse()

	defer zLogger.Sync()

	var err error
	cc, err = grpc.Dial(catalogAddr, grpc.WithStatsHandler(&ocgrpc.ClientHandler{
		StartOptions: trace.StartOptions{
			Sampler: trace.AlwaysSample(),
		},
	}), grpc.WithInsecure())
	if err != nil {
		sugar.Errorf("Unable to dial product catalog client: %v\n", err)
	}

	sugar.Infof("starting grpc server at :%s", port)
	run(port)
	select {}
}

func run(port string) string {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		sugar.Fatal(err)
	}
	srv := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	svc := &recommendation{}
	pb.RegisterRecommendationServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)
	go srv.Serve(l)
	return l.Addr().String()
}

func initTracing() {
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	initJaegerTracing()
}

func initJaegerTracing() {
	agentAddr := os.Getenv("JAEGER_AGENT_ADDR")
	if agentAddr == "" {
		sugar.Info("jaeger initialization disabled")
		return
	}
	// Register the Jaeger exporter to be able to retrieve
	// the collected spans.
	exporter, err := jaeger.NewExporter(jaeger.Options{
		AgentEndpoint: agentAddr,
		Process: jaeger.Process{
			ServiceName: "recommendationservice",
		},
	})
	if err != nil {
		sugar.Fatal(err)
	}
	trace.RegisterExporter(exporter)

	sugar.Info("jaeger initialization completed.")
}

type recommendation struct{}

func (r *recommendation) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (r *recommendation) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (r *recommendation) ListRecommendations(ctx context.Context, req *pb.ListRecommendationsRequest) (*pb.ListRecommendationsResponse, error) {
	maxResponses := 5
	// fetch list of products from product catalog stub
	client := pb.NewProductCatalogServiceClient(cc)
	resp, err := client.ListProducts(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	// get set difference between all product IDs and request IDs
	allSet := mapset.NewSet()
	for _, p := range resp.GetProducts() {
		allSet.Add(p.GetId())
	}
	filterSet := mapset.NewSet()
	for _, p := range req.GetProductIds() {
		filterSet.Add(p)
	}
	resultSet := allSet.Difference(filterSet)

	filteredProducts := resultSet.ToSlice()
	numProducts := len(filteredProducts)

	numReturn := func() int {
		if maxResponses < numProducts {
			return maxResponses
		}
		return numProducts
	}()

	// sample list of indicies to return
	indices := make([]int, numReturn)
	for i := 0; i < numReturn; i++ {
		indices[i] = i
	}
	rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })

	// fetch product ids from indices
	resultIDs := make([]string, len(indices))
	for i, x := range indices {
		resultIDs[i] = filteredProducts[x].(string)
	}

	sugar.Infof("[Recv ListRecommendations] product_ids=%v\n", resultIDs)

	// build and return response
	return &pb.ListRecommendationsResponse{
		ProductIds: resultIDs,
	}, nil
}
