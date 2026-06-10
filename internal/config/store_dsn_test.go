package config

import "testing"

func TestParseStoreDSN(t *testing.T) {
	tests := []struct {
		raw      string
		kind     StoreKind
		dsn      string
		filePath string
	}{
		{
			raw:  "postgres://llm:llm@localhost:5433/llm?sslmode=disable",
			kind: StorePostgres,
			dsn:  "postgres://llm:llm@localhost:5433/llm?sslmode=disable",
		},
		{
			raw:  "sqlite://./data/llm.db",
			kind: StoreSQLite,
			dsn:  "file:./data/llm.db?cache=shared&mode=rwc",
		},
		{
			raw:  "file:./data/llm.db",
			kind: StoreSQLite,
			dsn:  "file:./data/llm.db?cache=shared&mode=rwc",
		},
		{
			raw:      "file://./data/store.yml",
			kind:     StoreFile,
			filePath: "./data/store.yml",
		},
		{
			raw:      "file:./data/store.yaml",
			kind:     StoreFile,
			filePath: "./data/store.yaml",
		},
		{
			raw:      "",
			kind:     StoreFile,
			filePath: "./data/store.yml",
		},
	}

	for _, tt := range tests {
		got := ParseStoreDSN(tt.raw)
		if got.Kind != tt.kind {
			t.Errorf("ParseStoreDSN(%q).Kind = %q, want %q", tt.raw, got.Kind, tt.kind)
		}
		if tt.dsn != "" && got.DSN != tt.dsn {
			t.Errorf("ParseStoreDSN(%q).DSN = %q, want %q", tt.raw, got.DSN, tt.dsn)
		}
		if tt.filePath != "" && got.FilePath != tt.filePath {
			t.Errorf("ParseStoreDSN(%q).FilePath = %q, want %q", tt.raw, got.FilePath, tt.filePath)
		}
	}
}
