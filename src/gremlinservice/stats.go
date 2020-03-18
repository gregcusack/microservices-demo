package main

import (
	"fmt"
	"strings"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
)

// Graph is a map of Nodes
type Graph struct {
	nodes map[string]Node
}

// GoString Implements GoString interface
func (g Graph) GoString() string {
	s := strings.Builder{}
	for _, n := range g.nodes {
		s.WriteString(fmt.Sprintf("%#v", n))
	}
	return s.String()
}

// Node represents a node in the service mesh
type Node struct {
	name           string
	downstreamSvcs map[string]DownstreamSvc
}

// GoString Implements GoString interface
func (n Node) GoString() string {
	s := strings.Builder{}
	s.WriteString(fmt.Sprintf("%v: {\n", n.name))
	for _, svc := range n.downstreamSvcs {
		s.WriteString(fmt.Sprintf("\t%#v\n", svc))
	}
	s.WriteString("}\n")
	return s.String()
}

// DownstreamSvc represents a downstream service in the microservice call graph
type DownstreamSvc struct {
	name     string
	requests map[string]RequestStats
}

// GoString Implements GoString interface
func (d DownstreamSvc) GoString() string {
	s := strings.Builder{}
	s.WriteString(fmt.Sprintf("%v: {\n", d.name))
	for req, stats := range d.requests {
		s.WriteString(fmt.Sprintf("\t\t%v: %.2f%%\n", req, float64(stats.success)/float64(stats.total)*100))
	}
	s.WriteString("\t}")
	return s.String()
}

// RequestStats records the total number of requests and the amount of 200 responses
type RequestStats struct {
	total   int
	success int
}

// MeasureSuccessRate takes in tracing spans and outputs the total amount of requests and
// 200 status ratio for each edge found
func MeasureSuccessRate(chunks map[string]*api_v2.SpansResponseChunk) (g Graph, err error) {
	g = Graph{nodes: make(map[string]Node, 0)}
	result := g.nodes

	for svc, chunk := range chunks {

		node := Node{
			name:           svc,
			downstreamSvcs: make(map[string]DownstreamSvc, 0),
		}
		result[svc] = node

		for _, span := range chunk.GetSpans() {
			// http url
			var url string
			// isError used for fault filter aborts
			var isError bool
			// status code == 200
			var is200 bool
			// downstream service
			var downstreamSvc string

			for _, t := range span.GetTags() {
				switch t.GetKey() {
				case "http.url":
					url = t.GetVStr()
				case "http.status_code":
					if t.GetVStr() == "200" {
						is200 = true
					} else {
						is200 = false
					}
				case "error":
					isError = t.GetVBool()
				case "upstream_cluster":
					u := t.GetVStr()
					if u == "-" {
						break
					}
					arr := strings.Split(u, "|")
					if arr[0] == "outbound" {
						downstreamSvc = strings.TrimSuffix(arr[len(arr)-1], ".svc.cluster.local")
					}
				}
			}

			if downstreamSvc == "" {
				continue
			}

			if _, ok := node.downstreamSvcs[downstreamSvc]; !ok {
				node.downstreamSvcs[downstreamSvc] = DownstreamSvc{
					name:     downstreamSvc,
					requests: make(map[string]RequestStats, 0),
				}
			}

			requests := node.downstreamSvcs[downstreamSvc].requests

			stats, ok := requests[url]

			if !ok {
				requests[url] = RequestStats{
					total:   0,
					success: 0,
				}
			}

			stats.total++
			if is200 && !isError {
				stats.success++
			}

			requests[url] = stats
		}
	}

	return
}
