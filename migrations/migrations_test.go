package migrations_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"github.com/sambady/heretous/internal/database"
	"github.com/sambady/heretous/internal/testutil"
	"github.com/sambady/heretous/migrations"
)

func TestConcurrentInstancesApplyMigrationsOnce(t *testing.T) {
	db, cfg := testutil.Database(t)
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	providers := make([]*goose.Provider, 3)
	for i := range providers {
		// Separate pools and providers simulate independently started processes.
		instanceDB, err := database.Open(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = instanceDB.Close() })
		providers[i], err = migrations.New(instanceDB)
		if err != nil {
			t.Fatal(err)
		}
	}
	type result struct {
		count int
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, len(providers))
	for _, provider := range providers {
		go func() {
			<-start
			applied, err := provider.Up(ctx)
			results <- result{count: len(applied), err: err}
		}()
	}
	close(start)
	total := 0
	for range providers {
		result := <-results
		if result.err != nil {
			t.Errorf("concurrent migration failed: %v", result.err)
		}
		total += result.count
	}
	if total != 1 {
		t.Fatalf("migration applied %d times across instances, want 1", total)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id = 1 AND is_applied").Scan(&count); err != nil || count != 1 {
		t.Fatalf("applied version records=%d err=%v, want 1 record", count, err)
	}
}

func TestMigrationLockWaitHonorsDeadline(t *testing.T) {
	db, _ := testutil.Database(t)
	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		t.Fatal(err)
	}
	if err := locker.SessionLock(ctx, conn); err != nil {
		t.Fatal(err)
	}
	held := true
	defer func() {
		if held {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := locker.SessionUnlock(cleanupCtx, conn); err != nil {
				t.Errorf("release migration lock: %v", err)
			}
		}
	}()
	provider, err := migrations.New(db)
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	_, err = provider.Up(waitCtx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline while waiting for migration lock, got %v", err)
	}
	var exists bool
	if err := db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'app')").Scan(&exists); err != nil || exists {
		t.Fatalf("migration ran while lock was held: exists=%v err=%v", exists, err)
	}
	if err := locker.SessionUnlock(ctx, conn); err != nil {
		t.Fatal(err)
	}
	held = false
	applied, err := provider.Up(ctx)
	if err != nil || len(applied) != 1 {
		t.Fatalf("migration after releasing lock: applied=%d err=%v", len(applied), err)
	}
}

func TestMigrationsUpRepeatDownAndReapply(t *testing.T) {
	db, _ := testutil.Database(t)
	provider, err := migrations.New(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	for attempt := 0; attempt < 2; attempt++ {
		applied, err := provider.Up(ctx)
		if err != nil || len(applied) != 1 {
			t.Fatalf("up: applied=%d err=%v", len(applied), err)
		}
		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'app')").Scan(&exists); err != nil || !exists {
			t.Fatalf("schema not created: exists=%v err=%v", exists, err)
		}
		version, err := provider.GetDBVersion(ctx)
		if err != nil || version != 1 {
			t.Fatalf("version=%d err=%v", version, err)
		}
		applied, err = provider.Up(ctx)
		if err != nil || len(applied) != 0 {
			t.Fatalf("repeat up: applied=%d err=%v", len(applied), err)
		}
		if _, err := provider.Down(ctx); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'app')").Scan(&exists); err != nil || exists {
			t.Fatalf("schema not removed: exists=%v err=%v", exists, err)
		}
		version, err = provider.GetDBVersion(ctx)
		if err != nil || version != 0 {
			t.Fatalf("version after down=%d err=%v", version, err)
		}
	}
}
