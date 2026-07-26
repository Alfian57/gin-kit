package apptest

import (
	"context"
	"testing"

	"github.com/Alfian57/gin-kit/framework/database"
)

func TestOpenSQLiteAndMigrate(t *testing.T) {
	for _, orm := range []database.ORM{database.GORM, database.SQLX} {
		t.Run(string(orm), func(t *testing.T) {
			connection := OpenSQLite(t, orm)
			Migrate(t, connection, database.SQLite, "testdata/migrations")

			Seed(t, connection, func(ctx context.Context, db *database.Connection) error {
				_, err := db.SQL.ExecContext(ctx,
					"INSERT INTO notes (id, body, created_at) VALUES ('1', 'hello', CURRENT_TIMESTAMP)")
				return err
			})
			var count int
			if err := connection.SQL.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("count = %d", count)
			}
			switch orm {
			case database.GORM:
				if connection.GORM == nil {
					t.Fatal("GORM handle missing")
				}
			case database.SQLX:
				if connection.SQLX == nil {
					t.Fatal("SQLX handle missing")
				}
			}
		})
	}
}

func TestSQLiteDatabasesAreIsolated(t *testing.T) {
	first := OpenSQLite(t, database.ORMNone)
	second := OpenSQLite(t, database.ORMNone)
	if _, err := first.SQL.Exec("CREATE TABLE isolated (id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := second.SQL.Exec("SELECT * FROM isolated"); err == nil {
		t.Fatal("databases are not isolated")
	}
}
