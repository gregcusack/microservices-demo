package main

import (
	"fmt"
	"github.com/jaegertracing/jaeger/model"
	"strings"
)

type dag struct {
	vertices map[string]vertex // map[spanID]Vertex
	edges    []edge
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

func (d dag) exportDag(index int, vLabels, eLabels map[string]int) string {
	s := strings.Builder{}
	s.WriteString(fmt.Sprintf("t # %d\n", index))

	for _, v := range d.vertices {
		if _, ok := vLabels[v.label]; !ok {
			vLabels[v.label] = len(vLabels)
		}
	}

	vertexIndexes := make(map[string]int, 0)
	i := 0
	for spanID, v := range d.vertices {
		labelIndex := vLabels[v.label]
		s.WriteString(fmt.Sprintf("v %d %d\n", i, labelIndex))
		vertexIndexes[spanID] = i
		i++
	}

	for _, e := range d.edges {
		if _, ok := eLabels[e.label]; !ok {
			eLabels[e.label] = len(eLabels)
		}
	}

	for _, e := range d.edges {
		edgeIndex := eLabels[e.label]
		s.WriteString(fmt.Sprintf("e %d %d %d\n", vertexIndexes[e.source], vertexIndexes[e.dest], edgeIndex))
	}

	return s.String()
}