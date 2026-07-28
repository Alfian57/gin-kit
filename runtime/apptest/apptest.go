// Package apptest provides small helpers for exercising a gin-kit application
// in tests and decoding its envelope responses. Assertions beyond status
// checks stay in plain Go.
package apptest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/runtime"
	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/gin-gonic/gin"
)

// App wraps a runtime application for in-process HTTP testing.
type App struct {
	// t store data used by this type.
	t *testing.T
	// app store data used by this type.
	app *runtime.Application
}

// New builds an application from options, fails the test on error, and
// closes the application (shutdown hooks) when the test finishes.
func New(t *testing.T, options runtime.Options) *App {
	t.Helper()
	gin.SetMode(gin.TestMode)
	application, err := runtime.New(options)
	if err != nil {
		t.Fatalf("apptest: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(context.Background()); err != nil {
			t.Errorf("apptest: close application: %v", err)
		}
	})
	return &App{t: t, app: application}
}

// Router exposes the underlying engine for route registration.
func (a *App) Router() *gin.Engine { return a.app.Router() }

// Application exposes the wrapped runtime application.
func (a *App) Application() *runtime.Application { return a.app }

// RequestOption customizes one outgoing request.
type RequestOption func(*http.Request)

// WithHeader sets a single header.
func WithHeader(key, value string) RequestOption {
	return func(r *http.Request) { r.Header.Set(key, value) }
}

// WithHeaders adds every value of h.
func WithHeaders(h http.Header) RequestOption {
	return func(r *http.Request) {
		for key, values := range h {
			for _, value := range values {
				r.Header.Add(key, value)
			}
		}
	}
}

// WithBearer sets the Authorization header with a Bearer token.
func WithBearer(token string) RequestOption {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

// WithCookie attaches a cookie to the request.
func WithCookie(cookie *http.Cookie) RequestOption {
	return func(r *http.Request) { r.AddCookie(cookie) }
}

// Form marks a body as application/x-www-form-urlencoded.
type Form = url.Values

// Raw sends the body verbatim with the given content type.
type Raw struct {
	// ContentType store data used by this type.
	ContentType string
	// Body store data used by this type.
	Body []byte
}

// MultipartBody builds a multipart/form-data request body.
type MultipartBody struct {
	// buffer store data used by this type.
	buffer bytes.Buffer
	// writer store data used by this type.
	writer *multipart.Writer
	// err store data used by this type.
	err error
}

// NewMultipart performs this package operation.
func NewMultipart() *MultipartBody {
	body := &MultipartBody{}
	body.writer = multipart.NewWriter(&body.buffer)
	return body
}

// Field performs this package operation.
func (m *MultipartBody) Field(name, value string) *MultipartBody {
	if m.err == nil {
		m.err = m.writer.WriteField(name, value)
	}
	return m
}

// File performs this package operation.
func (m *MultipartBody) File(field, filename, contentType string, content []byte) *MultipartBody {
	if m.err != nil {
		return m
	}
	header := make(map[string][]string)
	header["Content-Disposition"] = []string{`form-data; name="` + field + `"; filename="` + filename + `"`}
	if contentType != "" {
		header["Content-Type"] = []string{contentType}
	}
	part, err := m.writer.CreatePart(header)
	if err != nil {
		m.err = err
		return m
	}
	_, m.err = part.Write(content)
	return m
}

// Do performs an in-process request. Bodies: nil sends none, Form is
// form-urlencoded, *MultipartBody is multipart, Raw is sent verbatim, and
// anything else is JSON-encoded.
func (a *App) Do(method, path string, body any, opts ...RequestOption) *Response {
	a.t.Helper()
	request := buildRequest(a.t, method, path, body, opts)
	recorder := httptest.NewRecorder()
	a.app.Router().ServeHTTP(recorder, request)
	return &Response{ResponseRecorder: recorder, t: a.t}
}

// buildRequest performs this package operation.
func buildRequest(t *testing.T, method, path string, body any, opts []RequestOption) *http.Request {
	t.Helper()
	var reader io.Reader
	contentType := ""
	switch typed := body.(type) {
	case nil:
	case Form:
		reader = strings.NewReader(typed.Encode())
		contentType = "application/x-www-form-urlencoded"
	case *MultipartBody:
		if typed.err == nil {
			typed.err = typed.writer.Close()
		}
		if typed.err != nil {
			t.Fatalf("apptest: build multipart body: %v", typed.err)
		}
		reader = &typed.buffer
		contentType = typed.writer.FormDataContentType()
	case Raw:
		reader = bytes.NewReader(typed.Body)
		contentType = typed.ContentType
	default:
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("apptest: encode request body: %v", err)
		}
		reader = bytes.NewReader(encoded)
		contentType = "application/json"
	}
	request := httptest.NewRequest(method, path, reader)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	for _, opt := range opts {
		opt(request)
	}
	return request
}

// GET performs this package operation.
func (a *App) GET(path string, opts ...RequestOption) *Response {
	return a.Do(http.MethodGet, path, nil, opts...)
}

// POST performs this package operation.
func (a *App) POST(path string, body any, opts ...RequestOption) *Response {
	return a.Do(http.MethodPost, path, body, opts...)
}

// PUT performs this package operation.
func (a *App) PUT(path string, body any, opts ...RequestOption) *Response {
	return a.Do(http.MethodPut, path, body, opts...)
}

// PATCH performs this package operation.
func (a *App) PATCH(path string, body any, opts ...RequestOption) *Response {
	return a.Do(http.MethodPatch, path, body, opts...)
}

// DELETE performs this package operation.
func (a *App) DELETE(path string, opts ...RequestOption) *Response {
	return a.Do(http.MethodDelete, path, nil, opts...)
}

// Client returns a stateful client that carries cookies across requests, for
// session and CSRF flows.
func (a *App) Client() *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		a.t.Fatalf("apptest: cookie jar: %v", err)
	}
	return &Client{app: a, jar: jar}
}

// clientBase anchors the cookie jar; in-process requests never dial it.
var clientBase = &url.URL{Scheme: "http", Host: "apptest.local"}

// Client performs requests while persisting cookies between them.
type Client struct {
	// app store data used by this type.
	app *App
	// jar store data used by this type.
	jar *cookiejar.Jar
}

// Do performs this package operation.
func (c *Client) Do(method, path string, body any, opts ...RequestOption) *Response {
	c.app.t.Helper()
	request := buildRequest(c.app.t, method, path, body, opts)
	for _, cookie := range c.jar.Cookies(clientBase) {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	c.app.app.Router().ServeHTTP(recorder, request)
	c.jar.SetCookies(clientBase, recorder.Result().Cookies())
	return &Response{ResponseRecorder: recorder, t: c.app.t}
}

// GET performs this package operation.
func (c *Client) GET(path string, opts ...RequestOption) *Response {
	return c.Do(http.MethodGet, path, nil, opts...)
}

// POST performs this package operation.
func (c *Client) POST(path string, body any, opts ...RequestOption) *Response {
	return c.Do(http.MethodPost, path, body, opts...)
}

// PUT performs this package operation.
func (c *Client) PUT(path string, body any, opts ...RequestOption) *Response {
	return c.Do(http.MethodPut, path, body, opts...)
}

// PATCH performs this package operation.
func (c *Client) PATCH(path string, body any, opts ...RequestOption) *Response {
	return c.Do(http.MethodPatch, path, body, opts...)
}

// DELETE performs this package operation.
func (c *Client) DELETE(path string, opts ...RequestOption) *Response {
	return c.Do(http.MethodDelete, path, nil, opts...)
}

// csrfFieldPattern define package-level implementation state.
var csrfFieldPattern = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

// CSRFToken GETs path with this client (establishing the session cookie) and
// extracts the CSRF token from the rendered hidden form field.
func (c *Client) CSRFToken(path string) string {
	c.app.t.Helper()
	response := c.GET(path)
	match := csrfFieldPattern.FindStringSubmatch(response.Body.String())
	if match == nil {
		c.app.t.Fatalf("apptest: no CSRF field found in %s response", path)
	}
	return match[1]
}

// Response wraps a recorded response with envelope decoding helpers.
type Response struct {
	*httptest.ResponseRecorder
	// t store data used by this type.
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

// Meta decodes the envelope's meta field into out.
func (r *Response) Meta(out any) *Response {
	r.t.Helper()
	var envelope struct {
		Meta json.RawMessage `json:"meta"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &envelope); err != nil {
		r.t.Fatalf("apptest: decode envelope: %v; body: %s", err, r.Body.String())
	}
	if err := json.Unmarshal(envelope.Meta, out); err != nil {
		r.t.Fatalf("apptest: decode meta: %v; body: %s", err, r.Body.String())
	}
	return r
}

// JSON decodes the whole response body into out.
func (r *Response) JSON(out any) *Response {
	r.t.Helper()
	if err := json.Unmarshal(r.Body.Bytes(), out); err != nil {
		r.t.Fatalf("apptest: decode body: %v; body: %s", err, r.Body.String())
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
