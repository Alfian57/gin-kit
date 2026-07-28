package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/runtime/session"
	"github.com/gin-gonic/gin"
)

type fakeProvider struct {
	name     string
	identity Identity
	code     string
	verifier string
	nonce    string
}

func (p *fakeProvider) Name() string { return p.name }

func (p *fakeProvider) AuthorizationURL(state, verifier, nonce string) string {
	return "https://provider.example/authorize?state=" + url.QueryEscape(state) + "&challenge=" + url.QueryEscape(verifier) + "&nonce=" + url.QueryEscape(nonce)
}

func (p *fakeProvider) Identity(_ context.Context, code, verifier, nonce string) (Identity, error) {
	p.code, p.verifier, p.nonce = code, verifier, nonce
	return p.identity, nil
}

func TestManagerCompletesAndConsumesBrowserFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := &fakeProvider{name: "google", identity: Identity{Subject: "subject-1", Email: "USER@EXAMPLE.COM"}}
	manager, err := NewManager(provider)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	middleware, err := session.Middleware(session.Options{Secret: []byte(strings.Repeat("s", session.MinimumSecretLength))})
	if err != nil {
		t.Fatal(err)
	}
	router.Use(middleware)
	router.GET("/start", func(c *gin.Context) {
		location, beginErr := manager.Begin(c, "google")
		if beginErr != nil {
			c.Error(beginErr)
			return
		}
		c.Redirect(http.StatusFound, location)
	})
	router.GET("/callback", func(c *gin.Context) {
		identity, completeErr := manager.Complete(c, "google", c.Query("code"), c.Query("state"))
		if completeErr != nil {
			c.String(http.StatusBadRequest, completeErr.Error())
			return
		}
		c.JSON(http.StatusOK, identity)
	})

	start := httptest.NewRecorder()
	router.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/start", nil))
	if start.Code != http.StatusFound {
		t.Fatalf("start status = %d, want %d", start.Code, http.StatusFound)
	}
	location, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := location.Query().Get("state")
	if state == "" || location.Query().Get("challenge") == "" || location.Query().Get("nonce") == "" {
		t.Fatalf("authorization URL missing flow values: %q", location)
	}
	cookies := start.Result().Cookies()
	callback := httptest.NewRequest(http.MethodGet, "/callback?code=provider-code&state="+url.QueryEscape(state), nil)
	for _, cookie := range cookies {
		callback.AddCookie(cookie)
	}
	completed := httptest.NewRecorder()
	router.ServeHTTP(completed, callback)
	if completed.Code != http.StatusOK {
		t.Fatalf("callback status = %d, body = %s", completed.Code, completed.Body.String())
	}
	if provider.code != "provider-code" || provider.verifier == "" || provider.nonce == "" {
		t.Fatalf("provider flow = code:%q verifier:%q nonce:%q", provider.code, provider.verifier, provider.nonce)
	}
	if !strings.Contains(completed.Body.String(), `"email":"user@example.com"`) {
		t.Fatalf("identity was not normalized: %s", completed.Body.String())
	}

	replayed := httptest.NewRequest(http.MethodGet, "/callback?code=provider-code&state="+url.QueryEscape(state), nil)
	for _, cookie := range completed.Result().Cookies() {
		replayed.AddCookie(cookie)
	}
	replayResponse := httptest.NewRecorder()
	router.ServeHTTP(replayResponse, replayed)
	if replayResponse.Code != http.StatusBadRequest || !strings.Contains(replayResponse.Body.String(), ErrStateInvalid.Error()) {
		t.Fatalf("replayed callback was accepted: %d %s", replayResponse.Code, replayResponse.Body.String())
	}
}

func TestManagerRejectsUnavailableProviderAndInvalidState(t *testing.T) {
	manager, err := NewManager(&fakeProvider{name: "github"})
	if err != nil {
		t.Fatal(err)
	}
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	context.Request = request
	if _, err := manager.Begin(context, "google"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("unavailable provider error = %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	if err := (Config{}).Validate(); err == nil {
		t.Fatal("empty config was accepted")
	}
	if err := (Config{ClientID: "id", ClientSecret: "secret", RedirectURL: "https://app.example/callback"}).Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if err := (Config{ClientID: "id", ClientSecret: "secret", RedirectURL: "/callback"}).Validate(); err == nil {
		t.Fatal("relative provider callback URL was accepted")
	}
}
