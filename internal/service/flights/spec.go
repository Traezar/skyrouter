package flights

import (
	"context"
	"time"
)

// RouteElement is a single waypoint in a flight's route, ordered by sequence.
type RouteElement struct {
	SeqNum       int      `json:"seqNum"`
	WaypointName string   `json:"waypointName"`
	Airway       *string  `json:"airway,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
}

// Flight is the read model returned by the service.
type Flight struct {
	ID                   string         `json:"id"`
	Callsign             string         `json:"callsign"`
	FlightType           *string        `json:"flightType,omitempty"`
	Operator             *string        `json:"operator,omitempty"`
	AircraftType         *string        `json:"aircraftType,omitempty"`
	AircraftRegistration *string        `json:"aircraftRegistration,omitempty"`
	DepartureAerodrome   string         `json:"departureAerodrome"`
	DepartureLat         *float64       `json:"departureLat,omitempty"`
	DepartureLon         *float64       `json:"departureLon,omitempty"`
	DateOfFlight         time.Time      `json:"dateOfFlight"`
	ScheduledDepartureAt *time.Time     `json:"scheduledDepartureAt,omitempty"`
	DestinationAerodrome string         `json:"destinationAerodrome"`
	DestinationLat       *float64       `json:"destinationLat,omitempty"`
	DestinationLon       *float64       `json:"destinationLon,omitempty"`
	ScheduledArrivalAt   *time.Time     `json:"scheduledArrivalAt,omitempty"`
	Route                []RouteElement `json:"route"`
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
	GetByID(ctx context.Context, id string) (*Flight, error)
}
