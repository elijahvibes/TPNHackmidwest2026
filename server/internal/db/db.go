// Package db connects to MySQL (Akamai Managed Database in prod, a local
// MariaDB/MySQL in dev) and applies the embedded migrations on boot.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DSN accepts either the Akamai/Aiven console format
//
//	mysql://user:pass@host:port/db?ssl-mode=REQUIRED
//
// or a native go-sql-driver DSN, and returns a native DSN with the options
// the app needs (TLS, parseTime, multiStatements for migrations).
func DSN(raw string) (string, error) {
	if !strings.HasPrefix(raw, "mysql://") {
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	pass, _ := u.User.Password()
	name := strings.TrimPrefix(u.Path, "/")
	tls := "false"
	if m := u.Query().Get("ssl-mode"); m != "" && !strings.EqualFold(m, "DISABLED") {
		// Managed DBs use a private CA; skip-verify keeps encryption without cert pinning.
		tls = "skip-verify"
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?tls=%s&parseTime=true&multiStatements=true&charset=utf8mb4&loc=UTC",
		u.User.Username(), pass, u.Host, name, tls), nil
}

// Connect opens a pool, pings, and applies every migrations/*.sql in name
// order. Migrations are idempotent (IF NOT EXISTS) so re-running is safe.
func Connect(ctx context.Context, raw string) (*sql.DB, error) {
	dsn, err := DSN(raw)
	if err != nil {
		return nil, err
	}
	pool, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}
	pool.SetMaxOpenConns(10)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(5 * time.Minute)
	if err := pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func migrate(ctx context.Context, pool *sql.DB) error {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		raw, err := migrations.ReadFile("migrations/" + n)
		if err != nil {
			return err
		}
		if _, err := pool.ExecContext(ctx, string(raw)); err != nil {
			return fmt.Errorf("migration %s: %w", n, err)
		}
	}
	return nil
}
