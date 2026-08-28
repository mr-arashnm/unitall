// Package devtools holds dev-only helpers. TrustAllMembership answers
// yes to every authorization question — NEVER enable outside local dev;
// it exists so standalone services (without the identity internal API
// wired) can be explored via Swagger UI.
package devtools

import (
	"context"
	"log/slog"
)

// TrustAllMembership wraps any checker, allowing everything.
type TrustAllMembership struct{ Warn string }

func (t TrustAllMembership) HasRole(context.Context, string, string, string) (bool, error) {
	t.warn()
	return true, nil
}

func (t TrustAllMembership) AnyRole(context.Context, string, string, ...string) (bool, error) {
	t.warn()
	return true, nil
}

func (t TrustAllMembership) warn() {
	slog.Warn("DEV MODE: authorization bypassed (DEV_TRUST_ALL=1)")
}
