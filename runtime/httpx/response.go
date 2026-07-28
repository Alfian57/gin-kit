package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Alfian57/gin-kit/runtime/validation"
	"github.com/gin-gonic/gin"
)

// RequestIDKey is the Gin context key for the request identifier.
const RequestIDKey = "request_id"

// LoggerKey is the gin context key under which the runtime stores the
// request-scoped logger.
const LoggerKey = "logger"

// ValidatorKey is the gin context key under which the runtime stores the
// application validator, so binders resolve app-registered rules and
// messages without every call site passing the validator explicitly.
const ValidatorKey = "validator"

// Logger returns the request-scoped logger stored by the runtime,
// pre-populated with the request ID, method, and path. It falls back to
// slog.Default() when absent, for example in tests without the runtime
// middleware.
func Logger(c *gin.Context) *slog.Logger {
	if value, exists := c.Get(LoggerKey); exists {
		if logger, ok := value.(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return slog.Default()
}

// Envelope is the canonical successful JSON response shape.
type Envelope struct {
	// Data is the response payload.
	Data any `json:"data"`
	// Meta contains optional pagination or response metadata.
	Meta any `json:"meta,omitempty"`
}

// ErrorEnvelope is the canonical failed JSON response shape.
type ErrorEnvelope struct {
	// Error contains the safe public error details.
	Error ErrorBody `json:"error"`
}

// ErrorBody contains the serializable portion of one failed response.
type ErrorBody struct {
	// Code is the stable, machine-readable error code.
	Code string `json:"code"`
	// Message is the safe human-readable error message.
	Message string `json:"message"`
	// Details optionally carries safe structured error details.
	Details any `json:"details,omitempty"`
	// RequestID correlates the response with structured server logs.
	RequestID string `json:"request_id,omitempty"`
}

// Error is a public HTTP error. Cause is retained for logging and never serialized.
type Error struct {
	// Status is the HTTP status code written to the response.
	Status int
	// Code is the stable application error code.
	Code string
	// Message is safe to serialize to clients.
	Message string
	// Details carries optional safe structured data.
	Details any
	// Cause retains the original internal error for logging and errors.Is checks.
	Cause error
}

// Error returns the internal cause message when available, otherwise Message.
func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Message
}

// Unwrap exposes Cause to Go error inspection without serializing it.
func (e *Error) Unwrap() error { return e.Cause }

// NewError constructs a public error without an internal cause.
func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// WrapError constructs a public error while preserving an internal cause.
func WrapError(status int, code, message string, cause error) *Error {
	return &Error{Status: status, Code: code, Message: message, Cause: cause}
}

// ValidationError converts structured validation failures into the canonical
// 422 validation_failed response.
func ValidationError(failures *validation.Errors) *Error {
	return &Error{
		Status:  http.StatusUnprocessableEntity,
		Code:    "validation_failed",
		Message: "The given data was invalid.",
		Details: failures,
		Cause:   failures,
	}
}

// Mapper converts an arbitrary handler error into a safe public Error.
type Mapper func(error, *gin.Context) *Error

// DefaultMapper preserves public and validation errors; every other error
// becomes a generic internal_error without exposing its cause.
func DefaultMapper(err error, _ *gin.Context) *Error {
	var public *Error
	if errors.As(err, &public) {
		return public
	}
	var failures *validation.Errors
	if errors.As(err, &failures) {
		return ValidationError(failures)
	}
	return WrapError(http.StatusInternalServerError, "internal_error", "An unexpected error occurred.", err)
}

// OK writes a 200 response using the canonical success envelope.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Data: data})
}

// Created writes a 201 response using the canonical success envelope.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Data: data})
}

// List writes a 200 response with data and optional pagination metadata.
func List(c *gin.Context, data, meta any) {
	c.JSON(http.StatusOK, Envelope{Data: data, Meta: meta})
}

// NoContent writes an empty 204 response.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Fail aborts the request with a validated public error envelope.
func Fail(c *gin.Context, err *Error) {
	if err == nil {
		err = NewError(http.StatusInternalServerError, "internal_error", "An unexpected error occurred.")
	}
	status := err.Status
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	requestID, _ := c.Get(RequestIDKey)
	c.AbortWithStatusJSON(status, ErrorEnvelope{Error: ErrorBody{
		Code:      err.Code,
		Message:   err.Message,
		Details:   err.Details,
		RequestID: stringValue(requestID),
	}})
}

// Handle maps an arbitrary error and aborts the request with the result.
func Handle(c *gin.Context, mapper Mapper, err error) {
	if mapper == nil {
		mapper = DefaultMapper
	}
	Fail(c, mapper(err, c))
}

// stringValue returns value when it is a string, otherwise the empty string.
func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
