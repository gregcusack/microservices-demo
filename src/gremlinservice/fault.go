package main

import (
	"context"

	pb "github.com/triplewy/microservices-demo/src/gremlinservice/genproto"
	"google.golang.org/grpc"
)

type FaultServiceClient struct {
	cc *grpc.ClientConn
}

func NewFaultServiceClient(addr string) *FaultServiceClient {
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	return &FaultServiceClient{cc: cc}
}

func (f *FaultServiceClient) ApplyFault(svc string) error {
	client := pb.NewFaultServiceClient(f.cc)
	_, err := client.Create(context.Background(), &pb.CreateRequest{
		Svc: svc,
	})
	return err
}

func (f *FaultServiceClient) DeleteFault(svc string) error {
	client := pb.NewFaultServiceClient(f.cc)
	_, err := client.Delete(context.Background(), &pb.DeleteRequest{
		Svc: svc,
	})
	return err
}
