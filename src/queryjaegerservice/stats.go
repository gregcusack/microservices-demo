package main

import (
	"strings"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
)

// Node represents a node in the service mesh
type Node struct {
	name           string
	downstreamSvcs map[string]DownstreamSvc
}

// DownstreamSvc represents a downstream service in the microservice call graph
type DownstreamSvc struct {
	name     string
	requests map[string]RequestStats
}

// RequestStats records the total number of requests and the amount of 200 responses
type RequestStats struct {
	total   int
	success int
}

// measureSuccessRate takes in tracing spans and outputs the total amount of requests and
// 200 status ratio for each edge found
func measureSuccessRate(chunks map[string]*api_v2.SpansResponseChunk) (result map[string]Node, err error) {
	result = make(map[string]Node, 0)

	for svc, chunk := range chunks {

		node := Node{
			name:           svc,
			downstreamSvcs: make(map[string]DownstreamSvc, 0),
		}
		result[svc] = node

		for _, span := range chunk.GetSpans() {
			// http url
			var url string
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
			if is200 {
				stats.success++
			}

			requests[url] = stats
		}
	}

	return result, nil
}
