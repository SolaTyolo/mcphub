package config

import (
	"path/filepath"
	"strings"
)

type StoreKind string

const (
	StorePostgres StoreKind = "postgres"
	StoreSQLite   StoreKind = "sqlite"
	StoreFile     StoreKind = "file"
)

// StoreConfig is parsed from LLM_STORE_DSN (scheme://path).
type StoreConfig struct {
	Kind     StoreKind
	RawDSN   string // original env value
	DSN      string // driver DSN for postgres / sqlite
	FilePath string // yaml path when Kind == file
}

const defaultStoreDSN = "file://./data/store.yml"

// ParseStoreDSN infers backend from URL scheme and path extension.
//
//   - postgres:// / postgresql:// → Postgres
//   - sqlite://path → SQLite (driver: file:path?…)
//   - file://path.db or file:path.db → SQLite
//   - file://path.yml / .yaml → YAML file store
func ParseStoreDSN(raw string) StoreConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = defaultStoreDSN
	}

	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return StoreConfig{Kind: StorePostgres, RawDSN: raw, DSN: raw}
	}

	if strings.HasPrefix(lower, "sqlite://") {
		return sqliteStore(raw, strings.TrimPrefix(raw, "sqlite://"))
	}
	if strings.HasPrefix(lower, "sqlite:") {
		return sqliteStore(raw, strings.TrimPrefix(raw, "sqlite:"))
	}

	if strings.HasPrefix(lower, "file://") || strings.HasPrefix(lower, "file:") {
		return classifyLocalPath(raw, extractFilePath(raw))
	}

	return classifyLocalPath(raw, raw)
}

func sqliteStore(raw, path string) StoreConfig {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "./data/llm.db"
	}
	return StoreConfig{
		Kind:   StoreSQLite,
		RawDSN: raw,
		DSN:    "file:" + path + "?cache=shared&mode=rwc",
	}
}

func classifyLocalPath(raw, path string) StoreConfig {
	path = strings.TrimSpace(path)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".db":
		return sqliteStore(raw, path)
	case ".yml", ".yaml":
		return StoreConfig{Kind: StoreFile, RawDSN: raw, FilePath: path}
	}
	if strings.HasPrefix(strings.ToLower(raw), "file:") {
		return StoreConfig{Kind: StoreFile, RawDSN: raw, FilePath: path}
	}
	return StoreConfig{Kind: StoreFile, RawDSN: raw, FilePath: path}
}

func extractFilePath(raw string) string {
	if strings.HasPrefix(raw, "file://") {
		return raw[len("file://"):]
	}
	if strings.HasPrefix(raw, "file:") {
		return raw[len("file:"):]
	}
	return raw
}

func applyLegacyStoreKind(store StoreConfig, legacy string) StoreConfig {
	switch strings.ToLower(strings.TrimSpace(legacy)) {
	case "file", "yaml", "yml":
		store.Kind = StoreFile
	case "sqlite":
		store.Kind = StoreSQLite
	case "postgres", "postgresql", "pg":
		store.Kind = StorePostgres
	}
	return store
}
