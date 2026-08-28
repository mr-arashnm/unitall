// Notification service entrypoint: wiring + background workers.
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
	"unital/backend/services/notifications/internal/adapter/channel"
	httpapi "unital/backend/services/notifications/internal/adapter/httpapi"
	"unital/backend/services/notifications/internal/adapter/store/memory"
	sqlstore "unital/backend/services/notifications/internal/adapter/store/sql"
	"unital/backend/services/notifications/internal/domain"
	"unital/backend/services/notifications/internal/usecase"
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

	b := memory.New()
	membership := domain.MembershipChecker(b.Directory)
	if config.Str("DEV_TRUST_ALL", "0") == "1" {
		membership = devtools.TrustAllMembership{Warn: "notifications"}
	}
	// channels constructed with env config (log adapters when unconfigured)
	chans := []domain.Channel{
		&channel.InApp{Inbox: b.Inbox},
		channel.Email(),
		channel.SMS(),
		channel.Webhook(),
	}
	notifier := usecase.New(b.Templates, b.Campaigns, b.Deliveries, b.Inbox, b.Directory, membership, chans, events.LogPublisher{})

	var tickets domain.TicketStore = b.Tickets
	var ticketResp domain.TicketResponseStore = b.TicketResp
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
		tickets = sqlstore.NewTicketStore(db)
		ticketResp = sqlstore.NewTicketResponseStore(db)
	}

	comms := usecase.NewComms(b.Announcements, b.Meetings, b.Attendance, b.Minutes,
		tickets, ticketResp, b.Inbox, b.Directory, membership, events.LogPublisher{})
	seedTemplates(notifier)

	// dev convenience: demo residents so the pipeline is exercisable out of
	// the box (disable with DEV_SEED=0; production resolves via identity).
	if config.Str("DEV_SEED", "1") == "1" {
		b.Directory.SeedMembership("demo-manager", "demo-building", "manager")
		b.Directory.SeedMembership("demo-sara", "demo-building", "owner")
		b.Directory.SeedMembership("demo-ali", "demo-building", "resident")
		b.Directory.Seed("demo-building",
			domain.Recipient{ID: "demo-sara", Name: "Sara", Email: "sara@example.com", Phone: "+989120000001"},
			domain.Recipient{ID: "demo-ali", Name: "Ali", Phone: "+989120000002"},
		)
		slog.Info("dev directory seeded", "building", "demo-building", "users", 2)
	}

	// workers: dispatch every 5s, activate scheduled campaigns every 30s
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for range t.C {
			if _, err := notifier.Dispatch(context.Background(), 100); err != nil {
				slog.Error("dispatch", "err", err)
			}
		}
	}()
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			if err := notifier.ActivateScheduled(context.Background()); err != nil {
				slog.Error("activate scheduled", "err", err)
			}
		}
	}()

	signer := jwtx.NewSigner(config.Str("JWT_SECRET", "dev-secret-change-me"), 15*time.Minute)
	addr := config.Str("ADDR", ":9006")
	srv := &http.Server{
		Addr: addr, Handler: httpapi.NewServer(httpapi.New(notifier, comms, signer)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("notifications listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

// seedTemplates registers the default event templates (NOTIFICATION_DESIGN §5).
func seedTemplates(n *usecase.Notifier) {
	ctx := context.Background()
	defaults := []*domain.Template{
		{
			Name: "charge.overdue.reminder", Severity: domain.SevUrgent,
			Channels: []string{domain.ChanInApp, domain.ChanSMS},
			Variants: map[string]domain.TemplateVariant{
				domain.ChanInApp: {Title: "یادآوری شارژ معوق", Body: "شارژ دوره {{period}} شما معوق است. مبلغ باقی‌مانده: {{remaining}} ریال."},
				domain.ChanSMS:   {Title: "یونیتال", Body: "یونیتال: شارژ دوره {{period}} معوق است. لطفاً پرداخت کنید."},
			},
		},
		{
			Name: "payment.receipt", Severity: domain.SevNormal,
			Channels: []string{domain.ChanInApp, domain.ChanEmail},
			Variants: map[string]domain.TemplateVariant{
				domain.ChanInApp: {Title: "رسید پرداخت", Body: "پرداخت مبلغ {{amount}} ریال برای دوره {{period}} با شماره پیگیری {{ref}} ثبت شد."},
				domain.ChanEmail: {Title: "رسید پرداخت شارژ — یونیتال", Body: "پرداخت شما مبلغ {{amount}} ریال برای دوره {{period}} ثبت شد.\nشماره پیگیری: {{ref}}"},
			},
		},
		{
			Name: "charges.generated.summary", Severity: domain.SevNormal,
			Channels: []string{domain.ChanInApp},
			Variants: map[string]domain.TemplateVariant{
				domain.ChanInApp: {Title: "شارژ دوره جدید صادر شد", Body: "شارژهای دوره {{period}} برای {{units}} واحد صادر شد."},
			},
		},
		{
			Name: "booking.confirmed.reminder", Severity: domain.SevNormal,
			Channels: []string{domain.ChanInApp, domain.ChanSMS},
			Variants: map[string]domain.TemplateVariant{
				domain.ChanInApp: {Title: "رزرو تأیید شد", Body: "رزرو {{facility}} برای {{start}} تأیید شد."},
				domain.ChanSMS:   {Title: "یونیتال", Body: "یونیتال: رزرو {{facility}} تأیید شد."},
			},
		},
	}
	for _, t := range defaults {
		if _, err := n.UpsertTemplate(ctx, "system:seed", t); err != nil {
			slog.Error("seed template", "name", t.Name, "err", err)
		}
	}
}
