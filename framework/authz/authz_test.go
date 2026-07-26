package authz

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestAuthorizeAllowWritesNothing(t *testing.T) {
	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		if !Authorize(c, Allow()) {
			return
		}
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusNoContent || recorder.Body.Len() != 0 {
		t.Fatalf("allow must pass through untouched: %d %q", recorder.Code, recorder.Body)
	}
}

func TestAuthorizeDenyWritesForbiddenAndAborts(t *testing.T) {
	reached := false
	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		if !Authorize(c, Deny("subject does not own the ticket")) {
			return
		}
		c.Status(http.StatusNoContent)
	}, func(*gin.Context) {
		reached = true
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"forbidden"`) {
		t.Fatalf("stable code missing: %s", body)
	}
	if strings.Contains(body, "subject does not own the ticket") {
		t.Fatalf("deny reason leaked into the response: %s", body)
	}
	if reached {
		t.Fatal("deny did not abort the handler chain")
	}
}

func TestErrContract(t *testing.T) {
	if err := Allow().Err(); err != nil {
		t.Fatalf("allow produced an error: %v", err)
	}
	err := Deny("account suspended").Err()
	var public *httpx.Error
	if !errors.As(err, &public) {
		t.Fatalf("deny did not produce a *httpx.Error: %v", err)
	}
	if public.Status != http.StatusForbidden || public.Code != "forbidden" ||
		public.Message != "You are not allowed to perform this action." {
		t.Fatalf("unstable public shape: %#v", public)
	}
	if public.Cause == nil || !strings.Contains(public.Cause.Error(), "account suspended") {
		t.Fatalf("deny reason must travel as the wrapped cause: %#v", public)
	}
}
