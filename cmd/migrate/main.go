package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5"

	"nac/internal/config"
	"nac/internal/database"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	pool, err := database.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		log.Fatalf("connect postgres failed: %v", err)
	}
	defer pool.Close()

	migrationsDir := resolveMigrationsDir()

	if err := ensureSchemaMigrationsTable(ctx, pool); err != nil {
		log.Fatalf("ensure schema_migrations failed: %v", err)
	}

	files, err := collectMigrationFiles(migrationsDir)
	if err != nil {
		log.Fatalf("collect migrations failed: %v", err)
	}

	if len(files) == 0 {
		log.Printf("no migration files found in %s", migrationsDir)
		return
	}

	applied := 0
	for _, file := range files {
		name := filepath.Base(file)
		done, err := isApplied(ctx, pool, name)
		if err != nil {
			log.Fatalf("check migration %s failed: %v", name, err)
		}
		if done {
			log.Printf("skip %s (already applied)", name)
			continue
		}

		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("read migration %s failed: %v", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			log.Fatalf("begin tx for %s failed: %v", name, err)
		}

		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("execute migration %s failed: %v", name, err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("record migration %s failed: %v", name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			log.Fatalf("commit migration %s failed: %v", name, err)
		}

		applied++
		log.Printf("applied %s", name)
	}

	log.Printf("migration complete, applied=%d", applied)
}

func resolveMigrationsDir() string {
	candidates := []string{
		"migrations",
		filepath.Join("nac", "migrations"),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return "migrations"
}

func collectMigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}

	sort.Strings(files)
	return files, nil
}

func ensureSchemaMigrationsTable(ctx context.Context, pool interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func isApplied(ctx context.Context, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, filename string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename = $1)`, filename).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
