package flights

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gofrs/uuid/v5"
)

type FlightRepo struct {
	db Executor
}

func NewFlightRepo(db Executor) *FlightRepo {
	return &FlightRepo{db: db}
}

func (r *FlightRepo) UpsertFlights(ctx context.Context, inputs []UpsertFlightInput) error {
	for i, input := range inputs {
		if err := r.upsertOne(ctx, input); err != nil {
			return fmt.Errorf("flight %d (%s): %w", i, input.SourceID, err)
		}
	}
	return nil
}

// upsertOne inserts or updates a single flight.
// If the source_id already exists with the same last_updated_at, it is skipped.
// If last_updated_at differs (or it is a new row), the flight fields are updated
// and a new route version with its elements is inserted.
func (r *FlightRepo) upsertOne(ctx context.Context, in UpsertFlightInput) error {
	// Upsert the flight row. The WHERE clause skips the update when
	// last_updated_at has not changed, causing RETURNING to return nothing.
	const flightSQL = `
INSERT INTO flights (
	source_id, message_type, callsign,
	flight_type, operator, src, remark,
	aircraft_type, wake_turbulence, aircraft_registration, aircraft_address,
	departure_aerodrome, date_of_flight, estimated_off_block_time, scheduled_departure_at,
	destination_aerodrome, alternate_aerodromes, scheduled_arrival_at,
	alternate_enroute, mode_a_code,
	gufi, gufi_originator,
	reception_time, last_updated_at
) VALUES (
	$1,$2,$3,
	$4,$5,$6,$7,
	$8,$9,$10,$11,
	$12,$13,$14,$15,
	$16,$17,$18,
	$19,$20,
	$21,$22,
	$23,$24
)
ON CONFLICT (source_id) DO UPDATE SET
	message_type           = EXCLUDED.message_type,
	callsign               = EXCLUDED.callsign,
	flight_type            = EXCLUDED.flight_type,
	operator               = EXCLUDED.operator,
	src                    = EXCLUDED.src,
	remark                 = EXCLUDED.remark,
	aircraft_type          = EXCLUDED.aircraft_type,
	wake_turbulence        = EXCLUDED.wake_turbulence,
	aircraft_registration  = EXCLUDED.aircraft_registration,
	aircraft_address       = EXCLUDED.aircraft_address,
	departure_aerodrome    = EXCLUDED.departure_aerodrome,
	date_of_flight         = EXCLUDED.date_of_flight,
	estimated_off_block_time = EXCLUDED.estimated_off_block_time,
	scheduled_departure_at = EXCLUDED.scheduled_departure_at,
	destination_aerodrome  = EXCLUDED.destination_aerodrome,
	alternate_aerodromes   = EXCLUDED.alternate_aerodromes,
	scheduled_arrival_at   = EXCLUDED.scheduled_arrival_at,
	alternate_enroute      = EXCLUDED.alternate_enroute,
	mode_a_code            = EXCLUDED.mode_a_code,
	gufi                   = EXCLUDED.gufi,
	gufi_originator        = EXCLUDED.gufi_originator,
	reception_time         = EXCLUDED.reception_time,
	last_updated_at        = EXCLUDED.last_updated_at
WHERE flights.last_updated_at IS DISTINCT FROM EXCLUDED.last_updated_at
RETURNING id`

	rows, err := r.db.QueryContext(ctx, flightSQL,
		in.SourceID, in.MessageType, in.Callsign,
		in.FlightType, in.Operator, in.SRC, in.Remark,
		in.AircraftType, in.WakeTurbulence, in.AircraftRegistration, in.AircraftAddress,
		in.DepartureAerodrome, in.DateOfFlight, in.EstimatedOffBlockTime, in.ScheduledDepartureAt,
		in.DestinationAerodrome, in.AlternateAerodromes, in.ScheduledArrivalAt,
		in.AlternateEnroute, in.ModeACode,
		in.Gufi, in.GufiOriginator,
		in.ReceptionTime, in.LastUpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert flight: %w", err)
	}
	defer rows.Close()

	// If RETURNING produced no row, last_updated_at was unchanged — nothing to do.
	if !rows.Next() {
		return rows.Err()
	}

	var flightID uuid.UUID
	if err := rows.Scan(&flightID); err != nil {
		return fmt.Errorf("scan flight id: %w", err)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	return r.insertRouteVersion(ctx, flightID, in.Route)
}

func (r *FlightRepo) insertRouteVersion(ctx context.Context, flightID uuid.UUID, route RouteInput) error {
	// Determine the next version number for this flight.
	var nextVersion int
	row, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM flight_route_versions WHERE flight_id = $1`,
		flightID,
	)
	if err != nil {
		return fmt.Errorf("query max version: %w", err)
	}
	defer row.Close()
	if row.Next() {
		if err := row.Scan(&nextVersion); err != nil {
			return fmt.Errorf("scan max version: %w", err)
		}
	}
	if err := row.Close(); err != nil {
		return err
	}

	// Insert the route version.
	versionRows, err := r.db.QueryContext(ctx, `
INSERT INTO flight_route_versions
	(flight_id, version, flight_rules, cruising_speed, cruising_level, route_text, total_elapsed_time, fir_estimates)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING id`,
		flightID, nextVersion,
		route.FlightRules, route.CruisingSpeed, route.CruisingLevel,
		route.RouteText, route.TotalElapsedTime, route.FirEstimates,
	)
	if err != nil {
		return fmt.Errorf("insert route version: %w", err)
	}
	defer versionRows.Close()

	if !versionRows.Next() {
		return fmt.Errorf("insert route version returned no id")
	}
	var versionID uuid.UUID
	if err := versionRows.Scan(&versionID); err != nil {
		return fmt.Errorf("scan route version id: %w", err)
	}
	if err := versionRows.Close(); err != nil {
		return err
	}

	if len(route.Elements) == 0 {
		return nil
	}

	return r.insertRouteElements(ctx, versionID, route.Elements)
}

func (r *FlightRepo) insertRouteElements(ctx context.Context, versionID uuid.UUID, elements []RouteElementInput) error {
	const chunkSize = 500
	for i := 0; i < len(elements); i += chunkSize {
		end := i + chunkSize
		if end > len(elements) {
			end = len(elements)
		}
		if err := r.insertRouteElementsChunk(ctx, versionID, elements[i:end]); err != nil {
			return fmt.Errorf("elements chunk at %d: %w", i, err)
		}
	}
	return nil
}

func (r *FlightRepo) insertRouteElementsChunk(ctx context.Context, versionID uuid.UUID, elements []RouteElementInput) error {
	// Build: INSERT INTO flight_route_elements (...) VALUES ($1,$2,...),(...)
	query := "INSERT INTO flight_route_elements (route_version_id, seq_num, waypoint_name, airway, airway_type, change_speed, change_level) VALUES "
	args := make([]any, 0, len(elements)*7)
	for i, el := range elements {
		if i > 0 {
			query += ","
		}
		n := i*7 + 1
		query += fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d)", n, n+1, n+2, n+3, n+4, n+5, n+6)
		args = append(args, versionID, el.SeqNum, el.WaypointName, el.Airway, el.AirwayType, el.ChangeSpeed, el.ChangeLevel)
	}

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("insert elements: %w", err)
	}
	return nil
}

// nullableTime is a helper to scan a nullable time into sql.NullTime for use in QueryContext.
// Not used directly but kept for reference if raw scanning is needed elsewhere.
var _ = sql.NullTime{}
