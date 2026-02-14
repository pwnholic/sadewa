package core

import (
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
)

// RateLimitConfig holds rate limiting configuration for an exchange.
type RateLimitConfig struct {
	RequestsPerSecond int
	OrdersPerSecond   int
	Burst             int
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
	Exchange    string       `json:"exchange" validate:"required"`
	MarketType  MarketType   `json:"market_type"`
	Sandbox     bool         `json:"sandbox"`
	Credentials *Credentials `json:"credentials,omitempty"`

	// Timeout is the maximum duration for HTTP requests.
	Timeout      time.Duration `json:"timeout" validate:"min=1ms"`
	MaxRetries   int           `json:"max_retries" validate:"min=0"`
	RetryWaitMin time.Duration `json:"retry_wait_min" validate:"min=0"`
	RetryWaitMax time.Duration `json:"retry_wait_max" validate:"min=0"`

	RateLimitRequests int           `json:"rate_limit_requests" validate:"min=1"`
	RateLimitPeriod   time.Duration `json:"rate_limit_period" validate:"min=1ms"`

	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl" validate:"min=0"`

	CircuitBreakerEnabled          bool          `json:"circuit_breaker_enabled"`
	CircuitBreakerFailThreshold    int           `json:"circuit_breaker_fail_threshold"`
	CircuitBreakerSuccessThreshold int           `json:"circuit_breaker_success_threshold"`
	CircuitBreakerTimeout          time.Duration `json:"circuit_breaker_timeout"`

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

var validate = validator.New()

func (c *Config) Validate() error {
	if err := validate.Struct(c); err != nil {
		return err
	}
	if c.CircuitBreakerEnabled {
		if c.CircuitBreakerFailThreshold <= 0 {
			return errors.New("CircuitBreakerFailThreshold must be positive when enabled")
		}
		if c.CircuitBreakerSuccessThreshold <= 0 {
			return errors.New("CircuitBreakerSuccessThreshold must be positive when enabled")
		}
		if c.CircuitBreakerTimeout <= 0 {
			return errors.New("CircuitBreakerTimeout must be positive when enabled")
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
