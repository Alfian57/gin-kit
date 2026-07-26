package session

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

var testSecret = []byte(strings.Repeat("s", MinimumSecretLength))

func sessionRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	middleware, err := Middleware(Options{Secret: testSecret})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(middleware)
	return router
}

// carryCookies replays Set-Cookie headers from a response onto a request.
func carryCookies(t *testing.T, from *httptest.ResponseRecorder, to *http.Request) {
	t.Helper()
	for _, cookie := range from.Result().Cookies() {
		to.AddCookie(cookie)
	}
}

func TestMiddlewareRejectsShortSecret(t *testing.T) {
	if _, err := Middleware(Options{Secret: []byte("short")}); err == nil {
		t.Fatal("short secret accepted")
	}
}

func TestSessionRoundTripAcrossRequests(t *testing.T) {
	router := sessionRouter(t)
	router.GET("/set", func(c *gin.Context) {
		if err := Set(c, "user", "alfian"); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Status(http.StatusOK)
	})
	router.GET("/get", func(c *gin.Context) {
		value, _ := Get(c, "user").(string)
		c.String(http.StatusOK, value)
	})

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/set", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("set failed: %d %s", first.Code, first.Body)
	}
	cookie := first.Result().Cookies()
	if len(cookie) == 0 {
		t.Fatal("no session cookie issued")
	}
	if !cookie[0].HttpOnly || cookie[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags wrong: %+v", cookie[0])
	}

	second := httptest.NewRequest(http.MethodGet, "/get", nil)
	carryCookies(t, first, second)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, second)
	if recorder.Body.String() != "alfian" {
		t.Fatalf("session value lost: %q", recorder.Body.String())
	}
}

func TestFlashesAreOneShot(t *testing.T) {
	router := sessionRouter(t)
	router.POST("/create", func(c *gin.Context) {
		_ = PutFlash(c, "success", "Task created.")
		c.Status(http.StatusOK)
	})
	router.GET("/list", func(c *gin.Context) {
		flashes := Flashes(c)
		if len(flashes) == 0 {
			c.String(http.StatusOK, "none")
			return
		}
		c.String(http.StatusOK, flashes[0].Kind+":"+flashes[0].Message)
	})

	created := httptest.NewRecorder()
	router.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/create", nil))

	first := httptest.NewRequest(http.MethodGet, "/list", nil)
	carryCookies(t, created, first)
	firstRecorder := httptest.NewRecorder()
	router.ServeHTTP(firstRecorder, first)
	if firstRecorder.Body.String() != "success:Task created." {
		t.Fatalf("flash not delivered: %q", firstRecorder.Body.String())
	}

	second := httptest.NewRequest(http.MethodGet, "/list", nil)
	carryCookies(t, firstRecorder, second)
	secondRecorder := httptest.NewRecorder()
	router.ServeHTTP(secondRecorder, second)
	if secondRecorder.Body.String() != "none" {
		t.Fatalf("flash survived a second read: %q", secondRecorder.Body.String())
	}
}

func TestCSRFProtectsUnsafeMethods(t *testing.T) {
	router := sessionRouter(t)
	router.Use(CSRF(CSRFOptions{}))
	router.GET("/form", func(c *gin.Context) { c.String(http.StatusOK, Token(c)) })
	router.POST("/submit", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.POST("/api/v1/things", func(c *gin.Context) { c.Status(http.StatusOK) })

	form := httptest.NewRecorder()
	router.ServeHTTP(form, httptest.NewRequest(http.MethodGet, "/form", nil))
	token := form.Body.String()
	if token == "" {
		t.Fatal("GET did not mint a token")
	}

	deny := func(t *testing.T, presented string) {
		t.Helper()
		body := url.Values{}
		if presented != "" {
			body.Set("_csrf", presented)
		}
		request := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(body.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		carryCookies(t, form, request)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "csrf_token_mismatch") {
			t.Fatalf("unsafe request not rejected: %d %s", recorder.Code, recorder.Body)
		}
	}
	deny(t, "")
	deny(t, "wrong-token")

	viaForm := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(url.Values{"_csrf": []string{token}}.Encode()))
	viaForm.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	carryCookies(t, form, viaForm)
	formRecorder := httptest.NewRecorder()
	router.ServeHTTP(formRecorder, viaForm)
	if formRecorder.Code != http.StatusOK {
		t.Fatalf("valid form token rejected: %d %s", formRecorder.Code, formRecorder.Body)
	}

	viaHeader := httptest.NewRequest(http.MethodPost, "/submit", nil)
	viaHeader.Header.Set("X-CSRF-Token", token)
	carryCookies(t, form, viaHeader)
	headerRecorder := httptest.NewRecorder()
	router.ServeHTTP(headerRecorder, viaHeader)
	if headerRecorder.Code != http.StatusOK {
		t.Fatalf("valid header token rejected: %d", headerRecorder.Code)
	}

	api := httptest.NewRecorder()
	router.ServeHTTP(api, httptest.NewRequest(http.MethodPost, "/api/v1/things", nil))
	if api.Code != http.StatusOK {
		t.Fatalf("/api/ path was not skipped: %d", api.Code)
	}
}

func TestTemplateFieldRendersHiddenInput(t *testing.T) {
	router := sessionRouter(t)
	router.GET("/form", func(c *gin.Context) {
		c.String(http.StatusOK, string(TemplateField(c)))
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/form", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `type="hidden"`) || !strings.Contains(body, `name="_csrf"`) {
		t.Fatalf("template field malformed: %s", body)
	}
}
