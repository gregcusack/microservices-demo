package main

import (
	"flag"
	"github.com/triplewy/microservices-demo/src/faultservice"
	"os"
)

var (
	port       string
	queryAddr  string
	kubeconfig string
)

func init() {
	flag.StringVar(&queryAddr, "addr", "cs1380.cs.brown.edu:5000", "addr for jaeger-query")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")

	if value := os.Getenv("PORT"); value != "" {
		port = value
	} else {
		port = "8080"
	}

	if value := os.Getenv("JAEGER_QUERY_ADDR"); value != "" {
		queryAddr = value
		kubeconfig = ""
	}
}

func main() {
	faultservice.Run(port, queryAddr, kubeconfig)
}
