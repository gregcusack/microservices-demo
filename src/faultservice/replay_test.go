package faultservice

import (
	"fmt"
	"gotest.tools/v3/assert"
	"testing"
)

func TestReplay(t *testing.T) {
	chunks, err := replayChunks()
	assert.NilError(t, err)

	graph, err := MeasureSuccessRate(chunks)
	assert.NilError(t, err)

	fmt.Printf("%#v\n", graph)
}
