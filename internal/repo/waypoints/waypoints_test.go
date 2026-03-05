package waypoints_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	bobpgx "github.com/stephenafamo/bob/drivers/pgx"

	"skyrouter/internal/repo/waypoints"
	svcwaypoints "skyrouter/internal/service/waypoints"
	"skyrouter/models"
)

var testExec waypoints.Executor

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	host := os.Getenv("DB_HOST")
	if host == "" {
		return m.Run()
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host,
		envOr("DB_PORT", "5432"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		envOr("DB_SSLMODE", "disable"),
	)

	exec, err := bobpgx.New(context.Background(), dsn)
	if err != nil {
		return m.Run()
	}
	defer exec.Close()

	testExec = exec
	return m.Run()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestWaypointRepo_List(t *testing.T) {
	if testExec == nil {
		t.Skip("requires database: set DB_HOST to run")
	}

	r := waypoints.NewWaypointRepo(testExec)
	ctx := context.Background()

	// Insert two known waypoints for this test.
	r.BulkUpsert(ctx, []svcwaypoints.UpsertWaypointInput{
		{Name: "TST_LST1", Latitude: 1.0, Longitude: 2.0},
		{Name: "TST_LST2", Latitude: 1.0, Longitude: 2.0},
	})
	t.Cleanup(func() {
		testExec.ExecContext(ctx, "DELETE FROM waypoints WHERE name LIKE 'TST_LST%'")
	})

	result, err := r.List(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) < 2 {
		t.Errorf("expected at least 2 waypoints, got %d", len(result))
	}
}

func TestWaypointRepo_GetByID(t *testing.T) {
	if testExec == nil {
		t.Skip("requires database: set DB_HOST to run")
	}

	r := waypoints.NewWaypointRepo(testExec)
	ctx := context.Background()
	const name = "TST_GBI"

	t.Cleanup(func() {
		testExec.ExecContext(ctx, "DELETE FROM waypoints WHERE name = $1", name)
	})

	if err := r.BulkUpsert(ctx, []svcwaypoints.UpsertWaypointInput{
		{Name: name, Latitude: 30.0, Longitude: 40.0},
	}); err != nil {
		t.Fatalf("setup: failed to insert waypoint: %v", err)
	}

	all, err := r.List(ctx)
	if err != nil {
		t.Fatalf("setup: failed to list waypoints: %v", err)
	}
	var inserted *models.Waypoint
	for _, w := range all {
		if w.Name == name {
			inserted = w
			break
		}
	}
	if inserted == nil {
		t.Fatal("setup: inserted waypoint not found in list")
	}

	t.Run("returns waypoint by id", func(t *testing.T) {
		wp, err := r.GetByID(ctx, inserted.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if wp.ID != inserted.ID {
			t.Errorf("expected ID %d, got %d", inserted.ID, wp.ID)
		}
		if wp.Name != name {
			t.Errorf("expected Name %q, got %q", name, wp.Name)
		}
	})

	t.Run("returns error for non-existent id", func(t *testing.T) {
		_, err := r.GetByID(ctx, -1)
		if err == nil {
			t.Fatal("expected error for non-existent id, got nil")
		}
	})
}
