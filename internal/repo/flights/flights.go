package flights

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/gofrs/uuid/v5"
	"github.com/lib/pq"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"

	svcflights "skyrouter/internal/service/flights"
	"skyrouter/models"
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

func (r *FlightRepo) GetByID(ctx context.Context, id string) (*svcflights.Flight, error) {
	uid, err := uuid.FromString(id)
	if err != nil {
		return nil, sql.ErrNoRows
	}

	// 1. Fetch the flight row.
	mf, err := models.FindFlight(ctx, r.db, uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	// 2. Fetch the latest route version.
	rv, err := models.FlightRouteVersions.Query(
		sm.Where(psql.Quote("flight_route_versions", "flight_id").EQ(psql.Arg(uid))),
		sm.OrderBy(psql.Quote("flight_route_versions", "version")).Desc(),
		sm.Limit(1),
	).One(ctx, r.db)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// 3. Fetch route elements for that version (empty slice if no version).
	var elements []*models.FlightRouteElement
	if rv != nil {
		elements, err = models.FlightRouteElements.Query(
			sm.Where(psql.Quote("flight_route_elements", "route_version_id").EQ(psql.Arg(rv.ID))),
			sm.OrderBy(psql.Quote("flight_route_elements", "seq_num")),
		).All(ctx, r.db)
		if err != nil {
			return nil, err
		}
	}

	// 4. Bulk-fetch waypoints for lat/lon enrichment.
	wpMap := map[string]struct{ lat, lon float64 }{}
	if len(elements) > 0 {
		names := make([]string, 0, len(elements))
		seen := map[string]bool{}
		for _, el := range elements {
			if !seen[el.WaypointName] {
				names = append(names, el.WaypointName)
				seen[el.WaypointName] = true
			}
		}
		wpRows, err := r.db.QueryContext(ctx,
			`SELECT name, latitude, longitude FROM waypoints WHERE name = ANY($1)`,
			pq.Array(names),
		)
		if err != nil {
			return nil, err
		}
		defer wpRows.Close()
		for wpRows.Next() {
			var name string
			var lat, lon float64
			if err := wpRows.Scan(&name, &lat, &lon); err != nil {
				return nil, err
			}
			wpMap[name] = struct{ lat, lon float64 }{lat, lon}
		}
		if err := wpRows.Err(); err != nil {
			return nil, err
		}
	}

	// 5. Map to service model.
	f := &svcflights.Flight{
		ID:                   mf.ID.String(),
		Callsign:             mf.Callsign,
		DepartureAerodrome:   mf.DepartureAerodrome,
		DateOfFlight:         mf.DateOfFlight,
		DestinationAerodrome: mf.DestinationAerodrome,
		Route:                make([]svcflights.RouteElement, 0, len(elements)),
	}
	if mf.FlightType.IsValue() {
		v := mf.FlightType.MustGet()
		f.FlightType = &v
	}
	if mf.Operator.IsValue() {
		v := mf.Operator.MustGet()
		f.Operator = &v
	}
	if mf.AircraftType.IsValue() {
		v := mf.AircraftType.MustGet()
		f.AircraftType = &v
	}
	if mf.AircraftRegistration.IsValue() {
		v := mf.AircraftRegistration.MustGet()
		f.AircraftRegistration = &v
	}
	if mf.ScheduledDepartureAt.IsValue() {
		v := mf.ScheduledDepartureAt.MustGet()
		f.ScheduledDepartureAt = &v
	}
	if mf.ScheduledArrivalAt.IsValue() {
		v := mf.ScheduledArrivalAt.MustGet()
		f.ScheduledArrivalAt = &v
	}

	for _, el := range elements {
		re := svcflights.RouteElement{
			SeqNum:       int(el.SeqNum),
			WaypointName: el.WaypointName,
		}
		if el.Airway.IsValue() {
			v := el.Airway.MustGet()
			re.Airway = &v
		}
		if wp, ok := wpMap[el.WaypointName]; ok {
			re.Latitude = &wp.lat
			re.Longitude = &wp.lon
		}
		f.Route = append(f.Route, re)
	}

	return f, nil
}

func (r *FlightRepo) List(ctx context.Context, filter svcflights.ListFlightsFilter) ([]svcflights.Flight, error) {
	conds := []string{}
	args := []any{}
	n := 1

	if filter.Callsign != "" {
		conds = append(conds, fmt.Sprintf("f.callsign ILIKE $%d", n))
		args = append(args, "%"+filter.Callsign+"%")
		n++
	}
	if filter.DepartureAerodrome != "" {
		conds = append(conds, fmt.Sprintf("f.departure_aerodrome = $%d", n))
		args = append(args, filter.DepartureAerodrome)
		n++
	}
	if filter.DestinationAerodrome != "" {
		conds = append(conds, fmt.Sprintf("f.destination_aerodrome = $%d", n))
		args = append(args, filter.DestinationAerodrome)
		n++
	}
	if filter.Operator != "" {
		conds = append(conds, fmt.Sprintf("f.operator = $%d", n))
		args = append(args, filter.Operator)
		n++
	}
	if filter.DateFrom != nil {
		conds = append(conds, fmt.Sprintf("f.date_of_flight >= $%d", n))
		args = append(args, *filter.DateFrom)
		n++
	}
	if filter.DateTo != nil {
		conds = append(conds, fmt.Sprintf("f.date_of_flight <= $%d", n))
		args = append(args, *filter.DateTo)
		n++
	}
	_ = n

	// LATERAL subquery picks the latest route version per flight.
	// Each flight row is repeated once per route element (or once with NULL cols if no route).
	query := `
SELECT
	f.id, f.callsign, f.flight_type, f.operator, f.aircraft_type, f.aircraft_registration,
	f.departure_aerodrome, f.date_of_flight, f.scheduled_departure_at,
	f.destination_aerodrome, f.scheduled_arrival_at,
	fre.seq_num, fre.waypoint_name, fre.airway, w.latitude, w.longitude
FROM flights f
LEFT JOIN LATERAL (
	SELECT id FROM flight_route_versions
	WHERE flight_id = f.id
	ORDER BY version DESC
	LIMIT 1
) lv ON true
LEFT JOIN flight_route_elements fre ON fre.route_version_id = lv.id
LEFT JOIN waypoints w ON w.name = fre.waypoint_name`

	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY f.date_of_flight DESC, f.scheduled_departure_at DESC NULLS LAST, fre.seq_num ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// index tracks insertion order so flights stay sorted as returned by the query.
	result := []svcflights.Flight{}
	index := map[string]int{}

	for rows.Next() {
		var (
			id                   string
			callsign             string
			flightType           sql.NullString
			operator             sql.NullString
			aircraftType         sql.NullString
			aircraftRegistration sql.NullString
			departureAerodrome   string
			dateOfFlight         sql.NullTime
			scheduledDepartureAt sql.NullTime
			destinationAerodrome string
			scheduledArrivalAt   sql.NullTime
			seqNum               sql.NullInt64
			waypointName         sql.NullString
			airway               sql.NullString
			latitude             sql.NullFloat64
			longitude            sql.NullFloat64
		)
		if err := rows.Scan(
			&id, &callsign, &flightType, &operator, &aircraftType, &aircraftRegistration,
			&departureAerodrome, &dateOfFlight, &scheduledDepartureAt,
			&destinationAerodrome, &scheduledArrivalAt,
			&seqNum, &waypointName, &airway, &latitude, &longitude,
		); err != nil {
			return nil, err
		}

		idx, exists := index[id]
		if !exists {
			f := svcflights.Flight{
				ID:                   id,
				Callsign:             callsign,
				DepartureAerodrome:   departureAerodrome,
				DestinationAerodrome: destinationAerodrome,
				Route:                []svcflights.RouteElement{},
			}
			if dateOfFlight.Valid {
				f.DateOfFlight = dateOfFlight.Time
			}
			if flightType.Valid {
				f.FlightType = &flightType.String
			}
			if operator.Valid {
				f.Operator = &operator.String
			}
			if aircraftType.Valid {
				f.AircraftType = &aircraftType.String
			}
			if aircraftRegistration.Valid {
				f.AircraftRegistration = &aircraftRegistration.String
			}
			if scheduledDepartureAt.Valid {
				f.ScheduledDepartureAt = &scheduledDepartureAt.Time
			}
			if scheduledArrivalAt.Valid {
				f.ScheduledArrivalAt = &scheduledArrivalAt.Time
			}
			result = append(result, f)
			idx = len(result) - 1
			index[id] = idx
		}

		if seqNum.Valid {
			el := svcflights.RouteElement{
				SeqNum:       int(seqNum.Int64),
				WaypointName: waypointName.String,
			}
			if airway.Valid {
				el.Airway = &airway.String
			}
			if latitude.Valid {
				el.Latitude = &latitude.Float64
			}
			if longitude.Valid {
				el.Longitude = &longitude.Float64
			}
			result[idx].Route = append(result[idx].Route, el)
		}
	}
	return result, rows.Err()
}
