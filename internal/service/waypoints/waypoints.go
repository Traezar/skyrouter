package waypoints

import (
	"context"
	"database/sql"
	"errors"
)

type WaypointService struct {
	repo WaypointRepository
}

func NewWaypointService(repo WaypointRepository) *WaypointService {
	return &WaypointService{repo: repo}
}

func (s *WaypointService) ListWaypoints(ctx context.Context, filter ListWaypointsFilter) ([]Waypoint, error) {
	return s.repo.List(ctx, filter)
}

func (s *WaypointService) GetWaypoint(ctx context.Context, id int32) (*Waypoint, error) {
	wp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return wp, nil
}
