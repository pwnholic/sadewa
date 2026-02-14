package core

import (
	"context"

	"resty.dev/v3"
)

// RateLimitConfig defines rate limiting parameters for an exchange protocol.
type RateLimitConfig struct {
	// RequestsPerSecond is the maximum general requests per second.
	RequestsPerSecond int `json:"requests_per_second"`
	// OrdersPerSecond is the maximum order placement requests per second.
	OrdersPerSecond int `json:"orders_per_second"`
	// Burst allows temporary exceeding of rate limits.
	Burst int `json:"burst"`
}

// Protocol defines the interface for exchange-specific protocol implementations.
// Each exchange must implement this interface to handle request building,
// response parsing, authentication, and rate limiting.
type Protocol interface {
	// Name returns the exchange identifier (e.g., "binance", "coinbase").
	Name() string

	// Version returns the API version being used.
	Version() string

	// BaseURL returns the API base URL for the given environment.
	// Sandbox mode returns the test environment URL when available.
	BaseURL(sandbox bool) string

	// BuildRequest constructs an HTTP request for the specified operation.
	// The params map contains operation-specific parameters.
	// Returns a Request object ready for execution or an error.
	BuildRequest(ctx context.Context, op Operation, params Params) (*Request, error)

	// ParseResponse deserializes the HTTP response and normalizes it to canonical types.
	// The op parameter specifies which operation was performed.
	// Returns the normalized canonical type for the operation or an error.
	ParseResponse(op Operation, resp *resty.Response) (any, error)

	// SignRequest adds authentication headers and signature to the request.
	// The credentials provide the keys needed for signing.
	SignRequest(req *resty.Request, creds Credentials) error

	// SupportedOperations returns the list of operations this protocol supports.
	SupportedOperations() []Operation

	// RateLimits returns the rate limiting configuration for this exchange.
	RateLimits() RateLimitConfig
}
