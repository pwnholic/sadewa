package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRequest(t *testing.T) {
	req := NewRequest("GET", "/api/v3/ticker/price")

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/api/v3/ticker/price", req.Path)
	assert.NotNil(t, req.Query)
	assert.NotNil(t, req.Headers)
	assert.Equal(t, 1, req.Weight)
}

func TestRequest_SetQuery(t *testing.T) {
	req := NewRequest("GET", "/api/v3/ticker/price")
	result := req.SetQuery("symbol", "BTCUSDT")

	assert.Equal(t, req, result)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
}

func TestRequest_SetBody(t *testing.T) {
	req := NewRequest("POST", "/api/v3/order")
	body := map[string]string{"symbol": "BTCUSDT"}
	result := req.SetBody(body)

	assert.Equal(t, req, result)
	assert.Equal(t, body, req.Body)
}

func TestRequest_SetHeader(t *testing.T) {
	req := NewRequest("GET", "/api/v3/ticker/price")
	result := req.SetHeader("X-Custom", "value")

	assert.Equal(t, req, result)
	assert.Equal(t, "value", req.Headers["X-Custom"])
}

func TestRequest_SetWeight(t *testing.T) {
	req := NewRequest("GET", "/api/v3/ticker/price")
	result := req.SetWeight(5)

	assert.Equal(t, req, result)
	assert.Equal(t, 5, req.Weight)
}

func TestRequest_SetCache(t *testing.T) {
	req := NewRequest("GET", "/api/v3/ticker/price")
	result := req.SetCache("ticker:BTCUSDT", 5*time.Second)

	assert.Equal(t, req, result)
	assert.Equal(t, "ticker:BTCUSDT", req.CacheKey)
	assert.Equal(t, 5*time.Second, req.CacheTTL)
}

func TestRequest_SetRequireAuth(t *testing.T) {
	req := NewRequest("POST", "/api/v3/order")
	result := req.SetRequireAuth(true)

	assert.Equal(t, req, result)
	assert.True(t, req.RequireAuth)
}

func TestRequest_SetQueryParams(t *testing.T) {
	req := NewRequest("GET", "/api/v3/ticker/price")
	params := Params{
		"symbol":  "BTCUSDT",
		"limit":   100,
		"enabled": true,
	}
	result := req.SetQueryParams(params)

	assert.Equal(t, req, result)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
	assert.Equal(t, 100, req.Query["limit"])
	assert.Equal(t, true, req.Query["enabled"])
}

func TestRequest_Chained(t *testing.T) {
	req := NewRequest("POST", "/api/v3/order").
		SetQuery("symbol", "BTCUSDT").
		SetHeader("X-MBX-APIKEY", "test-key").
		SetWeight(2).
		SetRequireAuth(true).
		SetCache("order:123", 0)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/api/v3/order", req.Path)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
	assert.Equal(t, "test-key", req.Headers["X-MBX-APIKEY"])
	assert.Equal(t, 2, req.Weight)
	assert.True(t, req.RequireAuth)
}

func TestParams(t *testing.T) {
	params := Params{
		"key1": "value1",
		"key2": 123,
	}

	assert.Equal(t, "value1", params["key1"])
	assert.Equal(t, 123, params["key2"])
}
