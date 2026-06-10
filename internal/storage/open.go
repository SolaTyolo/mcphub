package storage

import (
	"context"
	"fmt"

	"github.com/SolaTyolo/mcphub/internal/config"
)

// Open creates a Store from config (postgres, sqlite, or file/yaml).
func Open(ctx context.Context, cfg config.Config) (Store, error) {
	switch cfg.Store.Kind {
	case config.StoreFile:
		path := cfg.Store.FilePath
		if path == "" {
			return nil, fmt.Errorf("file store requires LLM_STORE_DSN (e.g. file://./data/store.yml)")
		}
		return NewFileStore(path)
	case config.StoreSQLite:
		dsn := cfg.Store.DSN
		if dsn == "" {
			return nil, fmt.Errorf("sqlite store requires LLM_STORE_DSN (e.g. sqlite://./data/llm.db or file:./data/llm.db)")
		}
		return NewSQLiteStore(ctx, dsn)
	case config.StorePostgres:
		dsn := cfg.Store.DSN
		if dsn == "" {
			return nil, fmt.Errorf("postgres store requires LLM_STORE_DSN (e.g. postgres://user:pass@host/db)")
		}
		return NewPGStore(ctx, dsn)
	default:
		return nil, fmt.Errorf("unsupported store DSN %q", cfg.Store.RawDSN)
	}
}
