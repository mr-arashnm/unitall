// Identity service entrypoint: wiring only.
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
	"unital/backend/pkg/jwtx"
	"unital/backend/services/identity/internal/adapter/httpapi"
	"unital/backend/services/identity/internal/adapter/store/memory"
	sqlstore "unital/backend/services/identity/internal/adapter/store/sql"
	"unital/backend/services/identity/internal/domain"
	"unital/backend/services/identity/internal/usecase"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// runMigrations runs embedded SQL migrations against db on startup.

// runMigrations executes each SQL statement in the migration files individually
// so that the simple protocol (default for lib/pq) handles them correctly.
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
		// Strip line-level comments first.
		var lines []string
		for _, line := range strings.Split(string(raw), "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "--") {
				continue
			}
			lines = append(lines, line)
		}
		cleaned := strings.Join(lines, "\n")
		// Split on `;` and execute each statement individually. lib/pq's
		// simple protocol (the default) only handles one statement per
		// Exec call.
		for _, stmt := range strings.Split(cleaned, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			res, err := db.ExecContext(context.Background(), stmt)
			if err != nil {
				msg := err.Error()
				ignored := strings.Contains(msg, "already exists") ||
					strings.Contains(msg, "duplicate") ||
					strings.Contains(msg, "does not exist")
				if !ignored {
					preview := stmt
					if len(preview) > 100 {
						preview = preview[:100] + "..."
					}
					slog.Error("migration stmt failed", "file", e.Name(), "stmt", preview, "err", err)
				}
			} else {
				// Log every successful statement for debugging.
				var rowsAffected int64 = -1
				if res != nil {
					rowsAffected, _ = res.RowsAffected()
				}
				slog.Info("migration stmt ok",
					"file", e.Name(),
					"rows", rowsAffected,
					"stmt", func() string {
						if len(stmt) > 60 {
							return stmt[:60] + "..."
						}
						return stmt
					}())
			}
		}
		slog.Info("applied migration", "file", e.Name())
	}
}

// devMailer drops mail on the floor. With DEV_AUTOVERIFY=1 it also flips
// the account to verified immediately so the auth flow is testable
// without receiving email (dev only).
type devMailer struct {
	autoVerify func(token string)
}

func (m devMailer) SendVerification(_ context.Context, to, token string) error {
	if m.autoVerify != nil {
		m.autoVerify(token)
	}
	return nil
}

func (m devMailer) SendPasswordReset(_ context.Context, to, token string) error { return nil }

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	var users domain.UserStore
	var mships domain.MembershipStore
	var refresh domain.RefreshStore

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
		// Quick sanity check: confirm we can list tables in public schema.
		var n int
		if err := db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM pg_tables WHERE schemaname='public'`).Scan(&n); err != nil {
			slog.Error("count tables", "err", err)
		} else {
			slog.Info("public tables before migrations", "count", n)
		}
		slog.Info("connected to postgres")
		sqlUsers := sqlstore.NewUsers(db)
		users = sqlUsers
		refresh = sqlUsers // Users satisfies both UserStore and RefreshStore
		mships = sqlstore.NewMemberships(db)
		runMigrations(db)
		// Sanity: how many public tables now?
		var afterN int
		_ = db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM pg_tables WHERE schemaname='public'`).Scan(&afterN)
		slog.Info("public tables after migrations", "count", afterN)
	} else {
		slog.Info("no DB_URL — using in-memory stores")
		memUsers := memory.NewUsers()
		users = memUsers
		refresh = memUsers
		mships = memory.NewMemberships()
	}

	mailer := &devMailer{}
	auth := usecase.NewAuth(users, refresh, mailer, 30*24*time.Hour)
	if config.Str("DEV_AUTOVERIFY", "0") == "1" {
		mailer.autoVerify = func(token string) {
			if err := auth.Verify(context.Background(), token); err != nil {
				slog.Error("dev autoverify", "err", err)
			}
		}
	}
	members := usecase.NewMemberships(mships, users)
	signer := jwtx.NewSigner(config.Str("JWT_SECRET", "dev-secret-change-me"), 24*time.Hour)

	addr := config.Str("ADDR", ":9001")
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewServer(httpapi.New(auth, members, signer)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("identity listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
