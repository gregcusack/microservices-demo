package main

import (
	"fmt"
	"gotest.tools/v3/assert"
	"testing"
)

func TestKiali(t *testing.T) {
	k := NewKialiClient()
	rates, err := k.GetAllTrafficRates()
	assert.NilError(t, err)
	fmt.Println(rates)
}
