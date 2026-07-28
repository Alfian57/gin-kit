// Package session provides encrypted cookie sessions, one-shot flash
// messages, and CSRF protection for UI-mode applications.
package session

import (
	"crypto/sha256"
	"errors"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// MinimumSecretLength is the smallest accepted session secret, in bytes.
const MinimumSecretLength = 32

// Options defines an implementation type used by this package.
type Options struct {
	// Secret signs and encrypts the session cookie and must contain at least
	// 32 bytes. One secret drives both keys.
	Secret []byte
	// CookieName defaults to gin_kit_session.
	CookieName string
	// MaxAge defaults to seven days, in seconds.
	MaxAge int
	// Secure marks the cookie HTTPS-only; enable it outside development.
	Secure bool
	// Domain store data used by this type.
	Domain string
	// Path defaults to /.
	Path string
	// SameSite defaults to Lax.
	SameSite http.SameSite
}

// Middleware installs the encrypted cookie session store.
func Middleware(options Options) (gin.HandlerFunc, error) {
	if len(options.Secret) < MinimumSecretLength {
		return nil, errors.New("session: secret must contain at least 32 bytes")
	}
	if options.CookieName == "" {
		options.CookieName = "gin_kit_session"
	}
	if options.MaxAge == 0 {
		options.MaxAge = 7 * 24 * 60 * 60
	}
	if options.Path == "" {
		options.Path = "/"
	}
	if options.SameSite == 0 {
		options.SameSite = http.SameSiteLaxMode
	}
	// One env secret yields distinct signing and encryption keys.
	encryptionKey := sha256.Sum256(append(append([]byte{}, options.Secret...), []byte("gin-kit.session.encryption")...))
	store := cookie.NewStore(options.Secret, encryptionKey[:])
	store.Options(sessions.Options{
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: true,
		SameSite: options.SameSite,
	})
	return sessions.Sessions(options.CookieName, store), nil
}

// Get reads a session value.
func Get(c *gin.Context, key string) any {
	return sessions.Default(c).Get(key)
}

// Set writes and saves a session value.
func Set(c *gin.Context, key string, value any) error {
	current := sessions.Default(c)
	current.Set(key, value)
	return current.Save()
}

// Forget removes a session value.
func Forget(c *gin.Context, key string) error {
	current := sessions.Default(c)
	current.Delete(key)
	return current.Save()
}
