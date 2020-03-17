package faultservice

import (
	"gotest.tools/v3/assert"
	"testing"
)

func TestReplay(t *testing.T) {
	_, err := replayChunks()
	assert.NilError(t, err)
}
