package flights

import (
	"context"
	"database/sql"
	"time"

	"github.com/aarondl/opt/null"
	"github.com/lib/pq"
	"github.com/stephenafamo/scan"
)

// RouteElementInput represents a single element in a flight route.
type RouteElementInput struct {
	SeqNum       int
	WaypointName string
	Airway       null.Val[string]
	AirwayType   null.Val[string]
	ChangeSpeed  null.Val[string]
	ChangeLevel  null.Val[string]
}

// RouteInput represents the route snapshot to attach to a flight version.
type RouteInput struct {
	FlightRules      null.Val[string]
	CruisingSpeed    null.Val[string]
	CruisingLevel    null.Val[string]
	RouteText        null.Val[string]
	TotalElapsedTime null.Val[string]
	FirEstimates     null.Val[pq.StringArray]
	Elements         []RouteElementInput
}

// UpsertFlightInput is the full payload for one flight record.
type UpsertFlightInput struct {
	// identity
	SourceID    string
	MessageType string
	Callsign    string

	// optional flight info
	FlightType null.Val[string]
	Operator   null.Val[string]
	SRC        null.Val[string]
	Remark     null.Val[string]

	// aircraft
	AircraftType         null.Val[string]
	WakeTurbulence       null.Val[string]
	AircraftRegistration null.Val[string]
	AircraftAddress      null.Val[string]

	// departure
	DepartureAerodrome    string
	DateOfFlight          time.Time
	EstimatedOffBlockTime null.Val[time.Time]
	ScheduledDepartureAt  null.Val[time.Time]

	// arrival
	DestinationAerodrome string
	AlternateAerodromes  null.Val[pq.StringArray]
	ScheduledArrivalAt   null.Val[time.Time]

	// enroute
	AlternateEnroute null.Val[string]
	ModeACode        null.Val[string]

	// gufi
	Gufi          null.Val[string]
	GufiOriginator null.Val[string]

	// meta
	ReceptionTime null.Val[time.Time]
	LastUpdatedAt null.Val[time.Time]

	// route
	Route RouteInput
}

// FlightRepository is the interface this package satisfies.
type FlightRepository interface {
	UpsertFlights(ctx context.Context, inputs []UpsertFlightInput) error
}

// Executor is the database interface FlightRepo depends on.
type Executor interface {
	QueryContext(ctx context.Context, query string, args ...any) (scan.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
