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
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/triplewy/microservices-demo/src/paymentservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"contrib.go.opencensus.io/exporter/jaeger"
	"github.com/satori/go.uuid"
	cardValidator "github.com/sgumirov/go-cards-validation"
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
)

func init() {
	port = "8080"

	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
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
	svc := &payment{}
	pb.RegisterPaymentServiceServer(srv, svc)
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
			ServiceName: "paymentservice",
		},
	})
	if err != nil {
		sugar.Fatal(err)
	}
	trace.RegisterExporter(exporter)

	sugar.Info("jaeger initialization completed.")
}

type payment struct{}

func (p *payment) Charge(ctx context.Context, req *pb.ChargeRequest) (*pb.ChargeResponse, error) {
	amount := req.GetAmount()
	creditCard := req.GetCreditCard()

	card := cardValidator.Card{
		Number: strings.ReplaceAll(creditCard.CreditCardNumber, "-", ""),
		Cvv:    fmt.Sprint(creditCard.CreditCardCvv),
		Month:  fmt.Sprint(creditCard.CreditCardExpirationMonth),
		Year:   fmt.Sprint(creditCard.CreditCardExpirationYear),
	}

	if err := card.Validate(true); err != nil {
		sugar.Error("invalid card number: %v", card)
		return nil, err
	}

	if err := card.Brand(); err != nil {
		sugar.Error("invalid card brand: %v", card)
		return nil, err
	}

	if !(strings.ToLower(card.Company.Name) == "visa" || strings.ToLower(card.Company.Name) == "mastercard") {
		sugar.Error("invalid card company: %v", card)
		return nil, errors.New("unaccepted credit card")
	}

	var cardYear, cardMonth int
	var err error

	if cardYear, err = strconv.Atoi(card.Year); err != nil {
		return nil, err
	}
	if cardMonth, err = strconv.Atoi(card.Month); err != nil {
		return nil, err
	}
	cardExpDate := time.Date(cardYear, time.Month(cardMonth), 0, 0, 0, 0, 0, time.UTC)

	if time.Now().Unix() > cardExpDate.Unix() {
		return nil, errors.New("expired credit card")
	}

	var lastFour string
	if lastFour, err = card.LastFour(); err != nil {
		return nil, err
	}

	sugar.Infof("Transaction processed: %v ending %v Amount: %v%v.%v", card.Company.Name, lastFour, amount.CurrencyCode, amount.Units, amount.Nanos)

	return &pb.ChargeResponse{
		TransactionId: uuid.NewV4().String(),
	}, nil
}

func (p *payment) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (p *payment) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}
