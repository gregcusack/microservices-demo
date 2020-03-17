package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/triplewy/microservices-demo/src/faultservice"
	"google.golang.org/grpc"
	"io"
	"log"
)

var (
	addr string
)

func init() {
	flag.StringVar(&addr, "addr", "", "address of fault service")
}

func main() {
	flag.Parse()

	fmt.Println(addr)

	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}

	client := faultservice.NewFaultServiceClient(cc)

	server, err := client.Experiment(context.Background(), &faultservice.EmptyMsg{})
	if err != nil {
		log.Fatal(err)
	}

	for {
		msg, err := server.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		fmt.Println(msg.GetInfo())
	}
}
