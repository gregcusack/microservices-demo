package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jaegertracing/jaeger/model"
)

type dag struct {
	vertices map[string]vertex // map[spanID]Vertex
	edges    []edge
	support  int
}

func (d dag) GoString() string {
	s := strings.Builder{}
	s.WriteString("Vertices:\n")
	for spanID, v := range d.vertices {
		s.WriteString(fmt.Sprintf("\t%v: %#v\n", spanID, v))
	}
	s.WriteString("Edges:\n")
	for _, e := range d.edges {
		s.WriteString(fmt.Sprintf("\t%#v\n", e))
	}
	s.WriteString(fmt.Sprintf("Support: %v\n", d.support))
	return s.String()
}

type vertex struct {
	label string
	value interface{}
}

func (v vertex) GoString() string {
	return v.label
}

type edge struct {
	label  string
	source string
	dest   string
}

func (e edge) GoString() string {
	return fmt.Sprintf("label: %v, %v --> %v", e.label, e.source, e.dest)
}

func traceToDag(trace []model.Span) dag {
	d := dag{
		vertices: make(map[string]vertex, 0),
		edges:    nil,
	}

	for _, span := range trace {
		spanID := span.SpanID.String()

		d.vertices[spanID] = vertex{
			label: span.GetProcess().GetServiceName(),
			value: span,
		}

		for _, ref := range span.GetReferences() {
			if _, ok := d.vertices[ref.SpanID.String()]; !ok {
				// Fill in with empty vertex for now
				d.vertices[ref.SpanID.String()] = vertex{}
			}
			switch ref.GetRefType().String() {
			case "CHILD_OF":
				d.edges = append(d.edges, edge{
					label:  span.GetOperationName(),
					source: ref.SpanID.String(),
					dest:   spanID,
				})
			default:
				sugar.Fatal("Have no idea what to do for FOLLOWS_FOR")
			}
		}
	}

	return d
}

func exportDag(d dag, index int, vLabels, eLabels map[string]int) (string, map[string]int) {
	for _, v := range d.vertices {
		if _, ok := vLabels[v.label]; !ok {
			vLabels[v.label] = len(vLabels)
		}
	}

	for _, e := range d.edges {
		if _, ok := eLabels[e.label]; !ok {
			eLabels[e.label] = len(eLabels)
		}
	}

	s := strings.Builder{}

	s.WriteString(fmt.Sprintf("t # %d\n", index))

	vertexIndexes := make(map[string]int, 0)

	for spanID, v := range d.vertices {
		labelIndex := vLabels[v.label]
		vertexIndexes[spanID] = len(vertexIndexes)
		s.WriteString(fmt.Sprintf("v %d %d\n", vertexIndexes[spanID], labelIndex))
	}

	for _, e := range d.edges {
		edgeIndex := eLabels[e.label]
		s.WriteString(fmt.Sprintf("e %d %d %d\n", vertexIndexes[e.source], vertexIndexes[e.dest], edgeIndex))
	}

	return s.String(), vertexIndexes
}

func tracesToDags(path string) error {
	// Map traceID to dag
	dags := make(map[string]dag, 0)

	// Populate dags
	if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		traceID := info.Name()
		trace, err := readChunk(path)
		if err != nil {
			return err
		}
		dags[traceID] = traceToDag(trace.GetSpans())
		return nil
	}); err != nil {
		return err
	}

	// Map traceID to graph index
	gLabels := make(map[string]int, 0)

	// Map traceID to vertices
	gVertices := make(map[string]map[string]int, 0)

	// Map service name to vertex index
	vLabels := make(map[string]int, 0)

	// Map request name to edge index
	eLabels := make(map[string]int, 0)

	s := strings.Builder{}

	for traceID, dag := range dags {
		gLabels[traceID] = len(gLabels)

		// vertices maps spanID to vertex index
		result, vertices := exportDag(dag, gLabels[traceID], vLabels, eLabels)
		s.WriteString(result)

		gVertices[traceID] = vertices
	}

	s.WriteString("t # -1\n")

	// Write all vertices and edges to a file
	if err := ioutil.WriteFile(filepath.Join(path, "traces.data"), []byte(s.String()), 0644); err != nil {
		return err
	}

	var data []byte
	var err error

	// Write vLabels to disk
	if data, err = json.Marshal(vLabels); err != nil {
		return err
	}
	if err = ioutil.WriteFile(filepath.Join(path, "vLabels"), data, 0644); err != nil {
		return err
	}

	// Write eLabels to disk
	if data, err = json.Marshal(eLabels); err != nil {
		return err
	}
	if err = ioutil.WriteFile(filepath.Join(path, "eLabels"), data, 0644); err != nil {
		return err
	}

	// Write gLables to disk
	if data, err = json.Marshal(gLabels); err != nil {
		return err
	}
	if err = ioutil.WriteFile(filepath.Join(path, "gLabels"), data, 0644); err != nil {
		return err
	}

	// Write traceID to spanID to index to disk
	if data, err = json.Marshal(gVertices); err != nil {
		return err
	}
	if err = ioutil.WriteFile(filepath.Join(path, "gVertices"), data, 0644); err != nil {
		return err
	}

	return nil
}

func parseDags(path string) (dags []dag, err error) {
	gData, err := ioutil.ReadFile(filepath.Join(path, "traces.result"))
	if err != nil {
		return nil, err
	}
	eData, err := ioutil.ReadFile(filepath.Join(path, "eLabels"))
	if err != nil {
		return nil, err
	}
	vData, err := ioutil.ReadFile(filepath.Join(path, "vLabels"))
	if err != nil {
		return nil, err
	}
	eJSON := make(map[string]int, 0)
	if err = json.Unmarshal(eData, &eJSON); err != nil {
		return
	}
	eLabels := make(map[string]string, 0)
	for name, index := range eJSON {
		eLabels[strconv.Itoa(index)] = name
	}
	vJSON := make(map[string]int, 0)
	if err = json.Unmarshal(vData, &vJSON); err != nil {
		return
	}
	vLabels := make(map[string]string, 0)
	for name, index := range vJSON {
		vLabels[strconv.Itoa(index)] = name
	}

	graphs := strings.Split(string(gData), "-----------------")

	for _, g := range graphs {
		d := dag{
			vertices: make(map[string]vertex),
			edges:    nil,
			support:  0,
		}
		arr := strings.Split(g, "\n")
		hasNotFrontendSvc := false
		for _, line := range arr {
			if strings.HasPrefix(line, "v") {
				elements := strings.Split(line, " ")
				index := elements[1]
				label := elements[2]
				d.vertices[index] = vertex{
					label: vLabels[label],
					value: nil,
				}
				if !strings.Contains(strings.ToLower(vLabels[label]), "frontend") {
					hasNotFrontendSvc = true
				}
			} else if strings.HasPrefix(line, "e") {
				elements := strings.Split(line, " ")
				src := elements[1]
				dst := elements[2]
				label := elements[3]
				d.edges = append(d.edges, edge{
					label:  eLabels[label],
					source: src,
					dest:   dst,
				})
			} else if strings.HasPrefix(line, "Support:") {
				var support int
				elements := strings.Split(line, " ")
				if support, err = strconv.Atoi(elements[1]); err != nil {
					return
				}
				d.support = support
			}
		}

		if hasNotFrontendSvc {
			dags = append(dags, d)
		}
	}

	sort.Slice(dags, func(i, j int) bool {
		return dags[i].support > dags[j].support
	})

	// Just get top 10 dags
	if len(dags) > 10 {
		dags = dags[:10]
	}
	return
}
