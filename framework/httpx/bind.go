package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Alfian57/ginkit/framework/validation"
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
	instance := validation.Default
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
		return value, false
	}
	return value, true
}

func invalidJSON(cause error) *Error {
	var tooLarge *http.MaxBytesError
	if errors.As(cause, &tooLarge) {
		return WrapError(http.StatusRequestEntityTooLarge, "body_too_large", "The request body is too large.", cause)
	}
	return WrapError(http.StatusBadRequest, "invalid_json", "The request body must contain valid JSON.", cause)
}
