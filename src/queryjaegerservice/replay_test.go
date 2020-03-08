package main

import (
	"testing"

	"gotest.tools/assert"
)

func TestReplay(t *testing.T) {
	_, err := replayChunks()
	assert.NilError(t, err)
}
