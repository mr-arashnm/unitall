// Billing service entrypoint: wiring only.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"unital/backend/pkg/config"
	"unital/backend/pkg/devtools"
	"unital/backend/pkg/events"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/billing/internal/adapter/httpapi"
	"unital/backend/services/billing/internal/adapter/store/memory"
	"unital/backend/services/billing/internal/domain"
	"unital/backend/services/billing/internal/usecase"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	b := memory.New()
	membership := domain.MembershipChecker(b.Membership)
	if config.Str("DEV_TRUST_ALL", "0") == "1" {
		membership = devtools.TrustAllMembership{Warn: "billing"}
	}
	billing := usecase.New(b.Templates, b.Charges, b.Txs, b.Invoices, b.Directory, membership, events.LogPublisher{})
	signer := jwtx.NewSigner(config.Str("JWT_SECRET", "dev-secret-change-me"), 15*time.Minute)

	// Overdue sweeper: daily tick inside the service (CronJob in production).
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := billing.SweepOverdue(context.Background()); err != nil {
				slog.Error("overdue sweep", "err", err)
			}
		}
	}()

	addr := config.Str("ADDR", ":9003")
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewServer(httpapi.New(billing, signer)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("billing listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
