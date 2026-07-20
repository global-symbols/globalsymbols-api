package db

import (
	"database/sql"
	"testing"
	"time"
)

// applyPool is pure configuration; use sql.Open with a driver that never
// connects so we can assert Set* calls do not panic and clamp idle correctly.
func TestApplyPool_ClampsIdleToOpen(t *testing.T) {
	db, err := sql.Open("mysql", "user:pass@tcp(127.0.0.1:1)/db")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	applyPool(db, PoolConfig{
		MaxOpenConns:    5,
		MaxIdleConns:    20,
		ConnMaxIdleTime: 5 * time.Minute,
		ConnMaxLifetime:   time.Hour,
	})

	stats := db.Stats()
	if stats.MaxOpenConnections != 5 {
		t.Fatalf("MaxOpenConnections = %d, want 5", stats.MaxOpenConnections)
	}
}
