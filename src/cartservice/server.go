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
	"net"
	"os"

	pb "github.com/triplewy/microservices-demo/src/cartservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"contrib.go.opencensus.io/exporter/jaeger"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	port string
	redisAddr string

	zLogger *zap.Logger
	sugar   *zap.SugaredLogger
)

func init() {
	port = "7070"
	redisAddr = "localhost:6379"

	if v := os.Getenv("PORT"); v != "" {
		port = v
	}

	if v := os.Getenv("REDIS_ADDR"); v != "" {
		redisAddr = v
	}

	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()
}

func main() {
	//initTracing()
	flag.Parse()

	defer zLogger.Sync()

	sugar.Infof("starting grpc server at :%s", port)
	run(port)
	select {}
}

func run(port string) string {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		sugar.Fatal(err)
	}
	srv := grpc.NewServer()
	svc := &cart{
		redis: newRedisClient(redisAddr),
	}
	pb.RegisterCartServiceServer(srv, svc)
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
			ServiceName: "cartservice",
		},
	})
	if err != nil {
		sugar.Fatal(err)
	}
	trace.RegisterExporter(exporter)

	sugar.Info("jaeger initialization completed.")
}

type cart struct {
	redis *redisClient
}

func (c *cart) AddItem(ctx context.Context, req *pb.AddItemRequest) (*pb.Empty, error) {
	return &pb.Empty{}, c.redis.AddItem(req)
}

func (c *cart) GetCart(ctx context.Context, req *pb.GetCartRequest) (*pb.Cart, error) {
	return c.redis.GetCart(req.GetUserId())
}

func (c *cart) EmptyCart(ctx context.Context, req *pb.EmptyCartRequest) (*pb.Empty, error) {
	return &pb.Empty{}, c.redis.EmptyCart(req.GetUserId())
}

func (c *cart) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (c *cart) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}
