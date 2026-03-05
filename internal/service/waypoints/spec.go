package waypoints

import (
	"context"

	"skyrouter/models"
)

//go:generate go tool mockery --name=WaypointRepository
type WaypointRepository interface {
	List(ctx context.Context) (models.WaypointSlice, error)
	GetByID(ctx context.Context, id int32) (*models.Waypoint, error)
}
