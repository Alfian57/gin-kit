package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Alfian57/gin-kit/framework/validation"
	"github.com/gin-gonic/gin"
)

const RequestIDKey = "request_id"

// LoggerKey is the gin context key under which the framework stores the
// request-scoped logger.
const LoggerKey = "logger"

// Logger returns the request-scoped logger stored by the framework,
// pre-populated with the request ID, method, and path. It falls back to
// slog.Default() when absent, for example in tests without the framework
// middleware.
func Logger(c *gin.Context) *slog.Logger {
	if value, exists := c.Get(LoggerKey); exists {
		if logger, ok := value.(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return slog.Default()
}

type Envelope struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Error is a public HTTP error. Cause is retained for logging and never serialized.
type Error struct {
	Status  int
	Code    string
	Message string
	Details any
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func WrapError(status int, code, message string, cause error) *Error {
	return &Error{Status: status, Code: code, Message: message, Cause: cause}
}

func ValidationError(failures *validation.Errors) *Error {
	return &Error{
		Status:  http.StatusUnprocessableEntity,
		Code:    "validation_failed",
		Message: "The given data was invalid.",
		Details: failures,
		Cause:   failures,
	}
}

type Mapper func(error, *gin.Context) *Error

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

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Data: data})
}

func List(c *gin.Context, data, meta any) {
	c.JSON(http.StatusOK, Envelope{Data: data, Meta: meta})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

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

func Handle(c *gin.Context, mapper Mapper, err error) {
	if mapper == nil {
		mapper = DefaultMapper
	}
	Fail(c, mapper(err, c))
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
