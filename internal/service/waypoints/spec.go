package waypoints

import (
	"context"

	"skyrouter/models"
)

type UpsertWaypointInput struct {
	Name      string
	Latitude  float64
	Longitude float64
}

//go:generate go tool mockery --name=WaypointRepository
type WaypointRepository interface {
	List(ctx context.Context) (models.WaypointSlice, error)
	GetByID(ctx context.Context, id int32) (*models.Waypoint, error)
	// Upsert(ctx context.Context, input UpsertWaypointInput) (*models.Waypoint, error)
}
