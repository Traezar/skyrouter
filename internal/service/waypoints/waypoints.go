package waypoints

import (
	"context"
	"database/sql"
	"errors"

	"skyrouter/models"
)

type WaypointService struct {
	repo WaypointRepository
}

func NewWaypointService(repo WaypointRepository) *WaypointService {
	return &WaypointService{repo: repo}
}

func (s *WaypointService) ListWaypoints(ctx context.Context) (models.WaypointSlice, error) {
	return s.repo.List(ctx)
}

func (s *WaypointService) GetWaypoint(ctx context.Context, id int32) (*models.Waypoint, error) {
	wp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return wp, nil
}
