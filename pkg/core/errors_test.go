package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		want      string
	}{
		{"unknown", ErrorTypeUnknown, "UNKNOWN"},
		{"network", ErrorTypeNetwork, "NETWORK"},
		{"timeout", ErrorTypeTimeout, "TIMEOUT"},
		{"rate_limit", ErrorTypeRateLimit, "RATE_LIMIT"},
		{"authentication", ErrorTypeAuthentication, "AUTHENTICATION"},
		{"bad_request", ErrorTypeBadRequest, "BAD_REQUEST"},
		{"not_found", ErrorTypeNotFound, "NOT_FOUND"},
		{"server_error", ErrorTypeServerError, "SERVER_ERROR"},
		{"insufficient_funds", ErrorTypeInsufficientFunds, "INSUFFICIENT_FUNDS"},
		{"invalid_order", ErrorTypeInvalidOrder, "INVALID_ORDER"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.errorType.String())
		})
	}
}

func TestExchangeError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ExchangeError
		want string
	}{
		{
			name: "without_code",
			err: &ExchangeError{
				Exchange:   "binance",
				Type:       ErrorTypeRateLimit,
				StatusCode: 429,
				Message:    "too many requests",
			},
			want: "[binance] RATE_LIMIT (429): too many requests",
		},
		{
			name: "with_code",
			err: &ExchangeError{
				Exchange:   "binance",
				Type:       ErrorTypeInvalidOrder,
				StatusCode: 400,
				Code:       "INVALID_QUANTITY",
				Message:    "invalid order quantity",
			},
			want: "[binance] INVALID_ORDER (400/INVALID_QUANTITY): invalid order quantity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestNewExchangeError(t *testing.T) {
	err := NewExchangeError("binance", ErrorTypeNetwork, 503, "service unavailable")

	assert.NotNil(t, err)
	assert.Equal(t, "binance", err.Exchange)
	assert.Equal(t, ErrorTypeNetwork, err.Type)
	assert.Equal(t, 503, err.StatusCode)
	assert.Equal(t, "service unavailable", err.Message)
	assert.False(t, err.Timestamp.IsZero())
}

func TestNewExchangeErrorWithCode(t *testing.T) {
	err := NewExchangeErrorWithCode("coinbase", ErrorTypeAuthentication, 401, "UNAUTHORIZED", "invalid api key")

	assert.NotNil(t, err)
	assert.Equal(t, "coinbase", err.Exchange)
	assert.Equal(t, ErrorTypeAuthentication, err.Type)
	assert.Equal(t, 401, err.StatusCode)
	assert.Equal(t, "UNAUTHORIZED", err.Code)
	assert.Equal(t, "invalid api key", err.Message)
}

func TestIsNetworkError(t *testing.T) {
	networkErr := NewExchangeError("test", ErrorTypeNetwork, 500, "network error")
	authErr := NewExchangeError("test", ErrorTypeAuthentication, 401, "auth error")

	assert.True(t, IsNetworkError(networkErr))
	assert.False(t, IsNetworkError(authErr))
	assert.False(t, IsNetworkError(nil))
}

func TestIsTimeoutError(t *testing.T) {
	timeoutErr := NewExchangeError("test", ErrorTypeTimeout, 408, "timeout")
	networkErr := NewExchangeError("test", ErrorTypeNetwork, 500, "network error")

	assert.True(t, IsTimeoutError(timeoutErr))
	assert.False(t, IsTimeoutError(networkErr))
	assert.False(t, IsTimeoutError(nil))
}

func TestIsRateLimitError(t *testing.T) {
	rateLimitErr := NewExchangeError("test", ErrorTypeRateLimit, 429, "rate limited")
	networkErr := NewExchangeError("test", ErrorTypeNetwork, 500, "network error")

	assert.True(t, IsRateLimitError(rateLimitErr))
	assert.False(t, IsRateLimitError(networkErr))
	assert.False(t, IsRateLimitError(nil))
}

func TestIsAuthenticationError(t *testing.T) {
	authErr := NewExchangeError("test", ErrorTypeAuthentication, 401, "unauthorized")
	networkErr := NewExchangeError("test", ErrorTypeNetwork, 500, "network error")

	assert.True(t, IsAuthenticationError(authErr))
	assert.False(t, IsAuthenticationError(networkErr))
	assert.False(t, IsAuthenticationError(nil))
}

func TestIsTerminalError(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		terminal bool
	}{
		{"insufficient_funds", ErrorTypeInsufficientFunds, true},
		{"invalid_order", ErrorTypeInvalidOrder, true},
		{"not_found", ErrorTypeNotFound, true},
		{"network", ErrorTypeNetwork, false},
		{"timeout", ErrorTypeTimeout, false},
		{"rate_limit", ErrorTypeRateLimit, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewExchangeError("test", tt.errType, 500, "message")
			assert.Equal(t, tt.terminal, IsTerminalError(err))
		})
	}

	assert.False(t, IsTerminalError(nil))
}
