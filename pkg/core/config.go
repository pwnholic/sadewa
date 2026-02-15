package core

import (
	"time"

	"github.com/go-playground/validator/v10"
)

// RateLimitConfig holds rate limiting configuration for an exchange.
type RateLimitConfig struct {
	// RequestsPerSecond is the maximum number of general requests per second.
	RequestsPerSecond int
	// OrdersPerSecond is the maximum number of order requests per second.
	OrdersPerSecond int
	// Burst allows temporary exceeding of the rate limit.
	Burst int
}

// Credentials holds API authentication credentials for an exchange.
type Credentials struct {
	// APIKey is the public API key identifier.
	APIKey string `json:"api_key"`
	// SecretKey is the private API key used for signing requests.
	SecretKey string `json:"secret_key"`
	// Passphrase is an optional additional credential required by some exchanges.
	Passphrase string `json:"passphrase,omitempty"`
}

// Config contains all configuration options for an exchange session.
// It includes authentication, networking, rate limiting, caching, and circuit breaker settings.
type Config struct {
	// Exchange is the name of the exchange (e.g., "binance", "okx").
	Exchange string `json:"exchange" validate:"required"`
	// MarketType specifies the market type (spot, futures, options).
	MarketType MarketType `json:"market_type"`
	// Sandbox enables test/sandbox mode for paper trading.
	Sandbox bool `json:"sandbox"`
	// Credentials holds the API authentication credentials.
	Credentials *Credentials `json:"credentials,omitempty"`

	// Timeout is the maximum duration for HTTP requests.
	Timeout time.Duration `json:"timeout" validate:"min=1ms"`
	// MaxRetries is the maximum number of retry attempts for failed requests.
	MaxRetries int `json:"max_retries" validate:"min=0"`
	// RetryWaitMin is the minimum wait time between retries.
	RetryWaitMin time.Duration `json:"retry_wait_min" validate:"min=0"`
	// RetryWaitMax is the maximum wait time between retries.
	RetryWaitMax time.Duration `json:"retry_wait_max" validate:"min=0"`

	// RateLimitRequests is the number of requests allowed per rate limit period.
	RateLimitRequests int `json:"rate_limit_requests" validate:"min=1"`
	// RateLimitPeriod is the time window for rate limiting.
	RateLimitPeriod time.Duration `json:"rate_limit_period" validate:"min=1ms"`

	// CacheEnabled enables response caching for idempotent operations.
	CacheEnabled bool `json:"cache_enabled"`
	// CacheTTL is the time-to-live for cached responses.
	CacheTTL time.Duration `json:"cache_ttl" validate:"min=0"`

	// CircuitBreakerEnabled enables the circuit breaker pattern for fault tolerance.
	CircuitBreakerEnabled bool `json:"circuit_breaker_enabled"`
	// CircuitBreakerFailThreshold is the number of failures before opening the circuit.
	CircuitBreakerFailThreshold int `json:"circuit_breaker_fail_threshold"`
	// CircuitBreakerSuccessThreshold is the number of successes before closing the circuit.
	CircuitBreakerSuccessThreshold int `json:"circuit_breaker_success_threshold"`
	// CircuitBreakerTimeout is the time to wait before attempting to close the circuit.
	CircuitBreakerTimeout time.Duration `json:"circuit_breaker_timeout"`

	// LogLevel sets the logging verbosity (debug, info, warn, error).
	LogLevel string `json:"log_level" validate:"omitempty,oneof=debug info warn error"`
}

// DefaultConfig returns a Config initialized with sensible defaults for the specified exchange.
// Default values: 10s timeout, 3 retries, 100ms-1s retry wait, 1200 req/min rate limit,
// 1s cache TTL, circuit breaker with 5 failures/2 successes/30s timeout.
func DefaultConfig(exchange string) *Config {
	return &Config{
		Exchange:     exchange,
		Sandbox:      false,
		Timeout:      10 * time.Second,
		MaxRetries:   3,
		RetryWaitMin: 100 * time.Millisecond,
		RetryWaitMax: 1 * time.Second,

		RateLimitRequests: 1200,
		RateLimitPeriod:   time.Minute,

		CacheEnabled: true,
		CacheTTL:     1 * time.Second,

		CircuitBreakerEnabled:          true,
		CircuitBreakerFailThreshold:    5,
		CircuitBreakerSuccessThreshold: 2,
		CircuitBreakerTimeout:          30 * time.Second,

		LogLevel: "info",
	}
}

// validate is the shared validator instance for config validation.
var validate = validator.New()

// Validate validates the configuration fields using struct tags and custom rules.
// Returns an error if any field fails validation, particularly for circuit breaker settings.
func (c *Config) Validate() error {
	if err := validate.Struct(c); err != nil {
		return NewExchangeError("", ErrorTypeBadRequest, 400, err.Error()).
			WithCode(ErrCodeInvalidConfig)
	}
	if c.CircuitBreakerEnabled {
		if c.CircuitBreakerFailThreshold <= 0 {
			return NewExchangeError("", ErrorTypeBadRequest, 400,
				"CircuitBreakerFailThreshold must be positive when enabled").
				WithCode(ErrCodeInvalidConfig)
		}
		if c.CircuitBreakerSuccessThreshold <= 0 {
			return NewExchangeError("", ErrorTypeBadRequest, 400,
				"CircuitBreakerSuccessThreshold must be positive when enabled").
				WithCode(ErrCodeInvalidConfig)
		}
		if c.CircuitBreakerTimeout <= 0 {
			return NewExchangeError("", ErrorTypeBadRequest, 400,
				"CircuitBreakerTimeout must be positive when enabled").
				WithCode(ErrCodeInvalidConfig)
		}
	}
	return nil
}

// WithCredentials sets the API credentials and returns the config for chaining.
func (c *Config) WithCredentials(creds *Credentials) *Config {
	c.Credentials = creds
	return c
}

// WithSandbox enables or disables sandbox mode and returns the config for chaining.
func (c *Config) WithSandbox(sandbox bool) *Config {
	c.Sandbox = sandbox
	return c
}

// WithTimeout sets the request timeout and returns the config for chaining.
func (c *Config) WithTimeout(timeout time.Duration) *Config {
	c.Timeout = timeout
	return c
}

// WithRateLimit sets the rate limiting parameters and returns the config for chaining.
func (c *Config) WithRateLimit(requests int, period time.Duration) *Config {
	c.RateLimitRequests = requests
	c.RateLimitPeriod = period
	return c
}

// WithCache enables or disables caching with the specified TTL and returns the config for chaining.
func (c *Config) WithCache(enabled bool, ttl time.Duration) *Config {
	c.CacheEnabled = enabled
	c.CacheTTL = ttl
	return c
}
