package main

import (
	"fmt"
	"gotest.tools/v3/assert"
	"testing"
)

func TestParseDag(t *testing.T) {
	dags, err := parseDags("1585705628")
	assert.NilError(t, err)
	fmt.Println(dags)
}
