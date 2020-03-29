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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"os"
	"path/filepath"

	pb "github.com/triplewy/microservices-demo/src/currencyservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"contrib.go.opencensus.io/exporter/jaeger"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	port string

	zLogger *zap.Logger
	sugar   *zap.SugaredLogger

	conversion map[string]float64
)

func init() {
	port = "7000"

	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()

	data, err := ioutil.ReadFile(filepath.Join("data", "currency_conversion.json"))
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &conversion); err != nil {
		panic(err)
	}
	fmt.Println(conversion)
}

func main() {
	initTracing()
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
	srv := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	svc := &currency{}
	pb.RegisterCurrencyServiceServer(srv, svc)
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
			ServiceName: "currencyservice",
		},
	})
	if err != nil {
		sugar.Fatal(err)
	}
	trace.RegisterExporter(exporter)

	sugar.Info("jaeger initialization completed.")
}

type currency struct{}

func carry(amount *pb.Money) *pb.Money {
	fractionSize := int64(math.Pow10(9))
	amount.Nanos += int32(amount.Units * fractionSize)
	amount.Units = int64(math.Floor(float64(amount.Units)) + math.Floor(float64(amount.Nanos) / float64(fractionSize)))
	amount.Nanos = amount.Nanos % int32(fractionSize)
	return amount
}
func (c *currency) GetSupportedCurrencies(context.Context, *pb.Empty) (*pb.GetSupportedCurrenciesResponse, error) {
	var supportedCodes []string
	for code := range conversion {
		supportedCodes = append(supportedCodes, code)
	}
	return &pb.GetSupportedCurrenciesResponse{
		CurrencyCodes:        supportedCodes,
	}, nil
}

func (c *currency) Convert(ctx context.Context, req *pb.CurrencyConversionRequest) (*pb.Money, error) {
	// Convert: from_currency --> EUR
	from := req.GetFrom()
	euros := carry(&pb.Money{
		CurrencyCode:         "",
		Units:                int64(float64(from.Units) / conversion[from.CurrencyCode]),
		Nanos:                int32(float64(from.Nanos) / conversion[from.CurrencyCode]),
	})

	//// Convert: EUR --> to_currency
	result := carry(&pb.Money{
		Units:                int64(float64(euros.Units) * conversion[req.GetToCode()]),
		Nanos:                int32(float64(euros.Nanos) * conversion[req.GetToCode()]),
	})

	result.CurrencyCode = req.GetToCode()

	return result, nil
}

func (c *currency) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (c *currency) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}
