package core

import "errors"

type ErrorCode string

const (
	ErrCodeNetwork           ErrorCode = "NETWORK_ERROR"
	ErrCodeTimeout           ErrorCode = "TIMEOUT"
	ErrCodeRateLimit         ErrorCode = "RATE_LIMIT"
	ErrCodeAuth              ErrorCode = "AUTH_ERROR"
	ErrCodeBadRequest        ErrorCode = "BAD_REQUEST"
	ErrCodeNotFound          ErrorCode = "NOT_FOUND"
	ErrCodeServerError       ErrorCode = "SERVER_ERROR"
	ErrCodeInsufficientFunds ErrorCode = "INSUFFICIENT_FUNDS"
	ErrCodeInvalidOrder      ErrorCode = "INVALID_ORDER"
	ErrCodeInvalidSymbol     ErrorCode = "INVALID_SYMBOL"
)

func IsErrorCode(err error, code ErrorCode) bool {
	var exErr *ExchangeError
	if errors.As(err, &exErr) {
		return ErrorCode(exErr.Code) == code
	}
	return false
}
