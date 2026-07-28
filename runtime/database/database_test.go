package database

import (
	"context"
	"testing"
)

func TestOpenRejectsMissingOrUnsupportedConfiguration(t *testing.T) {
	if _, err := Open(context.Background(), Config{}); err == nil {
		t.Fatal("expected missing dialect error")
	}
	if _, err := Open(context.Background(), Config{Dialect: "oracle", DSN: "x"}); err == nil {
		t.Fatal("expected unsupported dialect error")
	}
}

func TestDriverNames(t *testing.T) {
	for dialect, expected := range map[Dialect]string{MySQL: "mysql", MariaDB: "mysql", Postgres: "pgx", SQLite: "sqlite3"} {
		got, err := driverName(dialect)
		if err != nil || got != expected {
			t.Fatalf("%s: got %q, %v", dialect, got, err)
		}
	}
}
