// Package auth provides signed access and rotating refresh-token primitives.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const MinimumSecretLength = 32

type Claims struct {
	UserID string `json:"sub"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret    []byte
	Issuer    string
	AccessTTL time.Duration
}

func New(secret []byte, issuer string, accessTTL time.Duration) (*Manager, error) {
	if err := ValidateSecret(secret); err != nil {
		return nil, err
	}
	if issuer == "" {
		issuer = "ginkit"
	}
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	copied := append([]byte(nil), secret...)
	return &Manager{secret: copied, Issuer: issuer, AccessTTL: accessTTL}, nil
}

func ValidateSecret(secret []byte) error {
	if len(secret) < MinimumSecretLength {
		return errors.New("authentication secret must contain at least 32 bytes")
	}
	return nil
}

func (m *Manager) Issue(userID string, ttl ...time.Duration) (string, error) {
	if m == nil {
		return "", errors.New("authentication manager is nil")
	}
	lifetime := m.AccessTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		lifetime = ttl[0]
	}
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID, Issuer: m.Issuer, IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(lifetime)),
		},
	}).SignedString(m.secret)
}

func (m *Manager) Parse(tokenString string) (*Claims, error) {
	if m == nil {
		return nil, errors.New("authentication manager is nil")
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.Issuer))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func Issue(userID string, secret []byte, ttl time.Duration) (string, error) {
	manager, err := New(secret, "ginkit", ttl)
	if err != nil {
		return "", err
	}
	return manager.Issue(userID)
}

func Parse(token string, secret []byte) (*Claims, error) {
	manager, err := New(secret, "ginkit", time.Minute)
	if err != nil {
		return nil, err
	}
	return manager.Parse(token)
}

func NewRefreshToken() (plain, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(raw)
	return plain, HashRefreshToken(plain), nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
