package main

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"google.golang.org/grpc"
)

func query(addr string, disk bool) (chunks []*api_v2.SpansResponseChunk, services *api_v2.GetServicesResponse, err error) {
	if disk {
		services, err = replayServices()
		if err != nil {
			log.Fatalf("Replay services: %v\n", err)
		}
		chunks, err = replayChunks(services.GetServices())
		if err != nil {
			log.Fatalf("Replay chunks: %v\n", err)
		}
	} else {
		// Dial jaeger query
		cc, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			log.Fatalf("Dial: %v\n", err)
		}
		client := api_v2.NewQueryServiceClient(cc)

		services, err = queryServices(client)
		if err != nil {
			log.Fatalf("Query services: %v\n", err)
		}

		chunks, err = queryChunks(client, services.GetServices())
		if err != nil {
			log.Fatalf("Query chunks: %v\n", err)
		}
	}
	return
}

func queryServices(client api_v2.QueryServiceClient) (*api_v2.GetServicesResponse, error) {
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

	for _, svc := range res.GetServices() {
		err := os.MkdirAll(filepath.Join("data", "chunks", svc), 0755)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func replayServices() (*api_v2.GetServicesResponse, error) {
	b, err := ioutil.ReadFile(filepath.Join("data", "services"))
	if err != nil {
		return nil, err
	}
	services := &api_v2.GetServicesResponse{}
	err = services.Unmarshal(b)
	if err != nil {
		return nil, err
	}
	return services, nil
}

func queryChunks(client api_v2.QueryServiceClient, services []string) ([]*api_v2.SpansResponseChunk, error) {
	var chunks []*api_v2.SpansResponseChunk

	for _, svc := range services {
		if strings.Contains(svc, "jaeger") {
			continue
		}
		// Find all traces for this svc in the past hour with search depth 100
		res, err := client.FindTraces(context.Background(), &api_v2.FindTracesRequest{
			Query: &api_v2.TraceQueryParameters{
				ServiceName:  svc,
				StartTimeMin: time.Now().Add(time.Duration(-1) * time.Hour),
				StartTimeMax: time.Now(),
				SearchDepth:  100,
			},
		})
		if err != nil {
			return nil, err
		}

		// Populate graph
		i := 0
		for {
			c, err := res.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}

			d, err := c.Marshal()
			if err != nil {
				return nil, err
			}
			err = ioutil.WriteFile(filepath.Join("data", "chunks", svc, strconv.Itoa(i)), d, 0644)
			if err != nil {
				return nil, err
			}

			chunks = append(chunks, c)

			i++
		}
	}

	return chunks, nil
}

func replayChunks(services []string) ([]*api_v2.SpansResponseChunk, error) {
	var chunks []*api_v2.SpansResponseChunk

	for _, svc := range services {
		files, err := ioutil.ReadDir(filepath.Join("data", "chunks", svc))
		if err != nil {
			return nil, err
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}

			b, err := ioutil.ReadFile(filepath.Join("data", "chunks", svc, f.Name()))
			if err != nil {
				return nil, err
			}

			chunk := &api_v2.SpansResponseChunk{}

			err = chunk.Unmarshal(b)
			if err != nil {
				return nil, err
			}

			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}
