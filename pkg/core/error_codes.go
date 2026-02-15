package core

import "errors"

// ErrorCode represents an exchange-specific error identifier.
// Error codes provide a stable, machine-readable way to identify specific error conditions.
type ErrorCode string

// Error code constants define standardized error identifiers across all exchanges.
const (
	// ErrCodeNetwork indicates a network connectivity failure.
	ErrCodeNetwork ErrorCode = "NETWORK_ERROR"
	// ErrCodeTimeout indicates the request exceeded its deadline.
	ErrCodeTimeout ErrorCode = "TIMEOUT"
	// ErrCodeRateLimit indicates the rate limit was exceeded.
	ErrCodeRateLimit ErrorCode = "RATE_LIMIT"
	// ErrCodeAuth indicates authentication or authorization failure.
	ErrCodeAuth ErrorCode = "AUTH_ERROR"
	// ErrCodeBadRequest indicates invalid request parameters.
	ErrCodeBadRequest ErrorCode = "BAD_REQUEST"
	// ErrCodeNotFound indicates the requested resource was not found.
	ErrCodeNotFound ErrorCode = "NOT_FOUND"
	// ErrCodeServerError indicates a server-side error occurred.
	ErrCodeServerError ErrorCode = "SERVER_ERROR"
	// ErrCodeInsufficientFunds indicates the account lacks required balance.
	ErrCodeInsufficientFunds ErrorCode = "INSUFFICIENT_FUNDS"
	// ErrCodeInvalidOrder indicates the order violates exchange rules.
	ErrCodeInvalidOrder ErrorCode = "INVALID_ORDER"
	// ErrCodeInvalidSymbol indicates the trading pair is not recognized.
	ErrCodeInvalidSymbol ErrorCode = "INVALID_SYMBOL"

	// Configuration errors
	ErrCodeInvalidConfig ErrorCode = "INVALID_CONFIG"

	// Client state errors
	ErrCodeClientClosed ErrorCode = "CLIENT_CLOSED"
	ErrCodeInvalidState ErrorCode = "INVALID_STATE"

	// Stream/WebSocket errors
	ErrCodeStreamClosed ErrorCode = "STREAM_CLOSED"
	ErrCodeNotConnected ErrorCode = "NOT_CONNECTED"

	// Circuit breaker errors
	ErrCodeCircuitBreaker ErrorCode = "CIRCUIT_BREAKER_OPEN"

	// Authentication errors
	ErrCodeNoCredentials ErrorCode = "NO_CREDENTIALS"
	ErrCodeNoAPIKey      ErrorCode = "NO_API_KEY"

	// Unsupported operation
	ErrCodeUnsupported ErrorCode = "UNSUPPORTED_METHOD"
)

// IsErrorCode checks if the error matches the specified error code.
// It extracts the exchange error and compares its code field against the provided ErrorCode.
func IsErrorCode(err error, code ErrorCode) bool {
	var exErr *ExchangeError
	if errors.As(err, &exErr) {
		return ErrorCode(exErr.Code) == code
	}
	return false
}
