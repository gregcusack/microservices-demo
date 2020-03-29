package main

import (
	pb "github.com/triplewy/microservices-demo/src/currencyservice/genproto"
	"gotest.tools/v3/assert"
	"testing"
)

func TestCovert(t *testing.T) {
	// $2245
	from := &pb.Money{
		CurrencyCode: "USD",
		Units:        2245,
		Nanos:        0,
	}

	// Convert $2245 to euros: 1985.84697037
	euros := convertToEuros(from)
	assert.Equal(t, "EUR", euros.GetCurrencyCode())
	assert.Equal(t, int64(1985), euros.GetUnits())
	assert.Equal(t, int32(846970367), euros.GetNanos())

	// Convert euros back to USD
	usd := convertFromEuros(euros, "USD")
	assert.Equal(t, "USD", usd.GetCurrencyCode())
	assert.Equal(t, int64(2244), usd.GetUnits())
	assert.Equal(t, int32(999999999), usd.GetNanos())
}
