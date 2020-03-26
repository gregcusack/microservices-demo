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
	"fmt"
	"net"
	"os"

	"go.uber.org/zap"

	pb "github.com/triplewy/microservices-demo/src/testservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	ot "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	"github.com/openzipkin/zipkin-go/propagation/b3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	zLogger *zap.Logger
	sugar   *zap.SugaredLogger

	cc *grpc.ClientConn

	recommendationAddr string
	port               string
)

func init() {
	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()

	port = "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	recommendationAddr = "192.168.64.9:30871"
	if addr := os.Getenv("RECOMMENDATION_SERVICE_ADDR"); addr != "" {
		recommendationAddr = addr
	}

	var err error
	cc, err = grpc.Dial(recommendationAddr,
		grpc.WithUnaryInterceptor(ot.UnaryClientInterceptor()),
		grpc.WithInsecure(),
	)
	if err != nil {
		sugar.Fatal(err)
	}
}

func main() {
	defer zLogger.Sync()

	defer cc.Close()

	sugar.Infof("server listening on %v", port)
	run(port)
	select {}
}

func run(port string) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		sugar.Fatal(err)
	}
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(
			ot.UnaryServerInterceptor(),
		),
	)
	svc := &service{}
	pb.RegisterTestServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)
	go srv.Serve(l)
}

type service struct{}

func (s *service) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (s *service) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (s *service) Test(ctx context.Context, in *pb.Empty) (*pb.Empty, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if p := b3.ExtractGRPC(&md); p != nil {
			if span, err := p(); err != nil {
				sugar.Errorf("propagation extractor error: %v", err)
			} else {
				sugar.Infof("span context: %#v", span)
			}
		}
	}

	client := pb.NewRecommendationServiceClient(cc)
	resp, err := client.ListRecommendations(ctx, &pb.ListRecommendationsRequest{
		UserId:     "a",
		ProductIds: []string{"6E92ZMYYFZ", "9SIQT8TOJO", "L9ECAV7KIM", "LS4PSXUNUM", "OLJCESPC7Z"},
	})

	if err != nil {
		sugar.Error(err)
		return nil, err
	}

	sugar.Info(resp)

	return &pb.Empty{}, nil
}
