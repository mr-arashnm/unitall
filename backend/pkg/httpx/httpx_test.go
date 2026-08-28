package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMintsWhenAbsent(t *testing.T) {
	var seen string
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = TraceID(r.Context())
	}), RequestID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if seen == "" {
		t.Fatal("TraceID empty inside handler")
	}
	if got := rec.Header().Get("X-Request-Id"); got != seen {
		t.Fatalf("response header %q != context id %q", got, seen)
	}
}

func TestRequestIDHonorsIncomingHeader(t *testing.T) {
	var seen, echoed string
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = TraceID(r.Context())
		echoed = r.Header.Get("X-Request-Id") // what an upstream proxy would forward
	}), RequestID)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-Id", "gateway-trace-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != "gateway-trace-123" || echoed != "gateway-trace-123" {
		t.Fatalf("incoming id not propagated: ctx=%q header=%q", seen, echoed)
	}
	if got := rec.Header().Get("X-Request-Id"); got != "gateway-trace-123" {
		t.Fatalf("response header %q", got)
	}
}

func TestWriteErrorCarriesTraceID(t *testing.T) {
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, NewProblem("tests", "NOPE", "nope", http.StatusTeapot))
	}), RequestID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	var p Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.TraceID == "" || p.TraceID != rec.Header().Get("X-Request-Id") {
		t.Fatalf("problem trace_id %q != header %q", p.TraceID, rec.Header().Get("X-Request-Id"))
	}
}

func TestAccessLogRunsInsideRequestID(t *testing.T) {
	// The chain order used by every service must expose the trace id to
	// AccessLog; this fails if someone reorders RequestID innermost.
	var ctxID, logID string
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = TraceID(r.Context())
	}), RequestID, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logID = TraceID(r.Context()) // what AccessLog would read
			next.ServeHTTP(w, r)
		})
	})
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if logID == "" || logID != ctxID {
		t.Fatalf("middleware outside RequestID sees no id: log=%q ctx=%q", logID, ctxID)
	}
}
