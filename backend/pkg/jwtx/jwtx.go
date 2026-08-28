// Package jwtx issues and validates HS256 JWTs (stdlib only).
// RS256 is planned for production hardening; the interface stays the same.
package jwtx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims carried by every Unital access token.
type Claims struct {
	Sub      string `json:"sub"`      // user id
	Role     string `json:"role"`     // platform role
	Verified bool   `json:"verified"` // email verified at issue time
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

var (
	ErrExpired      = errors.New("token expired")
	ErrMalformed    = errors.New("malformed token")
	ErrBadSignature = errors.New("bad token signature")
)

// Signer issues tokens with a fixed TTL.
type Signer struct {
	key []byte
	ttl time.Duration
}

func NewSigner(secret string, ttl time.Duration) *Signer {
	return &Signer{key: []byte(secret), ttl: ttl}
}

func (s *Signer) TTL() time.Duration { return s.ttl }

func (s *Signer) Issue(sub, role string, verified bool) (string, error) {
	now := time.Now()
	c := Claims{Sub: sub, Role: role, Verified: verified, Iat: now.Unix(), Exp: now.Add(s.ttl).Unix()}
	header := base64URL([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	body := header + "." + base64URL(payload)
	return body + "." + base64URL(s.sign(body)), nil
}

// Parse validates signature and expiry, returning claims.
func (s *Signer) Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrMalformed
	}
	body := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, s.sign(body)) {
		return Claims{}, ErrBadSignature
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return Claims{}, ErrMalformed
	}
	if time.Now().Unix() >= c.Exp {
		return Claims{}, ErrExpired
	}
	return c, nil
}

func (s *Signer) sign(body string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(body))
	return m.Sum(nil)
}

func base64URL(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
