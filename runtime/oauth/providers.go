package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const (
	googleIssuer = "https://accounts.google.com"
	githubAPIURL = "https://api.github.com"
)

// Config contains the credentials and exact callback URL registered with an
// OAuth provider.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Validate rejects incomplete provider credentials and callback URLs.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecret) == "" {
		return errors.New("oauth: client ID and client secret are required")
	}
	if strings.TrimSpace(c.RedirectURL) == "" {
		return errors.New("oauth: redirect URL is required")
	}
	redirectURL, err := url.ParseRequestURI(strings.TrimSpace(c.RedirectURL))
	if err != nil || !redirectURL.IsAbs() || redirectURL.Host == "" {
		return errors.New("oauth: redirect URL must be an absolute HTTP(S) URL")
	}
	if redirectURL.Scheme != "http" && redirectURL.Scheme != "https" {
		return errors.New("oauth: redirect URL must be an absolute HTTP(S) URL")
	}
	return nil
}

// NewGoogle creates an OpenID Connect provider for Google sign-in.
func NewGoogle(ctx context.Context, config Config) (Provider, error) {
	return NewOIDC(ctx, "google", googleIssuer, config)
}

// NewOIDC creates a provider using OpenID Connect discovery. name must be a
// stable, lower-case application identifier.
func NewOIDC(ctx context.Context, name, issuer string, config Config) (Provider, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	name = normalizeProvider(name)
	if name == "" {
		return nil, errors.New("oauth: OIDC provider name is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	discovered, err := oidc.NewProvider(ctx, strings.TrimSpace(issuer))
	if err != nil {
		return nil, fmt.Errorf("oauth: discover OIDC provider: %w", err)
	}
	return &oidcProvider{
		name: name,
		config: oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			RedirectURL:  config.RedirectURL,
			Endpoint:     discovered.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: discovered.Verifier(&oidc.Config{ClientID: config.ClientID}),
	}, nil
}

type oidcProvider struct {
	name     string
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func (p *oidcProvider) Name() string { return p.name }

func (p *oidcProvider) AuthorizationURL(state, verifier, nonce string) string {
	return p.config.AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(verifier),
		oidc.Nonce(nonce),
	)
}

func (p *oidcProvider) Identity(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	ctx, cancel := oauthHTTPContext(ctx)
	defer cancel()
	token, err := p.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("oauth: exchange OIDC code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return Identity{}, errors.New("oauth: OIDC response did not contain an ID token")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, fmt.Errorf("oauth: verify OIDC ID token: %w", err)
	}
	if idToken.Nonce != nonce {
		return Identity{}, errors.New("oauth: OIDC nonce mismatch")
	}
	var claims struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("oauth: decode OIDC ID token claims: %w", err)
	}
	if !claims.EmailVerified {
		return Identity{}, ErrEmailUnverified
	}
	return Identity{Provider: p.name, Subject: claims.Subject, Email: claims.Email, Name: claims.Name, AvatarURL: claims.Picture}, nil
}

// NewGitHub creates GitHub OAuth sign-in with the minimum profile and email
// scopes required to establish a verified local account identity.
func NewGitHub(config Config) (Provider, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return newGitHub(config, githubAPIURL), nil
}

func newGitHub(config Config, apiURL string) Provider {
	return &githubProvider{config: oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     github.Endpoint,
		Scopes:       []string{"read:user", "user:email"},
	}, apiURL: strings.TrimRight(apiURL, "/")}
}

type githubProvider struct {
	config oauth2.Config
	apiURL string
}

func (*githubProvider) Name() string { return "github" }

func (p *githubProvider) AuthorizationURL(state, verifier, _ string) string {
	return p.config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
}

func (p *githubProvider) Identity(ctx context.Context, code, verifier, _ string) (Identity, error) {
	ctx, cancel := oauthHTTPContext(ctx)
	defer cancel()
	token, err := p.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("oauth: exchange GitHub code: %w", err)
	}
	client := p.config.Client(ctx, token)
	var profile struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := getJSON(client, p.apiURL+"/user", &profile); err != nil {
		return Identity{}, fmt.Errorf("oauth: read GitHub profile: %w", err)
	}
	if profile.ID == 0 {
		return Identity{}, errors.New("oauth: GitHub profile did not contain an ID")
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := getJSON(client, p.apiURL+"/user/emails", &emails); err != nil {
		return Identity{}, fmt.Errorf("oauth: read GitHub emails: %w", err)
	}
	for _, email := range emails {
		if email.Primary && email.Verified && strings.TrimSpace(email.Email) != "" {
			return Identity{Provider: "github", Subject: fmt.Sprint(profile.ID), Email: email.Email, Name: firstNonEmpty(profile.Name, profile.Login), AvatarURL: profile.AvatarURL}, nil
		}
	}
	return Identity{}, ErrEmailUnverified
}

func oauthHTTPContext(ctx context.Context) (context.Context, context.CancelFunc) {
	client := &http.Client{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	return context.WithValue(ctx, oauth2.HTTPClient, client), cancel
}

func getJSON(client *http.Client, url string, target any) error {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return err
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
