package main

import (
	"testing"

	"gotest.tools/assert"
)

func TestYaml(t *testing.T) {
	_, err := createFaultInjection("testservice")
	assert.NilError(t, err)
}
