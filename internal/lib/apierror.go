package lib

import "net/http"

// ErrorCode is a machine-readable error identifier.
type ErrorCode string

const (
	ErrCodeBadRequest       ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden        ErrorCode = "FORBIDDEN"
	ErrCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrCodeConflict         ErrorCode = "CONFLICT"
	ErrCodeInternal         ErrorCode = "INTERNAL_ERROR"
	ErrCodeTooManyRequests  ErrorCode = "TOO_MANY_REQUESTS"
	ErrCodeUnprocessable    ErrorCode = "UNPROCESSABLE_ENTITY"
	ErrCodeValidation       ErrorCode = "VALIDATION_ERROR"
)

// APIError is a structured error returned from the API layer.
type APIError struct {
	Code    ErrorCode         `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	Status  int               `json:"-"`
}

func (e *APIError) Error() string { return e.Message }

// Sentinel errors — use these for typed comparisons in services and handlers.
var (
	ErrBadRequest      = &APIError{Code: ErrCodeBadRequest, Message: "invalid request", Status: http.StatusBadRequest}
	ErrUnauthorized    = &APIError{Code: ErrCodeUnauthorized, Message: "unauthorized", Status: http.StatusUnauthorized}
	ErrForbidden       = &APIError{Code: ErrCodeForbidden, Message: "forbidden", Status: http.StatusForbidden}
	ErrNotFound        = &APIError{Code: ErrCodeNotFound, Message: "resource not found", Status: http.StatusNotFound}
	ErrConflict        = &APIError{Code: ErrCodeConflict, Message: "resource already exists", Status: http.StatusConflict}
	ErrInternalServer  = &APIError{Code: ErrCodeInternal, Message: "internal server error", Status: http.StatusInternalServerError}
	ErrTooManyRequests = &APIError{Code: ErrCodeTooManyRequests, Message: "too many requests", Status: http.StatusTooManyRequests}
)

// NewAPIError creates an APIError with the given code, message, and HTTP status.
func NewAPIError(code ErrorCode, message string, status int) *APIError {
	return &APIError{Code: code, Message: message, Status: status}
}

// BadRequest returns a 400 error with a custom message.
func BadRequest(msg string) *APIError {
	return &APIError{Code: ErrCodeBadRequest, Message: msg, Status: http.StatusBadRequest}
}

// BadRequestWithDetails returns a 400 error with field-level validation details.
func BadRequestWithDetails(msg string, details map[string]string) *APIError {
	return &APIError{Code: ErrCodeValidation, Message: msg, Status: http.StatusBadRequest, Details: details}
}

// Unauthorized returns a 401 error with a custom message.
func Unauthorized(msg string) *APIError {
	return &APIError{Code: ErrCodeUnauthorized, Message: msg, Status: http.StatusUnauthorized}
}

// Forbidden returns a 403 error with a custom message.
func Forbidden(msg string) *APIError {
	return &APIError{Code: ErrCodeForbidden, Message: msg, Status: http.StatusForbidden}
}

// NotFound returns a 404 error with a custom message.
func NotFound(msg string) *APIError {
	return &APIError{Code: ErrCodeNotFound, Message: msg, Status: http.StatusNotFound}
}

// Conflict returns a 409 error with a custom message.
func Conflict(msg string) *APIError {
	return &APIError{Code: ErrCodeConflict, Message: msg, Status: http.StatusConflict}
}

// InternalError returns a 500 error with a custom message.
func InternalError(msg string) *APIError {
	return &APIError{Code: ErrCodeInternal, Message: msg, Status: http.StatusInternalServerError}
}

// TooManyRequests returns a 429 error.
func TooManyRequests(msg string) *APIError {
	return &APIError{Code: ErrCodeTooManyRequests, Message: msg, Status: http.StatusTooManyRequests}
}

// UnprocessableEntity returns a 422 error.
func UnprocessableEntity(msg string) *APIError {
	return &APIError{Code: ErrCodeUnprocessable, Message: msg, Status: http.StatusUnprocessableEntity}
}
