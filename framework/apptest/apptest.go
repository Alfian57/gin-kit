// Package apptest provides small helpers for exercising a gin-kit application
// in tests and decoding its envelope responses. Assertions beyond status
// checks stay in plain Go.
package apptest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alfian57/gin-kit/framework"
	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-gonic/gin"
)

// App wraps a framework application for in-process HTTP testing.
type App struct {
	t   *testing.T
	app *framework.Application
}

// New builds an application from options and fails the test on error.
func New(t *testing.T, options framework.Options) *App {
	t.Helper()
	gin.SetMode(gin.TestMode)
	application, err := framework.New(options)
	if err != nil {
		t.Fatalf("apptest: %v", err)
	}
	return &App{t: t, app: application}
}

// Router exposes the underlying engine for route registration.
func (a *App) Router() *gin.Engine { return a.app.Router() }

// Application exposes the wrapped framework application.
func (a *App) Application() *framework.Application { return a.app }

// Do performs an in-process request. A non-nil body is JSON-encoded and sent
// with a JSON content type. Extra headers are optional.
func (a *App) Do(method, path string, body any, header http.Header) *Response {
	a.t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			a.t.Fatalf("apptest: encode request body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, values := range header {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	recorder := httptest.NewRecorder()
	a.app.Router().ServeHTTP(recorder, request)
	return &Response{ResponseRecorder: recorder, t: a.t}
}

func (a *App) GET(path string) *Response            { return a.Do(http.MethodGet, path, nil, nil) }
func (a *App) POST(path string, body any) *Response { return a.Do(http.MethodPost, path, body, nil) }
func (a *App) PUT(path string, body any) *Response  { return a.Do(http.MethodPut, path, body, nil) }
func (a *App) DELETE(path string) *Response         { return a.Do(http.MethodDelete, path, nil, nil) }

// Response wraps a recorded response with envelope decoding helpers.
type Response struct {
	*httptest.ResponseRecorder
	t *testing.T
}

// RequireStatus fails the test unless the response has the given status.
func (r *Response) RequireStatus(status int) *Response {
	r.t.Helper()
	if r.Code != status {
		r.t.Fatalf("apptest: status = %d, want %d; body: %s", r.Code, status, r.Body.String())
	}
	return r
}

// Data decodes the envelope's data field into out.
func (r *Response) Data(out any) *Response {
	r.t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &envelope); err != nil {
		r.t.Fatalf("apptest: decode envelope: %v; body: %s", err, r.Body.String())
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		r.t.Fatalf("apptest: decode data: %v; body: %s", err, r.Body.String())
	}
	return r
}

// Err decodes the error envelope.
func (r *Response) Err() httpx.ErrorBody {
	r.t.Helper()
	var envelope httpx.ErrorEnvelope
	if err := json.Unmarshal(r.Body.Bytes(), &envelope); err != nil {
		r.t.Fatalf("apptest: decode error envelope: %v; body: %s", err, r.Body.String())
	}
	return envelope.Error
}
