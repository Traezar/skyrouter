package flights

import "context"

type FlightService struct {
	repo FlightRepository
}

func NewFlightService(repo FlightRepository) *FlightService {
	return &FlightService{repo: repo}
}

func (s *FlightService) ListFlights(ctx context.Context, filter ListFlightsFilter) ([]Flight, error) {
	return s.repo.List(ctx, filter)
}

func (s *FlightService) GetFlight(ctx context.Context, id string) (*Flight, error) {
	return s.repo.GetByID(ctx, id)
}
