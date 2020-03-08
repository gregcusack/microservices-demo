package main

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jaegertracing/jaeger/model"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"google.golang.org/grpc"
)

func queryServices(cc *grpc.ClientConn) (*api_v2.GetServicesResponse, error) {
	client := api_v2.NewQueryServiceClient(cc)
	res, err := client.GetServices(context.Background(), &api_v2.GetServicesRequest{})
	if err != nil {
		return nil, err
	}

	d, err := res.Marshal()
	if err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(filepath.Join("data", "services"), d, 0644); err != nil {
		return nil, err
	}

	return res, nil
}

func queryChunks(cc *grpc.ClientConn, services []string, since time.Time) (map[string]*api_v2.SpansResponseChunk, error) {
	// Set data folder for saving chunks
	unixNow := strconv.FormatInt(time.Now().Unix(), 10)
	chunksDir := filepath.Join("data", "chunks", unixNow)
	err := os.MkdirAll(chunksDir, 0755)
	if err != nil {
		return nil, err
	}

	client := api_v2.NewQueryServiceClient(cc)
	result := make(map[string]*api_v2.SpansResponseChunk, 0)

	for _, svc := range services {
		// Find all traces for this svc in the past hour with search depth 50
		res, err := client.FindTraces(context.Background(), &api_v2.FindTracesRequest{
			Query: &api_v2.TraceQueryParameters{
				ServiceName:  svc,
				StartTimeMin: since,
				StartTimeMax: time.Now(),
				SearchDepth:  20,
			},
		})
		if err != nil {
			return nil, err
		}

		// Populate spans
		var spans []model.Span

		for {
			c, err := res.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}

			spans = append(spans, c.GetSpans()...)
		}

		chunk := &api_v2.SpansResponseChunk{Spans: spans}

		// Write chunks to file
		if err := func(chunk *api_v2.SpansResponseChunk, svc string) error {
			b, err := chunk.Marshal()
			if err != nil {
				return err
			}
			return ioutil.WriteFile(filepath.Join(chunksDir, svc), b, 0644)
		}(chunk, svc); err != nil {
			return nil, err
		}

		// update map
		result[svc] = chunk
	}

	return result, nil
}
