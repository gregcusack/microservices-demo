package main

import (
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
)

func replayServices() (*api_v2.GetServicesResponse, error) {
	b, err := ioutil.ReadFile(filepath.Join("data", "services"))
	if err != nil {
		return nil, err
	}
	services := &api_v2.GetServicesResponse{}
	if err := services.Unmarshal(b); err != nil {
		return nil, err
	}
	return services, nil
}

func replayChunks() (map[string]*api_v2.SpansResponseChunk, error) {
	result := make(map[string]*api_v2.SpansResponseChunk, 0)

	chunksDir := filepath.Join("data", "chunks")
	dirs, err := ioutil.ReadDir(filepath.Join("data", "chunks"))
	if err != nil {
		return nil, err
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].IsDir() != dirs[j].IsDir() {
			return dirs[i].IsDir()
		}
		return dirs[i].Name() > dirs[j].Name()
	})

	dir := filepath.Join(chunksDir, dirs[0].Name())
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		chunk, err := func(filename string) (*api_v2.SpansResponseChunk, error) {
			b, err := ioutil.ReadFile(filepath.Join(dir, filename))
			if err != nil {
				return nil, err
			}

			chunk := &api_v2.SpansResponseChunk{}

			if err := chunk.Unmarshal(b); err != nil {
				return nil, err
			}

			return chunk, nil
		}(f.Name())

		if err != nil {
			return nil, err
		}

		result[f.Name()] = chunk
	}

	return result, nil
}
