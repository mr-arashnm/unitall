// Facilities service entrypoint.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"unital/backend/pkg/config"
	"unital/backend/pkg/devtools"
	"unital/backend/pkg/events"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/facilities/internal/adapter/httpapi"
	"unital/backend/services/facilities/internal/adapter/store/memory"
	"unital/backend/services/facilities/internal/domain"
	"unital/backend/services/facilities/internal/usecase"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	b := memory.New()
	membership := domain.MembershipChecker(b.Membership)
	if config.Str("DEV_TRUST_ALL", "0") == "1" {
		membership = devtools.TrustAllMembership{Warn: "facilities"}
	}
	fac := usecase.New(b.Facilities, b.Bookings, b.Maintenance, membership, events.LogPublisher{})
	signer := jwtx.NewSigner(config.Str("JWT_SECRET", "dev-secret-change-me"), 15*time.Minute)

	addr := config.Str("ADDR", ":9004")
	srv := &http.Server{Addr: addr, Handler: httpapi.NewServer(httpapi.New(fac, signer)), ReadHeaderTimeout: 5 * time.Second}
	slog.Info("facilities listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
