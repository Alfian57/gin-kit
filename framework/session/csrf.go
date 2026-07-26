package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const csrfSessionKey = "gin-kit.csrf"

type CSRFOptions struct {
	// FieldName is the form field carrying the token, defaulting to _csrf.
	FieldName string
	// HeaderName is checked before the form field, defaulting to X-CSRF-Token.
	HeaderName string
	// Skipper exempts requests; by default paths under /api/ are skipped
	// because token-authenticated JSON APIs are not CSRF-prone.
	Skipper func(*gin.Context) bool
}

// CSRF validates a per-session token on every state-changing request. Safe
// methods (GET/HEAD/OPTIONS/TRACE) pass through and lazily mint the token.
// Install it after the session middleware.
func CSRF(options CSRFOptions) gin.HandlerFunc {
	if options.FieldName == "" {
		options.FieldName = "_csrf"
	}
	if options.HeaderName == "" {
		options.HeaderName = "X-CSRF-Token"
	}
	if options.Skipper == nil {
		options.Skipper = func(c *gin.Context) bool {
			return strings.HasPrefix(c.Request.URL.Path, "/api/")
		}
	}
	return func(c *gin.Context) {
		if options.Skipper(c) {
			c.Next()
			return
		}
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			ensureToken(c)
			c.Next()
			return
		}
		expected, _ := sessions.Default(c).Get(csrfSessionKey).(string)
		presented := c.GetHeader(options.HeaderName)
		if presented == "" {
			presented = c.PostForm(options.FieldName)
		}
		if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(presented)) != 1 {
			httpx.Fail(c, httpx.NewError(http.StatusForbidden, "csrf_token_mismatch", "CSRF token missing or invalid."))
			return
		}
		c.Next()
	}
}

// Token returns the session's CSRF token, minting one when absent.
func Token(c *gin.Context) string {
	return ensureToken(c)
}

// TemplateField renders a hidden input carrying the CSRF token for HTML forms.
func TemplateField(c *gin.Context) template.HTML {
	return template.HTML(fmt.Sprintf(`<input type="hidden" name="_csrf" value="%s">`, Token(c)))
}

func ensureToken(c *gin.Context) string {
	current := sessions.Default(c)
	if token, ok := current.Get(csrfSessionKey).(string); ok && token != "" {
		return token
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	current.Set(csrfSessionKey, token)
	_ = current.Save()
	return token
}
