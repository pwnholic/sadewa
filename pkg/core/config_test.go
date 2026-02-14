package core

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("binance")

	assert.Equal(t, "binance", config.Exchange)
	assert.False(t, config.Sandbox)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.RetryWaitMin)
	assert.Equal(t, 1*time.Second, config.RetryWaitMax)
	assert.Equal(t, 1200, config.RateLimitRequests)
	assert.Equal(t, time.Minute, config.RateLimitPeriod)
	assert.True(t, config.CacheEnabled)
	assert.Equal(t, 1*time.Second, config.CacheTTL)
	assert.True(t, config.CircuitBreakerEnabled)
	assert.Equal(t, 5, config.CircuitBreakerFailThreshold)
	assert.Equal(t, 2, config.CircuitBreakerSuccessThreshold)
	assert.Equal(t, 30*time.Second, config.CircuitBreakerTimeout)
	assert.Equal(t, "info", config.LogLevel)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid_config",
			config:  DefaultConfig("binance"),
			wantErr: false,
		},
		{
			name: "missing_exchange",
			config: &Config{
				Timeout: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "Exchange",
		},
		{
			name: "invalid_timeout",
			config: &Config{
				Exchange: "binance",
				Timeout:  -1 * time.Second,
			},
			wantErr: true,
			errMsg:  "Timeout",
		},
		{
			name: "negative_max_retries",
			config: &Config{
				Exchange:   "binance",
				Timeout:    10 * time.Second,
				MaxRetries: -1,
			},
			wantErr: true,
			errMsg:  "MaxRetries",
		},
		{
			name: "invalid_rate_limit_requests",
			config: &Config{
				Exchange:          "binance",
				Timeout:           10 * time.Second,
				RateLimitRequests: 0,
			},
			wantErr: true,
			errMsg:  "RateLimitRequests",
		},
		{
			name: "invalid_rate_limit_period",
			config: &Config{
				Exchange:          "binance",
				Timeout:           10 * time.Second,
				RateLimitRequests: 100,
				RateLimitPeriod:   0,
			},
			wantErr: true,
			errMsg:  "RateLimitPeriod",
		},
		{
			name: "invalid_circuit_breaker_fail_threshold",
			config: &Config{
				Exchange:                    "binance",
				Timeout:                     10 * time.Second,
				RateLimitRequests:           100,
				RateLimitPeriod:             time.Second,
				CircuitBreakerEnabled:       true,
				CircuitBreakerFailThreshold: 0,
			},
			wantErr: true,
			errMsg:  "CircuitBreakerFailThreshold",
		},
		{
			name: "invalid_circuit_breaker_success_threshold",
			config: &Config{
				Exchange:                       "binance",
				Timeout:                        10 * time.Second,
				RateLimitRequests:              100,
				RateLimitPeriod:                time.Second,
				CircuitBreakerEnabled:          true,
				CircuitBreakerFailThreshold:    5,
				CircuitBreakerSuccessThreshold: 0,
			},
			wantErr: true,
			errMsg:  "CircuitBreakerSuccessThreshold",
		},
		{
			name: "invalid_circuit_breaker_timeout",
			config: &Config{
				Exchange:                       "binance",
				Timeout:                        10 * time.Second,
				RateLimitRequests:              100,
				RateLimitPeriod:                time.Second,
				CircuitBreakerEnabled:          true,
				CircuitBreakerFailThreshold:    5,
				CircuitBreakerSuccessThreshold: 2,
				CircuitBreakerTimeout:          0,
			},
			wantErr: true,
			errMsg:  "CircuitBreakerTimeout",
		},
		{
			name: "circuit_breaker_disabled_skips_validation",
			config: &Config{
				Exchange:                       "binance",
				Timeout:                        10 * time.Second,
				RateLimitRequests:              100,
				RateLimitPeriod:                time.Second,
				CircuitBreakerEnabled:          false,
				CircuitBreakerFailThreshold:    0,
				CircuitBreakerSuccessThreshold: 0,
				CircuitBreakerTimeout:          0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, strings.Contains(err.Error(), tt.errMsg), "expected error to contain %q, got %q", tt.errMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_WithCredentials(t *testing.T) {
	config := DefaultConfig("binance")
	creds := &Credentials{
		APIKey:    "test-key",
		SecretKey: "test-secret",
	}

	result := config.WithCredentials(creds)

	assert.Equal(t, config, result)
	assert.Equal(t, creds, config.Credentials)
}

func TestConfig_WithSandbox(t *testing.T) {
	config := DefaultConfig("binance")
	result := config.WithSandbox(true)

	assert.Equal(t, config, result)
	assert.True(t, config.Sandbox)
}

func TestConfig_WithTimeout(t *testing.T) {
	config := DefaultConfig("binance")
	result := config.WithTimeout(30 * time.Second)

	assert.Equal(t, config, result)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestConfig_WithRateLimit(t *testing.T) {
	config := DefaultConfig("binance")
	result := config.WithRateLimit(100, 10*time.Second)

	assert.Equal(t, config, result)
	assert.Equal(t, 100, config.RateLimitRequests)
	assert.Equal(t, 10*time.Second, config.RateLimitPeriod)
}

func TestConfig_WithCache(t *testing.T) {
	config := DefaultConfig("binance")
	result := config.WithCache(false, 5*time.Second)

	assert.Equal(t, config, result)
	assert.False(t, config.CacheEnabled)
	assert.Equal(t, 5*time.Second, config.CacheTTL)
}
