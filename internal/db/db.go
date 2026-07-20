package db

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// PoolConfig limits how the process holds MySQL connections.
// Zero values leave the corresponding database/sql default unchanged
// (except MaxIdleConns is clamped to MaxOpenConns when both are set).
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime   time.Duration
}

func Open(dsn string, pool PoolConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	applyPool(db, pool)
	return db, nil
}

func applyPool(db *sql.DB, pool PoolConfig) {
	if pool.MaxOpenConns > 0 {
		db.SetMaxOpenConns(pool.MaxOpenConns)
	}

	maxIdle := pool.MaxIdleConns
	if maxIdle > 0 {
		if pool.MaxOpenConns > 0 && maxIdle > pool.MaxOpenConns {
			maxIdle = pool.MaxOpenConns
		}
		db.SetMaxIdleConns(maxIdle)
	}

	if pool.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
	}
	if pool.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	}
}
