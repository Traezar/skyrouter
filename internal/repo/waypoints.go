package repo

import (
	"context"

	"github.com/stephenafamo/bob"

	"skyrouter/models"
)

type WaypointRepo struct {
	db bob.Executor
}

func NewWaypointRepo(db bob.Executor) *WaypointRepo {
	return &WaypointRepo{db: db}
}

func (r *WaypointRepo) List(ctx context.Context) (models.WaypointSlice, error) {
	return models.Waypoints.Query().All(ctx, r.db)
}

func (r *WaypointRepo) GetByID(ctx context.Context, id int32) (*models.Waypoint, error) {
	return models.FindWaypoint(ctx, r.db, id)
}
