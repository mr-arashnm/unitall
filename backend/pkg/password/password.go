// Package password hashes secrets with PBKDF2-SHA256 (stdlib only).
// Format: pbkdf2$<iter>$<salt-hex>$<dk-hex>. Argon2id is a drop-in
// upgrade behind the same Hash/Verify signatures.
package password

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	iter    = 210_000 // OWASP 2023 guidance for PBKDF2-SHA256
	saltLen = 16
	keyLen  = 32
)

var ErrMismatch = errors.New("password does not match")

func Hash(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}
	dk, err := pbkdf2.Key(sha256.New, plain, salt, iter, keyLen)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}
	return fmt.Sprintf("pbkdf2$%d$%s$%s", iter, hex.EncodeToString(salt), hex.EncodeToString(dk)), nil
}

func Verify(plain, stored string) error {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2" {
		return errors.New("malformed password hash")
	}
	var it int
	if _, err := fmt.Sscanf(parts[1], "%d", &it); err != nil {
		return errors.New("malformed password hash")
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return errors.New("malformed password hash")
	}
	want, err := hex.DecodeString(parts[3])
	if err != nil {
		return errors.New("malformed password hash")
	}
	got, err := pbkdf2.Key(sha256.New, plain, salt, it, len(want))
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}
