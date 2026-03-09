package waypoints_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"skyrouter/internal/service/waypoints"
	"skyrouter/internal/service/waypoints/mocks"
)

func TestWaypointService_ListWaypoints(t *testing.T) {
	tests := []struct {
		name           string
		expectedResult []waypoints.Waypoint
		expectedError  error
		wantLen        int
		actualError    bool
	}{
		{
			name: "returns all waypoints",
			expectedResult: []waypoints.Waypoint{
				{ID: 1, Name: "Alpha", Latitude: 10.0, Longitude: 20.0},
				{ID: 2, Name: "Bravo", Latitude: 30.0, Longitude: 40.0},
			},
			wantLen: 2,
		},
		{
			name:           "returns empty slice",
			expectedResult: []waypoints.Waypoint{},
			wantLen:        0,
		},
		{
			name:          "propagates repo error",
			expectedError: errors.New("db error"),
			actualError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockRepo := mocks.NewMockWaypointRepository(t)
			filter := waypoints.ListWaypointsFilter{}
			mockRepo.EXPECT().List(ctx, filter).Return(tt.expectedResult, tt.expectedError)

			svc := waypoints.NewWaypointService(mockRepo)
			result, err := svc.ListWaypoints(ctx, filter)

			if tt.actualError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.wantLen {
				t.Errorf("expected %d waypoints, got %d", tt.wantLen, len(result))
			}
		})
	}
}

func TestWaypointService_GetWaypoint(t *testing.T) {
	tests := []struct {
		name           string
		id             int32
		expectedResult *waypoints.Waypoint
		expectedError  error
		actualError    error
	}{
		{
			name:           "returns waypoint by id",
			id:             1,
			expectedResult: &waypoints.Waypoint{ID: 1, Name: "Alpha", Latitude: 10.0, Longitude: 20.0},
		},
		{
			name:          "returns ErrNotFound when repo returns sql.ErrNoRows",
			id:            -1,
			expectedError: sql.ErrNoRows,
			actualError:   waypoints.ErrNotFound,
		},
		{
			name:          "propagates unexpected repo error",
			id:            1,
			expectedError: errors.New("db error"),
			actualError:   errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockRepo := mocks.NewMockWaypointRepository(t)
			mockRepo.EXPECT().GetByID(ctx, tt.id).Return(tt.expectedResult, tt.expectedError)

			svc := waypoints.NewWaypointService(mockRepo)
			wp, err := svc.GetWaypoint(ctx, tt.id)

			if tt.actualError != nil {
				if !errors.Is(err, tt.actualError) && err.Error() != tt.actualError.Error() {
					t.Errorf("expected error %v, got %v", tt.actualError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if wp.ID != tt.expectedResult.ID {
				t.Errorf("expected ID %d, got %d", tt.expectedResult.ID, wp.ID)
			}
			if wp.Name != tt.expectedResult.Name {
				t.Errorf("expected Name %q, got %q", tt.expectedResult.Name, wp.Name)
			}
		})
	}
}
