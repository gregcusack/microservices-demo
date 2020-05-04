package main

import (
	"fmt"
	"github.com/fatih/color"
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
	namespace      string
	downstreamSvcs map[string]DownstreamSvc
}

// GoString Implements GoString interface
func (n Node) GoString() string {
	if len(n.downstreamSvcs) == 0 {
		return ""
	}
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
		if stats.ratio < 0 {
			s.WriteString(fmt.Sprintf("\t\t%v: %v\n", req, color.RedString("%.2f%%", stats.ratio*100)))
			for id := range stats.failedTraces {
				s.WriteString(fmt.Sprintf("\t\t\tFailed traceID: %v\n", color.RedString(id)))
			}
		} else {
			s.WriteString(fmt.Sprintf("\t\t%v: %.2f%%\n", req, stats.ratio*100))
		}
	}
	s.WriteString("\t}")
	return s.String()
}

// RequestStats records the total number of requests and the amount of 200 responses
type RequestStats struct {
	total        int
	success      int
	ratio        float64
	failedTraces map[string]struct{}
}

// MeasureSuccessRate takes in tracing spans and outputs the total amount of requests and
// 200 status ratio for each edge found
func MeasureSuccessRate(chunks map[string]*api_v2.SpansResponseChunk) (g Graph, err error) {
	g = Graph{nodes: make(map[string]Node, 0)}
	result := g.nodes

	for svc, chunk := range chunks {
		svcArr := strings.Split(svc, ".")
		node := Node{
			name:           svcArr[0],
			namespace:      svcArr[1],
			downstreamSvcs: make(map[string]DownstreamSvc, 0),
		}
		result[svcArr[0]] = node

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
				case "grpc.authority":
					v := t.GetVStr()
					if v == "" {
						continue
					}
					arr := strings.Split(v, ":")
					downstreamSvc = arr[0]
				case "upstream_cluster":
					u := t.GetVStr()
					if downstreamSvc != "" || u == "-" {
						continue
					}
					arr := strings.Split(u, "|")
					if arr[0] == "outbound" {
						downstreamSvc = strings.TrimSuffix(arr[len(arr)-1], ".default.svc.cluster.local")
					}
				}
			}

			if downstreamSvc == "" || downstreamSvc == node.name {
				continue
			}

			if _, ok := node.downstreamSvcs[downstreamSvc]; !ok {
				node.downstreamSvcs[downstreamSvc] = DownstreamSvc{
					name:     downstreamSvc,
					requests: make(map[string]RequestStats, 0),
				}
			}

			requests := node.downstreamSvcs[downstreamSvc].requests

			if _, ok := requests[url]; !ok {
				requests[url] = RequestStats{
					total:        0,
					success:      0,
					ratio:        0,
					failedTraces: make(map[string]struct{}, 0),
				}
			}

			stats := requests[url]
			stats.total++

			if is200 && !isError {
				stats.success++
			} else {
				stats.failedTraces[span.TraceID.String()] = struct{}{}
			}

			stats.ratio = float64(stats.success) / float64(stats.total)
			requests[url] = stats
		}
	}

	return
}

func CalculateDeltas(before, after Graph) Graph {
	g := Graph{nodes: make(map[string]Node, 0)}

	for _, node1 := range after.nodes {
		for _, svc1 := range node1.downstreamSvcs {
			for url, req1 := range svc1.requests {
				afterRatio := float64(req1.success) / float64(req1.total)

				if node2, ok := before.nodes[node1.name]; ok {
					if svc2, ok := node2.downstreamSvcs[svc1.name]; ok {
						if req2, ok := svc2.requests[url]; ok {
							beforeRatio := float64(req2.success) / float64(req2.total)

							if _, ok := g.nodes[node1.name]; !ok {
								g.nodes[node1.name] = Node{
									name:           node1.name,
									downstreamSvcs: make(map[string]DownstreamSvc, 0),
								}
							}

							n := g.nodes[node1.name]

							if _, ok := n.downstreamSvcs[svc1.name]; !ok {
								n.downstreamSvcs[svc1.name] = DownstreamSvc{
									name:     svc1.name,
									requests: make(map[string]RequestStats, 0),
								}
							}

							s := n.downstreamSvcs[svc1.name]

							if _, ok := s.requests[url]; !ok {
								s.requests[url] = RequestStats{
									ratio:        afterRatio - beforeRatio,
									failedTraces: req1.failedTraces,
								}
							}
						}
					}
				}
			}
		}
	}

	return g
}
