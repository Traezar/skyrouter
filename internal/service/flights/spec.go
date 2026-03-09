package flights

import (
	"context"
	"time"
)

// Flight is the read model returned by the service.
type Flight struct {
	ID                   string     `json:"id"`
	Callsign             string     `json:"callsign"`
	FlightType           *string    `json:"flightType,omitempty"`
	Operator             *string    `json:"operator,omitempty"`
	AircraftType         *string    `json:"aircraftType,omitempty"`
	AircraftRegistration *string    `json:"aircraftRegistration,omitempty"`
	DepartureAerodrome   string     `json:"departureAerodrome"`
	DateOfFlight         time.Time  `json:"dateOfFlight"`
	ScheduledDepartureAt *time.Time `json:"scheduledDepartureAt,omitempty"`
	DestinationAerodrome string     `json:"destinationAerodrome"`
	ScheduledArrivalAt   *time.Time `json:"scheduledArrivalAt,omitempty"`
}

// ListFlightsFilter holds optional search parameters for listing flights.
type ListFlightsFilter struct {
	Callsign             string
	DepartureAerodrome   string
	DestinationAerodrome string
	Operator             string
	DateFrom             *time.Time
	DateTo               *time.Time
}

//go:generate go tool mockery --name=FlightRepository
type FlightRepository interface {
	List(ctx context.Context, filter ListFlightsFilter) ([]Flight, error)
}
