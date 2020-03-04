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
	"os"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"

	pb "github.com/triplewy/microservices-demo/src/testservice/genproto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	log                *logrus.Logger
	recommendationAddr string
	jaegerAddr         string
	cc                 *grpc.ClientConn
)

func init() {
	log = logrus.New()
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
	recommendationAddr = "localhost:8080"
	jaegerAddr = "localhost:14268"
}

func main() {
	initTracing()
	flag.Parse()

	if os.Getenv("RECOMMENDATION_SERVICE_ADDR") != "" {
		recommendationAddr = os.Getenv("RECOMMENDATION_SERVICE_ADDR")
	}

	var err error
	cc, err = grpc.Dial(recommendationAddr, grpc.WithInsecure(), grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
	if err != nil {
		log.Errorf("Unable to dial product catalog client: %v\n", err)
	}
	defer cc.Close()

	for i := 0; i < 5; i++ {
		client := pb.NewRecommendationServiceClient(cc)
		resp, err := client.ListRecommendations(context.Background(), &pb.ListRecommendationsRequest{
			UserId:     "a",
			ProductIds: []string{"6E92ZMYYFZ", "9SIQT8TOJO", "L9ECAV7KIM", "LS4PSXUNUM", "OLJCESPC7Z"},
		})
		if err != nil {
			log.Fatal(err)
		}
		log.Println(resp)
	}

	time.Sleep(1 * time.Second)
}

func initTracing() {
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	initJaegerTracing()
}

func initJaegerTracing() {
	if os.Getenv("JAEGER_SERVICE_ADDR") != "" {
		jaegerAddr = os.Getenv("JAEGER_SERVICE_ADDR")
	}
	// Register the Jaeger exporter to be able to retrieve
	// the collected spans.
	exporter, err := jaeger.NewExporter(jaeger.Options{
		Endpoint: fmt.Sprintf("http://%s", jaegerAddr),
		Process: jaeger.Process{
			ServiceName: "testservice",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)

	log.Info("jaeger initialization completed.")
}
