package apptest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/framework"
	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/Alfian57/gin-kit/framework/query"
	"github.com/Alfian57/gin-kit/framework/session"
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

func TestRequestOptionsAndBodies(t *testing.T) {
	app := New(t, framework.Options{})
	app.Router().PATCH("/inspect", func(c *gin.Context) {
		formTitle := c.PostForm("title")
		body, _ := io.ReadAll(c.Request.Body)
		httpx.OK(c, gin.H{
			"authorization": c.GetHeader("Authorization"),
			"custom":        c.GetHeader("X-Custom"),
			"content_type":  c.ContentType(),
			"form_title":    formTitle,
			"body":          string(body),
		})
	})

	var out struct {
		Authorization string `json:"authorization"`
		Custom        string `json:"custom"`
		ContentType   string `json:"content_type"`
		FormTitle     string `json:"form_title"`
	}
	app.PATCH("/inspect", Form{"title": []string{"hello"}},
		WithBearer("token-123"), WithHeader("X-Custom", "present")).
		RequireStatus(http.StatusOK).Data(&out)
	if out.Authorization != "Bearer token-123" || out.Custom != "present" {
		t.Fatalf("request options not applied: %+v", out)
	}
	if out.ContentType != "application/x-www-form-urlencoded" || out.FormTitle != "hello" {
		t.Fatalf("form body wrong: %+v", out)
	}

	var raw struct {
		ContentType string `json:"content_type"`
		Body        string `json:"body"`
	}
	app.PATCH("/inspect", Raw{ContentType: "text/plain", Body: []byte("verbatim")}).
		RequireStatus(http.StatusOK).Data(&raw)
	if raw.ContentType != "text/plain" || raw.Body != "verbatim" {
		t.Fatalf("raw body wrong: %+v", raw)
	}
}

func TestMultipartUpload(t *testing.T) {
	app := New(t, framework.Options{})
	app.Router().POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("document")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		opened, _ := file.Open()
		content, _ := io.ReadAll(opened)
		opened.Close()
		httpx.OK(c, gin.H{"field": c.PostForm("note"), "filename": file.Filename, "content": string(content)})
	})
	var out struct {
		Field    string `json:"field"`
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	body := NewMultipart().Field("note", "attached").File("document", "report.txt", "text/plain", []byte("file-bytes"))
	app.POST("/upload", body).RequireStatus(http.StatusOK).Data(&out)
	if out.Field != "attached" || out.Filename != "report.txt" || out.Content != "file-bytes" {
		t.Fatalf("multipart wrong: %+v", out)
	}
}

func TestMetaDecoding(t *testing.T) {
	app := New(t, framework.Options{})
	app.Router().GET("/list", func(c *gin.Context) {
		httpx.List(c, []string{"a"}, query.Result{Page: 2, PerPage: 10}.Meta(21))
	})
	var meta query.Meta
	app.GET("/list").RequireStatus(http.StatusOK).Meta(&meta)
	if meta.Page != 2 || meta.Total != 21 || meta.TotalPages != 3 {
		t.Fatalf("meta wrong: %+v", meta)
	}
}

func TestClientCarriesSessionAndCSRF(t *testing.T) {
	app := New(t, framework.Options{})
	middleware, err := session.Middleware(session.Options{Secret: []byte(strings.Repeat("s", session.MinimumSecretLength))})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().Use(middleware, session.CSRF(session.CSRFOptions{}))
	app.Router().GET("/form", func(c *gin.Context) {
		c.String(http.StatusOK, "<form>"+string(session.TemplateField(c))+"</form>")
	})
	app.Router().POST("/submit", func(c *gin.Context) { c.String(http.StatusOK, "accepted") })

	client := app.Client()
	token := client.CSRFToken("/form")
	if token == "" {
		t.Fatal("empty CSRF token")
	}
	client.POST("/submit", Form{"_csrf": []string{token}}).RequireStatus(http.StatusOK)

	// A fresh client without the session cookie must be rejected.
	app.Client().POST("/submit", Form{"_csrf": []string{token}}).RequireStatus(http.StatusForbidden)
}

func TestCleanupClosesApplication(t *testing.T) {
	var closed bool
	t.Run("scoped", func(t *testing.T) {
		app := New(t, framework.Options{})
		app.Application().OnShutdown(func(context.Context) error {
			closed = true
			return nil
		})
	})
	if !closed {
		t.Fatal("cleanup did not close the application")
	}
}
