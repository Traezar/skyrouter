package flights_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aarondl/opt/null"
	bobpgx "github.com/stephenafamo/bob/drivers/pgx"

	"skyrouter/internal/repo/flights"
	svcflights "skyrouter/internal/service/flights"
)

var testExec flights.Executor

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

// insertTestFlight inserts a minimal flight and returns its UUID string.
func insertTestFlight(ctx context.Context, t *testing.T, callsign string) string {
	t.Helper()
	r := flights.NewFlightRepo(testExec)
	err := r.UpsertFlights(ctx, []flights.UpsertFlightInput{{
		SourceID:             "TEST-" + callsign,
		MessageType:          "FPL",
		Callsign:             callsign,
		DepartureAerodrome:   "WSSS",
		DateOfFlight:         time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		DestinationAerodrome: "WMKK",
		Operator:             null.From("TEST_OP"),
	}})
	if err != nil {
		t.Fatalf("setup: UpsertFlights: %v", err)
	}
	all, err := r.List(ctx, svcflights.ListFlightsFilter{Callsign: callsign})
	if err != nil || len(all) == 0 {
		t.Fatalf("setup: could not find inserted flight: %v", err)
	}
	return all[0].ID
}

func TestFlightRepo_GetByID(t *testing.T) {
	if testExec == nil {
		t.Skip("requires database: set DB_HOST to run")
	}

	ctx := context.Background()
	const callsign = "TST_GBI"

	validID := insertTestFlight(ctx, t, callsign)
	t.Cleanup(func() {
		testExec.ExecContext(ctx, "DELETE FROM flights WHERE source_id = $1", "TEST-"+callsign)
	})

	r := flights.NewFlightRepo(testExec)

	tests := []struct {
		name             string
		id               string // use "<valid>" as a placeholder for the inserted flight's ID
		wantErr          error
		wantCallsign     string
		wantDeparture    string
		wantDestination  string
		wantOperator     string
	}{
		{
			name:            "found",
			id:              "<valid>",
			wantCallsign:    callsign,
			wantDeparture:   "WSSS",
			wantDestination: "WMKK",
			wantOperator:    "TEST_OP",
		},
		{
			name:    "unknown uuid",
			id:      "00000000-0000-0000-0000-000000000000",
			wantErr: sql.ErrNoRows,
		},
		{
			name:    "invalid uuid",
			id:      "not-a-uuid",
			wantErr: sql.ErrNoRows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.id
			if id == "<valid>" {
				id = validID
			}

			got, err := r.GetByID(ctx, id)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("GetByID(%q) error = %v, want %v", id, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetByID(%q) unexpected error: %v", id, err)
			}
			if got.ID != validID {
				t.Errorf("ID = %q, want %q", got.ID, validID)
			}
			if got.Callsign != tt.wantCallsign {
				t.Errorf("Callsign = %q, want %q", got.Callsign, tt.wantCallsign)
			}
			if got.DepartureAerodrome != tt.wantDeparture {
				t.Errorf("DepartureAerodrome = %q, want %q", got.DepartureAerodrome, tt.wantDeparture)
			}
			if got.DestinationAerodrome != tt.wantDestination {
				t.Errorf("DestinationAerodrome = %q, want %q", got.DestinationAerodrome, tt.wantDestination)
			}
			if got.Operator == nil || *got.Operator != tt.wantOperator {
				t.Errorf("Operator = %v, want %q", got.Operator, tt.wantOperator)
			}
			if got.Route == nil {
				t.Errorf("Route = nil, want non-nil slice")
			}
		})
	}
}

func TestFlightRepo_List(t *testing.T) {
	if testExec == nil {
		t.Skip("requires database: set DB_HOST to run")
	}

	ctx := context.Background()
	const prefix = "TST_LST"

	insertTestFlight(ctx, t, prefix+"1")
	insertTestFlight(ctx, t, prefix+"2")
	t.Cleanup(func() {
		testExec.ExecContext(ctx, "DELETE FROM flights WHERE source_id LIKE 'TEST-TST_LST%'")
	})

	r := flights.NewFlightRepo(testExec)

	tests := []struct {
		name          string
		filter        svcflights.ListFlightsFilter
		wantMinLen    int
		wantLen       int // -1 means not checked
		wantCallsigns []string
	}{
		{
			name:       "no filter returns all flights",
			filter:     svcflights.ListFlightsFilter{},
			wantMinLen: 2,
			wantLen:    -1,
		},
		{
			name:          "filter by callsign",
			filter:        svcflights.ListFlightsFilter{Callsign: prefix + "1"},
			wantLen:       1,
			wantCallsigns: []string{prefix + "1"},
		},
		{
			name:       "filter by departure aerodrome",
			filter:     svcflights.ListFlightsFilter{DepartureAerodrome: "WSSS"},
			wantMinLen: 2,
			wantLen:    -1,
		},
		{
			name:    "unmatched filter returns empty",
			filter:  svcflights.ListFlightsFilter{Callsign: "ZZZNOTEXIST"},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("List() unexpected error: %v", err)
			}
			if tt.wantLen >= 0 && len(got) != tt.wantLen {
				t.Errorf("len(List()) = %d, want %d", len(got), tt.wantLen)
			}
			if len(got) < tt.wantMinLen {
				t.Errorf("len(List()) = %d, want >= %d", len(got), tt.wantMinLen)
			}
			for _, wantCS := range tt.wantCallsigns {
				found := false
				for _, f := range got {
					if f.Callsign == wantCS {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("callsign %q not found in results", wantCS)
				}
			}
		})
	}
}
