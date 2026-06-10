package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/SolaTyolo/mcphub/internal/config"
	"github.com/SolaTyolo/mcphub/internal/logx"
	_ "modernc.org/sqlite"
)

const migrateTag = "migrate"

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	switch cfg.Store.Kind {
	case config.StoreFile:
		logx.Info(migrateTag, "file store (%s) — no SQL migrations needed", cfg.Store.FilePath)
		return
	case config.StoreSQLite:
		runSQLite(cfg)
	default:
		runPostgres(cfg)
	}
}

func migrationsDir(cfg config.Config) string {
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir != "" {
		return dir
	}
	switch cfg.Store.Kind {
	case config.StoreSQLite:
		return "migrations/sqlite"
	default:
		return "migrations/postgres"
	}
}

func runPostgres(cfg config.Config) {
	dir := migrationsDir(cfg)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Store.DSN)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	if err := ensurePGMigrationTable(ctx, pool); err != nil {
		log.Fatalf("schema_migrations: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		log.Fatalf("glob migrations: %v", err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		log.Fatalf("no migration files in %s", dir)
	}

	applied := 0
	for _, path := range files {
		version := filepath.Base(path)
		ok, err := isPGApplied(ctx, pool, version)
		if err != nil {
			log.Fatalf("check %s: %v", version, err)
		}
		if ok {
			logx.Info(migrateTag, "skip %s (already applied)", version)
			continue
		}
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("read %s: %v", path, err)
		}
		sqlText := strings.TrimSpace(string(sqlBytes))
		if sqlText == "" {
			continue
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			log.Fatalf("begin tx for %s: %v", version, err)
		}
		if _, err := tx.Exec(ctx, sqlText); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("apply %s: %v", version, err)
		}
		if _, err := tx.Exec(ctx,
			`insert into schema_migrations (version, applied_at) values ($1, $2)`,
			version, time.Now().UTC(),
		); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("record %s: %v", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			log.Fatalf("commit %s: %v", version, err)
		}
		logx.Info(migrateTag, "applied %s", version)
		applied++
	}
	logMigrateDone(applied, len(files))
}

func runSQLite(cfg config.Config) {
	dir := migrationsDir(cfg)
	dsn := cfg.Store.DSN
	if dsn == "" {
		log.Fatalf("sqlite: missing LLM_STORE_DSN")
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("sqlite: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := ensureSQLiteMigrationTable(db); err != nil {
		log.Fatalf("schema_migrations: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		log.Fatalf("glob migrations: %v", err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		log.Fatalf("no migration files in %s", dir)
	}

	applied := 0
	for _, path := range files {
		version := filepath.Base(path)
		ok, err := isSQLiteApplied(db, version)
		if err != nil {
			log.Fatalf("check %s: %v", version, err)
		}
		if ok {
			logx.Info(migrateTag, "skip %s (already applied)", version)
			continue
		}
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("read %s: %v", path, err)
		}
		sqlText := strings.TrimSpace(string(sqlBytes))
		if sqlText == "" {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("begin tx for %s: %v", version, err)
		}
		if _, err := tx.Exec(sqlText); err != nil {
			_ = tx.Rollback()
			log.Fatalf("apply %s: %v", version, err)
		}
		if _, err := tx.Exec(
			`insert into schema_migrations (version, applied_at) values (?, ?)`,
			version, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			log.Fatalf("record %s: %v", version, err)
		}
		if err := tx.Commit(); err != nil {
			log.Fatalf("commit %s: %v", version, err)
		}
		logx.Info(migrateTag, "applied %s", version)
		applied++
	}
	logMigrateDone(applied, len(files))
}

func logMigrateDone(applied, total int) {
	if applied == 0 {
		logx.Info(migrateTag, "database up to date (%d migrations checked)", total)
	} else {
		logx.Info(migrateTag, "done, applied %d migration(s)", applied)
	}
}

func ensurePGMigrationTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
create table if not exists schema_migrations (
  version    text primary key,
  applied_at timestamptz not null
);
`)
	return err
}

func isPGApplied(ctx context.Context, pool *pgxpool.Pool, version string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		`select exists(select 1 from schema_migrations where version = $1)`, version,
	).Scan(&exists)
	return exists, err
}

func ensureSQLiteMigrationTable(db *sql.DB) error {
	_, err := db.Exec(`
create table if not exists schema_migrations (
  version    text primary key,
  applied_at text not null
);
`)
	return err
}

func isSQLiteApplied(db *sql.DB, version string) (bool, error) {
	var exists int
	err := db.QueryRow(`select count(1) from schema_migrations where version = ?`, version).Scan(&exists)
	return exists > 0, err
}
