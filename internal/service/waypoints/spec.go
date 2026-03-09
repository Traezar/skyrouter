package waypoints

import "context"

type UpsertWaypointInput struct {
	Name      string
	Latitude  float64
	Longitude float64
	Grid      bool
}

type ListWaypointsFilter struct {
	Grid *bool
}

// Waypoint is the read model returned by the service.
type Waypoint struct {
	ID        int32   `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Grid      bool    `json:"grid"`
}

//go:generate go tool mockery --name=WaypointRepository
type WaypointRepository interface {
	List(ctx context.Context, filter ListWaypointsFilter) ([]Waypoint, error)
	GetByID(ctx context.Context, id int32) (*Waypoint, error)
}
