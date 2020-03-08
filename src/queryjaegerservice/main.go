package main

import (
	"flag"
	"fmt"
	"log"
)

var (
	disk bool
	addr string
)

func init() {
	flag.BoolVar(&disk, "d", true, "read from disk")
	flag.StringVar(&addr, "addr", "192.168.64.7:30580", "addr for jaeger-query")
}

func main() {
	flag.Parse()

	chunks, services, err := query(addr, disk)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("num services: %v\n", len(chunks))
	fmt.Printf("num chunks: %v\n", len(services.GetServices()))
}
