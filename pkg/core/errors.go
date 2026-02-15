package core

import (
	"errors"
	"fmt"
	"time"
)

// ErrorType represents the category of an exchange error.
type ErrorType int

// Error type constants categorize errors for proper handling and retry logic.
const (
	// ErrorTypeUnknown indicates an unclassified error.
	ErrorTypeUnknown ErrorType = iota
	// ErrorTypeNetwork indicates a network connectivity issue.
	ErrorTypeNetwork
	// ErrorTypeTimeout indicates the request exceeded its deadline.
	ErrorTypeTimeout
	// ErrorTypeRateLimit indicates rate limit was exceeded.
	ErrorTypeRateLimit
	// ErrorTypeAuthentication indicates invalid or expired credentials.
	ErrorTypeAuthentication
	// ErrorTypeBadRequest indicates invalid request parameters.
	ErrorTypeBadRequest
	// ErrorTypeNotFound indicates the requested resource does not exist.
	ErrorTypeNotFound
	// ErrorTypeServerError indicates a server-side error.
	ErrorTypeServerError
	// ErrorTypeInsufficientFunds indicates account lacks required balance.
	ErrorTypeInsufficientFunds
	// ErrorTypeInvalidOrder indicates the order violates exchange rules.
	ErrorTypeInvalidOrder
)

// String returns the string representation of the error type.
func (t ErrorType) String() string {
	return [...]string{
		"UNKNOWN",
		"NETWORK",
		"TIMEOUT",
		"RATE_LIMIT",
		"AUTHENTICATION",
		"BAD_REQUEST",
		"NOT_FOUND",
		"SERVER_ERROR",
		"INSUFFICIENT_FUNDS",
		"INVALID_ORDER",
	}[t]
}

// Sentinel errors for common error conditions.
var (
	// ErrClientClosed is returned when attempting to use a closed client.
	ErrClientClosed = errors.New("client is closed")
	// ErrStreamClosed is returned when attempting to use a closed stream.
	ErrStreamClosed = errors.New("stream is closed")
	// ErrNotConnected is returned when WebSocket is not connected.
	ErrNotConnected = errors.New("websocket not connected")
	// ErrCircuitBreakerOpen is returned when circuit breaker is open.
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
	// ErrNoCredentials is returned when no API credentials are configured.
	ErrNoCredentials = errors.New("no credentials configured")
	// ErrNoAPIKey is returned when no API key is available.
	ErrNoAPIKey = errors.New("no available API key")
)

// ExchangeError represents a structured error returned from an exchange.
// It provides detailed context for debugging and error handling.
type ExchangeError struct {
	// Type categorizes the error for programmatic handling.
	Type ErrorType `json:"type"`
	// StatusCode is the HTTP status code from the response.
	StatusCode int `json:"status_code"`
	// Code is the exchange-specific error code.
	Code string `json:"code"`
	// Message is the human-readable error description.
	Message string `json:"message"`
	// RawError contains the original error response for debugging.
	RawError any `json:"raw_error,omitempty"`
	// Exchange identifies which exchange returned this error.
	Exchange string `json:"exchange"`
	// Timestamp is when the error occurred.
	Timestamp time.Time `json:"timestamp"`
}

// Error implements the error interface for ExchangeError.
// It returns a formatted string with exchange name, error type, status code, and message.
func (e *ExchangeError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s (%d/%s): %s",
			e.Exchange, e.Type, e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("[%s] %s (%d): %s",
		e.Exchange, e.Type, e.StatusCode, e.Message)
}

// WithCode returns a new ExchangeError with the specified error code.
func (e *ExchangeError) WithCode(code ErrorCode) *ExchangeError {
	e.Code = string(code)
	return e
}

// NewExchangeError creates a new ExchangeError with the specified details.
// The timestamp is automatically set to the current time.
func NewExchangeError(exchange string, errorType ErrorType, statusCode int, message string) *ExchangeError {
	return &ExchangeError{
		Type:       errorType,
		StatusCode: statusCode,
		Message:    message,
		Exchange:   exchange,
		Timestamp:  time.Now(),
	}
}

// NewExchangeErrorWithCode creates a new ExchangeError including an exchange-specific error code.
// The timestamp is automatically set to the current time.
func NewExchangeErrorWithCode(exchange string, errorType ErrorType, statusCode int, code, message string) *ExchangeError {
	return &ExchangeError{
		Type:       errorType,
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
		Exchange:   exchange,
		Timestamp:  time.Now(),
	}
}

// IsNetworkError returns true if the error is a network connectivity issue.
// Network errors are typically retryable.
func IsNetworkError(err error) bool {
	if e, ok := err.(*ExchangeError); ok {
		return e.Type == ErrorTypeNetwork
	}
	return false
}

// IsTimeoutError returns true if the error is a timeout.
// Timeout errors are typically retryable with a longer deadline.
func IsTimeoutError(err error) bool {
	if e, ok := err.(*ExchangeError); ok {
		return e.Type == ErrorTypeTimeout
	}
	return false
}

// IsRateLimitError returns true if the error is a rate limit violation.
// Rate limit errors should be retried after a delay.
func IsRateLimitError(err error) bool {
	if e, ok := err.(*ExchangeError); ok {
		return e.Type == ErrorTypeRateLimit
	}
	return false
}

// IsAuthenticationError returns true if the error is an authentication failure.
// Authentication errors require credential validation and are not retryable.
func IsAuthenticationError(err error) bool {
	if e, ok := err.(*ExchangeError); ok {
		return e.Type == ErrorTypeAuthentication
	}
	return false
}

// IsTerminalError returns true if the error indicates a terminal condition.
// Terminal errors should not be retried as they will not succeed.
func IsTerminalError(err error) bool {
	if e, ok := err.(*ExchangeError); ok {
		return e.Type == ErrorTypeInsufficientFunds ||
			e.Type == ErrorTypeInvalidOrder ||
			e.Type == ErrorTypeNotFound
	}
	return false
}
