// Property service entrypoint: wiring only.
package main

import (
	"context"
	"database/sql"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"unital/backend/pkg/config"
	"unital/backend/pkg/devtools"
	"unital/backend/pkg/events"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/property/internal/adapter/httpapi"
	"unital/backend/services/property/internal/adapter/store/memory"
	sqlstore "unital/backend/services/property/internal/adapter/store/sql"
	"unital/backend/services/property/internal/domain"
	"unital/backend/services/property/internal/usecase"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// runMigrations executes each SQL statement in the migration files individually
// so the lib/pq simple protocol handles them correctly.
func runMigrations(db *sql.DB) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		slog.Error("embed readdir migrations", "err", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		raw, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			slog.Error("read migration", "file", e.Name(), "err", err)
			continue
		}
			// Strip line-level comments first, then split on `;`.
		var lines []string
		for _, line := range strings.Split(string(raw), "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "--") {
				continue
			}
			lines = append(lines, line)
		}
		cleaned := strings.Join(lines, "\n")
		for _, stmt := range strings.Split(cleaned, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.ExecContext(context.Background(), stmt); err != nil {
				if !strings.Contains(err.Error(), "already exists") &&
					!strings.Contains(err.Error(), "duplicate") &&
					!strings.Contains(err.Error(), "does not exist") {
					preview := stmt
					if len(preview) > 80 {
						preview = preview[:80] + "..."
					}
					slog.Error("migration stmt failed", "file", e.Name(), "stmt", preview, "err", err)
				}
			}
		}
		slog.Info("applied migration", "file", e.Name())
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	var prop *usecase.Property
	if dbURL := config.Str("DB_URL", ""); dbURL != "" {
		db, err := sql.Open("postgres", dbURL)
		if err != nil {
			slog.Error("open db", "err", err)
			os.Exit(1)
		}
		if err := db.Ping(); err != nil {
			slog.Error("ping db", "err", err)
			os.Exit(1)
		}
		slog.Info("connected to postgres")
		runMigrations(db)
		var membership domain.MembershipChecker = sqlstore.NewMembershipTable()
		if config.Str("DEV_TRUST_ALL", "0") == "1" {
			membership = devtools.TrustAllMembership{Warn: "property"}
		}
		prop = usecase.New(
			sqlstore.NewBuildingStore(db),
			sqlstore.NewUnitStore(db),
			sqlstore.NewAssetStore(db),
			sqlstore.NewPartyStore(db),
			sqlstore.NewContractStore(db),
			membership,
			events.LogPublisher{},
		)
	} else {
		bundle := memory.New()
		membership := domain.MembershipChecker(bundle.Membership)
		if config.Str("DEV_TRUST_ALL", "0") == "1" {
			membership = devtools.TrustAllMembership{Warn: "property"}
		}
		prop = usecase.New(bundle.Buildings, bundle.Units, bundle.Assets, bundle.Parties, bundle.Contracts, membership, events.LogPublisher{})
	}

	// Identity bootstrap: building creation also records the creator as a
	// manager in identity's membership table.
	gw := config.Str("GATEWAY_URL", "http://gateway:8080")
	if config.Str("DEV_BOOTSTRAP_OFF", "0") != "1" {
		prop = prop.WithBootstrap(usecase.NewHTTPBootstrap(gw, config.Str("INTERNAL_TOKEN", "dev-internal-token")))
	}
	signer := jwtx.NewSigner(config.Str("JWT_SECRET", "dev-secret-change-me"), 15*time.Minute)

	addr := config.Str("ADDR", ":9002")
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewServer(httpapi.New(prop, signer)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("property listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
