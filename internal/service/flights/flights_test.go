package flights_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"skyrouter/internal/service/flights"
	"skyrouter/internal/service/flights/mocks"
)

var (
	flightFixture = &flights.Flight{
		ID:                   "550e8400-e29b-41d4-a716-446655440000",
		Callsign:             "SQ123",
		DepartureAerodrome:   "WSSS",
		DestinationAerodrome: "WMKK",
		DateOfFlight:         time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		Route:                []flights.RouteElement{},
	}
	flightFixture2 = &flights.Flight{
		ID:                   "550e8400-e29b-41d4-a716-446655440001",
		Callsign:             "MH370",
		DepartureAerodrome:   "WMKK",
		DestinationAerodrome: "ZBAA",
		DateOfFlight:         time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC),
		Route:                []flights.RouteElement{},
	}
)

func TestFlightService_ListFlights(t *testing.T) {
	tests := []struct {
		name          string
		filter        flights.ListFlightsFilter
		repoResult    []flights.Flight
		repoErr       error
		wantLen       int
		wantErr       bool
	}{
		{
			name:       "returns all flights",
			filter:     flights.ListFlightsFilter{},
			repoResult: []flights.Flight{*flightFixture, *flightFixture2},
			wantLen:    2,
		},
		{
			name:       "filters by callsign",
			filter:     flights.ListFlightsFilter{Callsign: "SQ"},
			repoResult: []flights.Flight{*flightFixture},
			wantLen:    1,
		},
		{
			name:       "returns empty slice",
			filter:     flights.ListFlightsFilter{},
			repoResult: []flights.Flight{},
			wantLen:    0,
		},
		{
			name:    "propagates repo error",
			filter:  flights.ListFlightsFilter{},
			repoErr: errors.New("db error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockRepo := mocks.NewMockFlightRepository(t)
			mockRepo.EXPECT().List(ctx, tt.filter).Return(tt.repoResult, tt.repoErr)

			svc := flights.NewFlightService(mockRepo)
			result, err := svc.ListFlights(ctx, tt.filter)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Errorf("got nil, want non-nil slice (would encode as JSON null)")
			}
			if len(result) != tt.wantLen {
				t.Errorf("expected %d flights, got %d", tt.wantLen, len(result))
			}
		})
	}
}

func TestFlightService_GetFlight(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		repoResult *flights.Flight
		repoErr    error
		wantFlight *flights.Flight
		wantErr    error
	}{
		{
			name:       "returns flight by id",
			id:         flightFixture.ID,
			repoResult: flightFixture,
			wantFlight: flightFixture,
		},
		{
			name:    "not found returns sql.ErrNoRows",
			id:      "00000000-0000-0000-0000-000000000000",
			repoErr: sql.ErrNoRows,
			wantErr: sql.ErrNoRows,
		},
		{
			name:    "propagates unexpected repo error",
			id:      flightFixture.ID,
			repoErr: errors.New("db error"),
			wantErr: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockRepo := mocks.NewMockFlightRepository(t)
			mockRepo.EXPECT().GetByID(ctx, tt.id).Return(tt.repoResult, tt.repoErr)

			svc := flights.NewFlightService(mockRepo)
			result, err := svc.GetFlight(ctx, tt.id)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID != tt.wantFlight.ID {
				t.Errorf("expected ID %q, got %q", tt.wantFlight.ID, result.ID)
			}
			if result.Callsign != tt.wantFlight.Callsign {
				t.Errorf("expected Callsign %q, got %q", tt.wantFlight.Callsign, result.Callsign)
			}
		})
	}
}
