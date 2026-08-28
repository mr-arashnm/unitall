// Package httpx provides shared HTTP plumbing for all Unital services:
// RFC 9457 problem-details errors, JSON helpers, and common middleware.
package httpx

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"unital/backend/pkg/ids"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
)

// Problem is an RFC 9457 error envelope with an extra machine-readable code.
type Problem struct {
	Type    string            `json:"type,omitempty"`
	Title   string            `json:"title"`
	Status  int               `json:"status"`
	Code    string            `json:"code"`
	Detail  string            `json:"detail,omitempty"`
	TraceID string            `json:"trace_id,omitempty"`
	Errors  map[string]string `json:"errors,omitempty"`
}

func (p Problem) Error() string { return p.Code + ": " + p.Title }

// NewProblem builds a Problem. typeBase is the service error namespace,
// e.g. "identity" produces "https://api.unital.app/errors/identity/<code>".
func NewProblem(typeBase, code, title string, status int) Problem {
	return Problem{
		Type:   "https://api.unital.app/errors/" + typeBase + "/" + code,
		Title:  title,
		Status: status,
		Code:   code,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "err", err)
	}
}

// JSON writes a success response.
func JSON(w http.ResponseWriter, status int, v any) { writeJSON(w, status, v) }

// WriteError emits a problem-details response, attaching the request trace id.
func WriteError(w http.ResponseWriter, r *http.Request, p Problem) {
	if id := TraceID(r.Context()); id != "" {
		p.TraceID = id
	}
	writeJSON(w, p.Status, p)
}

// Decode parses a size-capped JSON body into dst.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	return dec.Decode(dst)
}

// Validation maps field -> message for 422 responses.
type Validation struct {
	Field   string
	Message string
}

func Invalid(typeBase string, errs ...Validation) Problem {
	p := NewProblem(typeBase, "VALIDATION_FAILED", "Request validation failed", http.StatusUnprocessableEntity)
	m := make(map[string]string, len(errs))
	for _, e := range errs {
		m[e.Field] = e.Message
	}
	p.Errors = m
	return p
}

// RequestID middleware assigns a trace id to every request: it honors an
// incoming X-Request-Id (set by the gateway, so one id spans the whole
// gateway→service hop) and otherwise mints a UUIDv7. Chain it OUTERMOST
// (before AccessLog/Recover) so downstream middleware can read the id.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = ids.New()
		}
		r.Header.Set("X-Request-Id", id) // forwarded to upstream services
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxRequestID, id)))
	})
}

// TraceID fetches the current request's trace id from a context ("" when
// absent). Use it in handlers and usecases to correlate custom log lines
// and error reports with the access log.
func TraceID(ctx context.Context) string {
	id, _ := ctx.Value(ctxRequestID).(string)
	return id
}

// Recover converts panics into 500 problems and logs them.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "trace_id", TraceID(r.Context()), "recover", rec, "path", r.URL.Path)
				WriteError(w, r, NewProblem("core", "INTERNAL", "Internal server error", http.StatusInternalServerError))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// AccessLog logs one line per request at info level, keyed by trace id.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r) // status capture omitted; keep hot path simple
		slog.Info("http", "trace_id", TraceID(r.Context()), "method", r.Method, "path", r.URL.Path, "dur_ms", time.Since(start).Milliseconds())
	})
}

// Chain applies middlewares left-to-right.
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
