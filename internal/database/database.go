package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/sambady/heretous/internal/config"
)

// Open configures the pool and verifies that PostgreSQL is reachable.
func Open(ctx context.Context, cfg config.Database) (*sql.DB, error) {
	connConfig, err := pgx.ParseConfig(cfg.URL)
	if err != nil {
		// The driver's parse error can contain the original URL and its password.
		return nil, errors.New("invalid database connection URL")
	}
	connConfig.ConnectTimeout = cfg.ConnectTimeout
	db := stdlib.OpenDB(*connConfig)
	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return db, nil
}
