package main

import (
	"context"

	pb "github.com/triplewy/microservices-demo/src/gremlinservice/genproto"
	"google.golang.org/grpc"
)

type faultServiceClient struct {
	cc *grpc.ClientConn
}

// NewFaultServiceClient creates a new fault service client
func NewFaultServiceClient(addr string) *faultServiceClient {
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	return &faultServiceClient{cc: cc}
}

// ApplyFault applies an istio virtual service
func (f *faultServiceClient) ApplyFault(svc string, percent float64) error {
	client := pb.NewFaultServiceClient(f.cc)
	_, err := client.Create(context.Background(), &pb.CreateRequest{
		Svc:     svc,
		Percent: percent,
	})
	return err
}

// DeleteFault deletes an istio virtual service
func (f *faultServiceClient) DeleteFault(svc string) error {
	client := pb.NewFaultServiceClient(f.cc)
	_, err := client.Delete(context.Background(), &pb.DeleteRequest{
		Svc: svc,
	})
	return err
}
