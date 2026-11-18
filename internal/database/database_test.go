package database_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sambady/heretous/internal/config"
	"github.com/sambady/heretous/internal/database"
	"github.com/sambady/heretous/internal/testutil"
)

func TestOpenPostgreSQL(t *testing.T) {
	db, cfg := testutil.Database(t)
	var name string
	if err := db.QueryRowContext(t.Context(), "SELECT current_database()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "heretous_test_") || db.Stats().MaxOpenConnections != cfg.MaxOpenConnections {
		t.Fatal("unexpected database or pool configuration")
	}
}

func TestOpenInvalidURLDoesNotExposePassword(t *testing.T) {
	cfg := config.Default().Database
	cfg.URL = "postgres://user:secret-password@localhost:invalid/db"
	_, err := database.Open(t.Context(), cfg)
	if err == nil || strings.Contains(err.Error(), "secret-password") {
		t.Fatalf("expected safe connection error, got %v", err)
	}
}

func TestOpenCanceledContext(t *testing.T) {
	cfg := config.Default().Database
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	db, err := database.Open(ctx, cfg)
	if err == nil {
		db.Close()
		t.Fatal("expected canceled connection to fail")
	}
}
