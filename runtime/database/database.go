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

// Dialect defines an implementation type used by this package.
type Dialect string

const (
	// MySQL define package-level implementation state.
	MySQL Dialect = "mysql"
	// MariaDB define package-level implementation state.
	MariaDB Dialect = "mariadb"
	// Postgres define package-level implementation state.
	Postgres Dialect = "postgres"
	// SQLite define package-level implementation state.
	SQLite Dialect = "sqlite"
)

// ORM defines an implementation type used by this package.
type ORM string

const (
	// ORMNone define package-level implementation state.
	ORMNone ORM = "none"
	// GORM define package-level implementation state.
	GORM ORM = "gorm"
	// SQLX define package-level implementation state.
	SQLX ORM = "sqlx"
)

// Config defines an implementation type used by this package.
type Config struct {
	// Dialect store data used by this type.
	Dialect Dialect
	// DSN store data used by this type.
	DSN string
	// ORM store data used by this type.
	ORM ORM
	// MaxOpenConns store data used by this type.
	MaxOpenConns int
	// MaxIdleConns store data used by this type.
	MaxIdleConns int
	// ConnMaxLifetime store data used by this type.
	ConnMaxLifetime time.Duration
	// PingTimeout store data used by this type.
	PingTimeout time.Duration
}

// Connection defines an implementation type used by this package.
type Connection struct {
	// SQL store data used by this type.
	SQL *sql.DB
	// GORM store data used by this type.
	GORM *gorm.DB
	// SQLX store data used by this type.
	SQLX *sqlx.DB
}

// Close performs this package operation.
func (c *Connection) Close() error {
	if c == nil || c.SQL == nil {
		return nil
	}
	return c.SQL.Close()
}

// Open performs this package operation.
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

// driverName performs this package operation.
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

// openGORM performs this package operation.
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
