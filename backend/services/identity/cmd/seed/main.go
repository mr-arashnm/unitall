// Seed CLI: assigns a platform role to an existing user.
//
// Usage:
//   seed --email admin@example.com --role system_admin
//
// Available roles: system_admin, supervisor, developer.
// Pass an empty --role to revoke.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	email := flag.String("email", "", "user email to update")
	role := flag.String("role", "", "platform role to assign (system_admin|supervisor|developer) or empty to revoke")
	flag.Parse()

	if *email == "" {
		slog.Error("--email is required")
		flag.Usage()
		os.Exit(2)
	}
	if *role != "" && *role != "system_admin" && *role != "supervisor" && *role != "developer" {
		slog.Error("--role must be one of: system_admin, supervisor, developer (or empty to revoke)")
		os.Exit(2)
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		slog.Error("DB_URL env var is required")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		slog.Error("ping db", "err", err)
		os.Exit(1)
	}

	var userID string
	err = db.QueryRowContext(context.Background(), `SELECT id FROM users WHERE email=$1`, *email).Scan(&userID)
	if err == sql.ErrNoRows {
		slog.Error("user not found", "email", *email)
		os.Exit(3)
	}
	if err != nil {
		slog.Error("lookup user", "err", err)
		os.Exit(1)
	}

	res, err := db.ExecContext(context.Background(),
		`UPDATE users SET platform_role=$2, updated_at=NOW() WHERE id=$1`,
		userID, *role,
	)
	if err != nil {
		slog.Error("update role", "err", err)
		os.Exit(1)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		slog.Error("no rows updated")
		os.Exit(1)
	}

	if *role == "" {
		fmt.Printf("revoked platform role for %s (id=%s)\n", *email, userID)
	} else {
		fmt.Printf("assigned %s to %s (id=%s)\n", *role, *email, userID)
	}
}
