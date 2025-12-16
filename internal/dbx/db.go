package dbx

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lildannita/qtiva-cloud/internal/commonx"
)

// Открывает БД по POSTGRES_DSN и настраивает пул по переменным окружения
func OpenFromEnv() (*sql.DB, time.Duration, error) {
	dsn, err := commonx.RequireString("POSTGRES_DSN")
	if err != nil {
		return nil, 0, err
	}

	maxOpen, err := commonx.RequireInt("QTIVA_DB_MAX_OPEN")
	if err != nil {
		return nil, 0, err
	}
	maxIdle, err := commonx.RequireInt("QTIVA_DB_MAX_IDLE")
	if err != nil {
		return nil, 0, err
	}
	maxLife, err := commonx.RequireDuration("QTIVA_DB_CONN_MAX_LIFETIME")
	if err != nil {
		return nil, 0, err
	}
	maxIdleTime, err := commonx.RequireDuration("QTIVA_DB_CONN_MAX_IDLE_TIME")
	if err != nil {
		return nil, 0, err
	}
	pingTimeout, err := commonx.RequireDuration("QTIVA_DB_PING_TIMEOUT")
	if err != nil {
		return nil, 0, err
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, 0, fmt.Errorf("dbx: sql.Open: %w", err)
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLife)
	db.SetConnMaxIdleTime(maxIdleTime)

	return db, pingTimeout, nil
}

// Проверяет доступность БД с таймаутом
func Ping(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := db.PingContext(cctx); err != nil {
		return fmt.Errorf("dbx: ping: %w", err)
	}
	return nil
}

// Выполняет fn в транзакции
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	if err != nil {
		return fmt.Errorf("dbx: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("dbx: commit: %w", err)
	}
	return nil
}
