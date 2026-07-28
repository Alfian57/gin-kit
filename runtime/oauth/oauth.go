// Package oauth provides explicit OAuth 2.0 and OpenID Connect sign-in flows.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Alfian57/gin-kit/runtime/session"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const (
	// DefaultFlowTTL limits how long a browser may complete an OAuth flow.
	DefaultFlowTTL = 10 * time.Minute
	// maxPendingFlows define package-level implementation state.
	maxPendingFlows = 5
	// flowsSessionKey define package-level implementation state.
	flowsSessionKey = "gin-kit.oauth.flows"
)

var (
	// ErrProviderUnavailable means the requested provider was not registered.
	ErrProviderUnavailable = errors.New("oauth provider is unavailable")
	// ErrStateInvalid means the callback cannot be tied to a pending browser flow.
	ErrStateInvalid = errors.New("oauth state is invalid or expired")
	// ErrAuthorizationDenied means the provider denied the authorization request.
	ErrAuthorizationDenied = errors.New("oauth authorization was denied")
	// ErrEmailUnverified means the provider did not return a verified email address.
	ErrEmailUnverified = errors.New("oauth identity does not include a verified email")
)

// Identity is the verified profile returned by a social provider. Provider
// tokens intentionally do not leave this package or enter application storage.
type Identity struct {
	// Provider store data used by this type.
	Provider string `json:"provider"`
	// Subject store data used by this type.
	Subject string `json:"subject"`
	// Email store data used by this type.
	Email string `json:"email"`
	// Name store data used by this type.
	Name string `json:"name"`
	// AvatarURL store data used by this type.
	AvatarURL string `json:"avatar_url"`
}

// Provider is an explicit OAuth provider implementation. Applications may add
// a provider without global registration or reflection.
type Provider interface {
	// Name define an operation required by this interface.
	Name() string
	// AuthorizationURL define an operation required by this interface.
	AuthorizationURL(state, verifier, nonce string) string
	// Identity define an operation required by this interface.
	Identity(ctx context.Context, code, verifier, nonce string) (Identity, error)
}

// Manager owns a fixed provider registry and the browser flow state stored in
// the application's encrypted session cookie.
type Manager struct {
	// providers store data used by this type.
	providers map[string]Provider
	// flowTTL store data used by this type.
	flowTTL time.Duration
	// now store data used by this type.
	now func() time.Time
}

// NewManager creates an explicit OAuth provider registry.
func NewManager(providers ...Provider) (*Manager, error) {
	m := &Manager{
		providers: make(map[string]Provider, len(providers)),
		flowTTL:   DefaultFlowTTL,
		now:       time.Now,
	}
	for _, provider := range providers {
		if provider == nil {
			return nil, errors.New("oauth: provider is nil")
		}
		name := normalizeProvider(provider.Name())
		if name == "" {
			return nil, errors.New("oauth: provider name is required")
		}
		if _, exists := m.providers[name]; exists {
			return nil, fmt.Errorf("oauth: duplicate provider %q", name)
		}
		m.providers[name] = provider
	}
	return m, nil
}

// Begin creates and stores a short-lived browser flow, then returns the
// provider's authorization URL. Session middleware must be installed first.
func (m *Manager) Begin(c *gin.Context, providerName string) (string, error) {
	provider, name := m.provider(providerName)
	if provider == nil {
		return "", ErrProviderUnavailable
	}
	state, err := randomURLValue(32)
	if err != nil {
		return "", fmt.Errorf("oauth: generate state: %w", err)
	}
	nonce, err := randomURLValue(32)
	if err != nil {
		return "", fmt.Errorf("oauth: generate nonce: %w", err)
	}
	verifier := oauth2.GenerateVerifier()
	flows, err := readFlows(c)
	if err != nil {
		return "", err
	}
	if len(flows) >= maxPendingFlows {
		return "", errors.New("oauth: too many pending authorization flows")
	}
	flows[state] = flow{
		Provider:  name,
		State:     state,
		Nonce:     nonce,
		Verifier:  verifier,
		ExpiresAt: m.now().UTC().Add(m.flowTTL),
	}
	if err := writeFlows(c, flows); err != nil {
		return "", err
	}
	return provider.AuthorizationURL(state, verifier, nonce), nil
}

// Complete validates and consumes a browser flow before exchanging the code
// with its provider. A flow is removed before the network request so callbacks
// cannot be replayed after either success or failure.
func (m *Manager) Complete(c *gin.Context, providerName, code, state string) (Identity, error) {
	provider, name := m.provider(providerName)
	if provider == nil {
		return Identity{}, ErrProviderUnavailable
	}
	flows, err := readFlows(c)
	if err != nil {
		return Identity{}, err
	}
	pending, exists := flows[state]
	delete(flows, state)
	if writeErr := writeFlows(c, flows); writeErr != nil {
		return Identity{}, writeErr
	}
	if !exists || pending.Provider != name || pending.ExpiresAt.Before(m.now().UTC()) ||
		subtle.ConstantTimeCompare([]byte(pending.State), []byte(state)) != 1 {
		return Identity{}, ErrStateInvalid
	}
	if strings.TrimSpace(code) == "" {
		return Identity{}, ErrStateInvalid
	}
	identity, err := provider.Identity(c.Request.Context(), code, pending.Verifier, pending.Nonce)
	if err != nil {
		return Identity{}, err
	}
	identity.Provider = name
	identity.Subject = strings.TrimSpace(identity.Subject)
	identity.Email = strings.ToLower(strings.TrimSpace(identity.Email))
	if identity.Subject == "" || identity.Email == "" {
		return Identity{}, ErrEmailUnverified
	}
	return identity, nil
}

// Denied reports whether an OAuth provider returned an authorization error.
func Denied(c *gin.Context) bool { return strings.TrimSpace(c.Query("error")) != "" }

// flow defines an implementation type used by this package.
type flow struct {
	// Provider store data used by this type.
	Provider string `json:"provider"`
	// State store data used by this type.
	State string `json:"state"`
	// Nonce store data used by this type.
	Nonce string `json:"nonce"`
	// Verifier store data used by this type.
	Verifier string `json:"verifier"`
	// ExpiresAt store data used by this type.
	ExpiresAt time.Time `json:"expires_at"`
}

// provider performs this package operation.
func (m *Manager) provider(name string) (Provider, string) {
	name = normalizeProvider(name)
	return m.providers[name], name
}

// normalizeProvider performs this package operation.
func normalizeProvider(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// readFlows performs this package operation.
func readFlows(c *gin.Context) (map[string]flow, error) {
	encoded, _ := session.Get(c, flowsSessionKey).(string)
	if encoded == "" {
		return map[string]flow{}, nil
	}
	flows := map[string]flow{}
	if err := json.Unmarshal([]byte(encoded), &flows); err != nil {
		return nil, errors.New("oauth: invalid session flow data")
	}
	return flows, nil
}

// writeFlows performs this package operation.
func writeFlows(c *gin.Context, flows map[string]flow) error {
	encoded, err := json.Marshal(flows)
	if err != nil {
		return fmt.Errorf("oauth: encode session flow data: %w", err)
	}
	if err := session.Set(c, flowsSessionKey, string(encoded)); err != nil {
		return fmt.Errorf("oauth: save authorization flow: %w", err)
	}
	return nil
}

// randomURLValue performs this package operation.
func randomURLValue(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
