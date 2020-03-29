package main

import (
	cardValidator "github.com/sgumirov/go-cards-validation"
	"gotest.tools/v3/assert"
	"testing"
)

func TestCard(t *testing.T) {
	card := cardValidator.Card{
		Number:  "4432801561520454",
		Cvv:     "672",
		Month:   "1",
		Year:    "2039",
	}
	err := card.Validate(true)
	assert.NilError(t, err)
	err = card.Brand()
	assert.NilError(t, err)
}
