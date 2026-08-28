// API gateway: JWT validation, user-context injection, Swagger docs,
// and routing to upstream services (see docs/API_DESIGN.md and
// openapi/unital-v1.yaml).
package main

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"unital/backend/openapi"
	"unital/backend/pkg/config"
	"unital/backend/pkg/httpx"
	"unital/backend/pkg/jwtx"
)

type service struct {
	name     string
	upstream *url.URL
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	signer := jwtx.NewSigner(config.Str("JWT_SECRET", "dev-secret-change-me"), 24*time.Hour)
	internalToken := config.Str("INTERNAL_TOKEN", "dev-internal-token")

	mustURL := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			slog.Error("bad upstream URL", "raw", raw, "err", err)
			os.Exit(1)
		}
		return u
	}

	svc := map[string]service{
		"identity":   {name: "identity", upstream: mustURL(config.Str("IDENTITY_URL", "http://localhost:9001"))},
		"property":   {name: "property", upstream: mustURL(config.Str("PROPERTY_URL", "http://localhost:9002"))},
		"billing":    {name: "billing", upstream: mustURL(config.Str("BILLING_URL", "http://localhost:9003"))},
		"facilities": {name: "facilities", upstream: mustURL(config.Str("FACILITIES_URL", "http://localhost:9004"))},
		"operations": {name: "operations", upstream: mustURL(config.Str("OPERATIONS_URL", "http://localhost:9005"))},
		"notif":      {name: "notifications", upstream: mustURL(config.Str("NOTIFICATIONS_URL", "http://localhost:9006"))},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { httpx.JSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/schema", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(openapi.Spec)
	})
	mux.HandleFunc("GET /api/docs", swaggerUI)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target, public := pickService(r.URL.Path)
		s, ok := svc[target]
		if !ok {
			httpx.WriteError(w, r, httpx.NewProblem("gateway", "NOT_FOUND", "Route not found", http.StatusNotFound))
			return
		}
		if !public {
			// /internal/* accepts a shared service token in lieu of a user JWT
			if isInternalRequest(r) {
				if r.Header.Get("X-Internal-Token") != internalToken {
					httpx.WriteError(w, r, httpx.NewProblem("gateway", "UNAUTHORIZED", "Valid service token required", http.StatusUnauthorized))
					return
				}
				r.Header.Set("X-User-Id", "internal")
				r.Header.Set("X-User-Role", "internal")
			} else {
				claims, ok := auth(r, signer)
				if !ok {
					httpx.WriteError(w, r, httpx.NewProblem("gateway", "UNAUTHORIZED", "Valid bearer token required", http.StatusUnauthorized))
					return
				}
				r.Header.Set("X-User-Id", claims.Sub)
				r.Header.Set("X-User-Role", claims.Role)
			}
		}
		// strip the /api/v1 prefix before forwarding to the service
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api/v1")
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		proxy := httputil.NewSingleHostReverseProxy(s.upstream)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("upstream", "err", err, "path", r.URL.Path)
			httpx.WriteError(w, r, httpx.NewProblem("gateway", "BAD_GATEWAY", "Upstream unavailable: "+s.name, http.StatusBadGateway))
		}
		proxy.ServeHTTP(w, r)
	})

	addr := config.Str("ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: httpx.Chain(mux, cors, httpx.RequestID, httpx.AccessLog, httpx.Recover), ReadHeaderTimeout: 5 * time.Second}
	slog.Info("gateway listening", "addr", addr, "docs", addr+"/api/docs")
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

// pickService maps a gateway path to its owning service. Suffix checks
// come first because billing, facilities, operations, and notification
// routes are nested under /buildings/{id}/…, which property also owns.
func pickService(path string) (name string, public bool) {
	p := strings.TrimPrefix(path, "/api/v1")
	switch {
	case strings.HasPrefix(p, "/internal"):
		// Service-to-service endpoints. Today the only internal surface
		// is identity's user/membership bootstrap, so the whole
		// /internal/* tree routes there. Auth happens via the shared
		// internal token in the request handler, not a user JWT.
		return "identity", false
	case strings.HasPrefix(p, "/auth"):
		return "identity", true
	case strings.HasPrefix(p, "/me/notifications"):
		return "notif", false
	case p == "/me" || strings.HasPrefix(p, "/me/"):
		return "identity", false // segment-exact: /meetings must not match here
	case strings.HasPrefix(p, "/templates"), strings.HasPrefix(p, "/notifications"):
		return "notif", false
	case strings.Contains(p, "/announcements"), strings.Contains(p, "/meetings"),
		strings.Contains(p, "/tickets"):
		return "notif", false // communications features
	case strings.Contains(p, "/memberships"):
		return "identity", false // building membership management
	case strings.Contains(p, "/charge-templates"), strings.Contains(p, "/charges"),
		strings.Contains(p, "/invoices"), strings.Contains(p, "/transactions"),
		strings.Contains(p, "/reports/financial"):
		return "billing", false
	case strings.Contains(p, "/facilities"), strings.Contains(p, "/maintenance-windows"),
		strings.HasPrefix(p, "/bookings"):
		return "facilities", false
	case strings.Contains(p, "/teams"), strings.Contains(p, "/tasks"),
		strings.Contains(p, "/service-requests"):
		return "operations", false
	case strings.HasPrefix(p, "/buildings"), strings.HasPrefix(p, "/units"),
		strings.HasPrefix(p, "/contracts"), strings.HasPrefix(p, "/parkings"),
		strings.HasPrefix(p, "/warehouses"), strings.HasPrefix(p, "/assets"):
		return "property", false
	default:
		return "", false
	}
}

// isInternalRequest reports whether the path is a service-to-service
// call. These bypass user JWT auth and use a shared internal token.
func isInternalRequest(r *http.Request) bool {
	p := strings.TrimPrefix(r.URL.Path, "/api/v1")
	return strings.HasPrefix(p, "/internal")
}

// cors allows browser frontends to call the API directly (the Vite dev
// proxy bypasses this). Restrict CORS_ORIGINS in production.
func cors(next http.Handler) http.Handler {
	allowed := config.Str("CORS_ORIGINS", "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// swaggerUI serves a minimal page loading Swagger UI (CDN) against the
// embedded schema. Self-host the assets if the deployment is offline.
func swaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
  <title>Unital API — Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css"/>
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js" crossorigin></script>
<script>
  window.ui = SwaggerUIBundle({
    url: "/api/schema",
    dom_id: "#swagger-ui",
    deepLinking: true,
    persistAuthorization: true,
    tryItOutEnabled: true,
  });
</script>
</body>
</html>`))
}

func auth(r *http.Request, signer *jwtx.Signer) (jwtx.Claims, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return jwtx.Claims{}, false
	}
	claims, err := signer.Parse(strings.TrimPrefix(h, "Bearer "))
	if err != nil {
		return jwtx.Claims{}, false
	}
	return claims, true
}
