package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Alfian57/gin-kit/runtime/validation"
	"github.com/gin-gonic/gin"
)

// BindJSON decodes one JSON value, rejects unknown fields and trailing JSON,
// validates it, and writes a stable error response when it fails.
func BindJSON[T any](c *gin.Context, validators ...*validation.Validator) (T, bool) {
	var value T
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		Fail(c, invalidJSON(err))
		return value, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		Fail(c, invalidJSON(err))
		return value, false
	}
	return value, validateBound(c, value, validators)
}

// BindQuery binds query parameters through `form` struct tags, validates the
// result, and writes a stable error response when it fails. Parameter values
// are never echoed back to the client.
func BindQuery[T any](c *gin.Context, validators ...*validation.Validator) (T, bool) {
	var value T
	if err := c.ShouldBindQuery(&value); err != nil {
		Fail(c, WrapError(http.StatusBadRequest, "invalid_query", "The request query parameters are invalid.", err))
		return value, false
	}
	return value, validateBound(c, value, validators)
}

// BindURI binds path parameters through `uri` struct tags, validates the
// result, and writes a stable error response when it fails. Give URI fields
// matching `json` or `form` tags so validation errors use readable names.
func BindURI[T any](c *gin.Context, validators ...*validation.Validator) (T, bool) {
	var value T
	if err := c.ShouldBindUri(&value); err != nil {
		Fail(c, WrapError(http.StatusBadRequest, "invalid_path", "The request path parameters are invalid.", err))
		return value, false
	}
	return value, validateBound(c, value, validators)
}

// validateBound resolves the validator as: explicit argument, then the
// application validator stored in the context, then validation.Default.
func validateBound(c *gin.Context, value any, validators []*validation.Validator) bool {
	instance := validation.Default
	if fromContext := contextValidator(c); fromContext != nil {
		instance = fromContext
	}
	if len(validators) > 0 && validators[0] != nil {
		instance = validators[0]
	}
	if err := instance.Struct(value); err != nil {
		var failures *validation.Errors
		if errors.As(err, &failures) {
			Fail(c, ValidationError(failures))
		} else {
			Fail(c, WrapError(http.StatusInternalServerError, "validation_unavailable", "Request validation is unavailable.", err))
		}
		return false
	}
	return true
}

func contextValidator(c *gin.Context) *validation.Validator {
	if value, exists := c.Get(ValidatorKey); exists {
		if v, ok := value.(*validation.Validator); ok && v != nil {
			return v
		}
	}
	return nil
}

func invalidJSON(cause error) *Error {
	var tooLarge *http.MaxBytesError
	if errors.As(cause, &tooLarge) {
		return WrapError(http.StatusRequestEntityTooLarge, "body_too_large", "The request body is too large.", cause)
	}
	return WrapError(http.StatusBadRequest, "invalid_json", "The request body must contain valid JSON.", cause)
}
