// Package database provides explicit SQL, GORM, and sqlx connectors.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Dialect string

const (
	MySQL    Dialect = "mysql"
	MariaDB  Dialect = "mariadb"
	Postgres Dialect = "postgres"
	SQLite   Dialect = "sqlite"
)

type ORM string

const (
	ORMNone ORM = "none"
	GORM    ORM = "gorm"
	SQLX    ORM = "sqlx"
)

type Config struct {
	Dialect         Dialect
	DSN             string
	ORM             ORM
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	PingTimeout     time.Duration
}

type Connection struct {
	SQL  *sql.DB
	GORM *gorm.DB
	SQLX *sqlx.DB
}

func (c *Connection) Close() error {
	if c == nil || c.SQL == nil {
		return nil
	}
	return c.SQL.Close()
}

func Open(ctx context.Context, config Config) (*Connection, error) {
	if config.Dialect == "" {
		return nil, errors.New("database: dialect is required")
	}
	if config.DSN == "" {
		return nil, errors.New("database: DSN is required")
	}
	driver, err := driverName(config.Dialect)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, config.DSN)
	if err != nil {
		return nil, fmt.Errorf("database: open %s: %w", config.Dialect, err)
	}
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	}
	timeout := config.PingTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	connection := &Connection{SQL: db}
	switch config.ORM {
	case "", ORMNone:
	case GORM:
		connection.GORM, err = openGORM(db, config.Dialect)
	case SQLX:
		connection.SQLX = sqlx.NewDb(db, driver)
	default:
		err = fmt.Errorf("database: unsupported ORM %q", config.ORM)
	}
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: initialize %s: %w", config.ORM, err)
	}
	return connection, nil
}

func driverName(dialect Dialect) (string, error) {
	switch dialect {
	case MySQL, MariaDB:
		return "mysql", nil
	case Postgres:
		return "pgx", nil
	case SQLite:
		return "sqlite3", nil
	default:
		return "", fmt.Errorf("database: unsupported dialect %q", dialect)
	}
}

func openGORM(db *sql.DB, dialect Dialect) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch dialect {
	case MySQL, MariaDB:
		dialector = mysql.New(mysql.Config{Conn: db, SkipInitializeWithVersion: true})
	case Postgres:
		dialector = postgres.New(postgres.Config{Conn: db})
	case SQLite:
		dialector = sqlite.Dialector{Conn: db}
	default:
		return nil, fmt.Errorf("unsupported dialect %q", dialect)
	}
	return gorm.Open(dialector, &gorm.Config{})
}
