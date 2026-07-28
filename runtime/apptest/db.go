package apptest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Alfian57/gin-kit/runtime/database"
	"github.com/pressly/goose/v3"
)

var sqliteCounter atomic.Uint64

// SQLiteConfig returns a database configuration for a unique in-memory
// SQLite database (shared cache, so every pooled connection sees the same
// data).
func SQLiteConfig(t *testing.T, orm database.ORM) database.Config {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_", "'", "").Replace(t.Name())
	dsn := fmt.Sprintf("file:apptest_%s_%d?mode=memory&cache=shared", name, sqliteCounter.Add(1))
	return database.Config{Dialect: database.SQLite, DSN: dsn, ORM: orm}
}

// OpenSQLite opens a unique in-memory SQLite connection for integration
// tests. One connection is pinned for the test's lifetime so pool churn
// cannot drop the shared-cache database, and everything closes via
// t.Cleanup. The test is skipped when the SQLite driver is unavailable
// (CGO disabled).
func OpenSQLite(t *testing.T, orm database.ORM) *database.Connection {
	t.Helper()
	ctx := context.Background()
	connection, err := database.Open(ctx, SQLiteConfig(t, orm))
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") || strings.Contains(err.Error(), "CGO_ENABLED") {
			t.Skipf("apptest: sqlite driver unavailable: %v", err)
		}
		t.Fatalf("apptest: open sqlite: %v", err)
	}
	pinned, err := connection.SQL.Conn(ctx)
	if err != nil {
		connection.Close()
		t.Fatalf("apptest: pin sqlite connection: %v", err)
	}
	t.Cleanup(func() {
		pinned.Close()
		connection.Close()
	})
	return connection
}

// Migrate applies the goose migrations in dir (e.g. "migrations") to the
// connection.
func Migrate(t *testing.T, connection *database.Connection, dialect database.Dialect, dir string) {
	t.Helper()
	gooseDialect := string(dialect)
	switch dialect {
	case database.SQLite:
		gooseDialect = "sqlite3"
	case database.MariaDB:
		gooseDialect = "mysql"
	}
	provider, err := goose.NewProvider(goose.Dialect(gooseDialect), connection.SQL, os.DirFS(dir),
		goose.WithVerbose(false))
	if err != nil {
		t.Fatalf("apptest: goose provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("apptest: migrate up: %v", err)
	}
}

// Seed runs seeder functions matching the generated Seeder.Run signature.
func Seed(t *testing.T, connection *database.Connection, seeders ...func(context.Context, *database.Connection) error) {
	t.Helper()
	ctx := context.Background()
	for index, seeder := range seeders {
		if err := seeder(ctx, connection); err != nil {
			t.Fatalf("apptest: seeder %d: %v", index, err)
		}
	}
}
