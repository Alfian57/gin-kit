package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/runtime/validation"
	"github.com/gin-gonic/gin"
)

type searchQuery struct {
	Term string `form:"term" validate:"required,min=3"`
	Age  int    `form:"age"`
}

type taskPath struct {
	ID int `uri:"id" json:"id" validate:"required,min=1"`
}

func TestBindQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name     string
		query    string
		status   int
		contains string
		absent   string
	}{
		{"valid", "term=hello&age=30", http.StatusOK, `"term":"hello"`, ""},
		{"type mismatch never echoes value", "term=hello&age=thirty", http.StatusBadRequest, `"code":"invalid_query"`, "thirty"},
		{"validation failure uses form names", "term=hi", http.StatusUnprocessableEntity, `"term"`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/search", func(c *gin.Context) {
				value, ok := BindQuery[searchQuery](c)
				if !ok {
					return
				}
				OK(c, gin.H{"term": value.Term})
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/search?"+test.query, nil))
			body := recorder.Body.String()
			if recorder.Code != test.status || !strings.Contains(body, test.contains) {
				t.Fatalf("status=%d body=%s", recorder.Code, body)
			}
			if test.absent != "" && strings.Contains(body, test.absent) {
				t.Fatalf("response echoed submitted value: %s", body)
			}
		})
	}
}

func TestBindQueryUsesCustomValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	custom := validation.New()
	custom.RegisterMessage("min", "Value for {field} is too short.")
	router := gin.New()
	router.GET("/search", func(c *gin.Context) {
		if _, ok := BindQuery[searchQuery](c, custom); !ok {
			return
		}
		OK(c, "ok")
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/search?term=hi", nil))
	if recorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(recorder.Body.String(), "Value for term is too short.") {
		t.Fatalf("custom validator not used: %d %s", recorder.Code, recorder.Body)
	}
}

func TestBindUsesContextValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	appValidator := validation.New()
	appValidator.RegisterMessage("min", "Context says {field} is too short.")
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(ValidatorKey, appValidator) })
	router.GET("/search", func(c *gin.Context) {
		if _, ok := BindQuery[searchQuery](c); !ok {
			return
		}
		OK(c, "ok")
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/search?term=hi", nil))
	if recorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(recorder.Body.String(), "Context says term is too short.") {
		t.Fatalf("context validator not used: %d %s", recorder.Code, recorder.Body)
	}
}

func TestExplicitValidatorBeatsContextValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fromContext := validation.New()
	fromContext.RegisterMessage("min", "context message")
	explicit := validation.New()
	explicit.RegisterMessage("min", "explicit message")
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(ValidatorKey, fromContext) })
	router.GET("/search", func(c *gin.Context) {
		if _, ok := BindQuery[searchQuery](c, explicit); !ok {
			return
		}
		OK(c, "ok")
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/search?term=hi", nil))
	if !strings.Contains(recorder.Body.String(), "explicit message") {
		t.Fatalf("explicit validator did not win: %s", recorder.Body)
	}
}

func TestBindURI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name     string
		path     string
		status   int
		contains string
	}{
		{"valid", "/tasks/7", http.StatusOK, `"id":7`},
		{"malformed", "/tasks/abc", http.StatusBadRequest, `"code":"invalid_path"`},
		{"validation failure", "/tasks/0", http.StatusUnprocessableEntity, `"code":"validation_failed"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/tasks/:id", func(c *gin.Context) {
				value, ok := BindURI[taskPath](c)
				if !ok {
					return
				}
				OK(c, gin.H{"id": value.ID})
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.contains) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body)
			}
		})
	}
}
