package apptest

import (
	"net/http"
	"testing"

	"github.com/Alfian57/gin-kit/framework"
	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-gonic/gin"
)

type echoBody struct {
	Name string `json:"name"`
}

func TestAppRequestAndEnvelopeDecoding(t *testing.T) {
	app := New(t, framework.Options{})
	app.Router().POST("/echo", func(c *gin.Context) {
		value, ok := httpx.BindJSON[echoBody](c)
		if !ok {
			return
		}
		httpx.Created(c, value)
	})

	var out echoBody
	app.POST("/echo", echoBody{Name: "gin-kit"}).RequireStatus(http.StatusCreated).Data(&out)
	if out.Name != "gin-kit" {
		t.Fatalf("decoded data = %+v", out)
	}

	failure := app.GET("/absent").RequireStatus(http.StatusNotFound).Err()
	if failure.Code != "not_found" || failure.RequestID == "" {
		t.Fatalf("error envelope = %+v", failure)
	}
}

func TestAppSendsHeaders(t *testing.T) {
	app := New(t, framework.Options{})
	app.Router().GET("/header", func(c *gin.Context) {
		httpx.OK(c, gin.H{"value": c.GetHeader("X-Custom")})
	})
	response := app.Do(http.MethodGet, "/header", nil, http.Header{"X-Custom": []string{"present"}})
	var out struct {
		Value string `json:"value"`
	}
	response.RequireStatus(http.StatusOK).Data(&out)
	if out.Value != "present" {
		t.Fatalf("header not forwarded: %+v", out)
	}
}
