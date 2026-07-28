package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// init initializes package-level implementation state.
func init() { gin.SetMode(gin.TestMode) }

func TestBindJSONValidationContractAndRedaction(t *testing.T) {
	type request struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"min=12"`
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(RequestIDKey, "request-123")
		c.Next()
	})
	router.POST("/", func(c *gin.Context) {
		value, ok := BindJSON[request](c)
		if ok {
			Created(c, value)
		}
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"","password":"secret"}`)))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Fatal("submitted secret leaked into response")
	}
	var body ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "validation_failed" || body.Error.RequestID != "request-123" {
		t.Fatalf("unexpected error: %#v", body.Error)
	}
	details := body.Error.Details.(map[string]any)
	fields := details["fields"].(map[string]any)
	if _, ok := fields["email"]; !ok {
		t.Fatalf("email details missing: %#v", fields)
	}
}

func TestBindJSONRejectsMalformedUnknownAndTrailingJSON(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}
	for _, body := range []string{`{"name":`, `{"unknown":true}`, `{"name":"ok"} {}`} {
		t.Run(body, func(t *testing.T) {
			router := gin.New()
			router.POST("/", func(c *gin.Context) { _, _ = BindJSON[request](c) })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"invalid_json"`) {
				t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body)
			}
		})
	}
}

func TestDefaultMapperHidesInternalCause(t *testing.T) {
	mapped := DefaultMapper(errors.New("database password=secret"), nil)
	if mapped.Code != "internal_error" || mapped.Message == mapped.Error() {
		t.Fatalf("internal cause was not separated: %#v", mapped)
	}
}
