package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"skyrouter/internal/config"
)

// Connect opens and verifies a PostgreSQL connection pool.
func Connect(cfg config.DatabaseConfig) (*sql.DB, error) {
	pool, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(5 * time.Minute)
	pool.SetConnMaxIdleTime(1 * time.Minute)

	if err := pool.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
