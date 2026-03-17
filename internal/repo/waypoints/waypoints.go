package waypoints

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"

	svcwaypoints "skyrouter/internal/service/waypoints"
	"skyrouter/models"
)

type WaypointRepo struct {
	db Executor
}

func NewWaypointRepo(db Executor) *WaypointRepo {
	return &WaypointRepo{db: db}
}

func (r *WaypointRepo) List(ctx context.Context, filter svcwaypoints.ListWaypointsFilter) ([]svcwaypoints.Waypoint, error) {
	var rows models.WaypointSlice
	var err error
	if filter.Grid != nil {
		rows, err = models.Waypoints.Query(
			sm.Where(psql.Quote("waypoints", "grid").EQ(psql.Arg(*filter.Grid))),
		).All(ctx, r.db)
	} else {
		rows, err = models.Waypoints.Query().All(ctx, r.db)
	}
	if err != nil {
		return nil, err
	}
	return toWaypointSlice(rows), nil
}

func (r *WaypointRepo) GetByID(ctx context.Context, id int32) (*svcwaypoints.Waypoint, error) {
	row, err := models.FindWaypoint(ctx, r.db, id)
	if err != nil {
		return nil, err
	}
	w := toWaypoint(row)
	return &w, nil
}

func toWaypoint(m *models.Waypoint) svcwaypoints.Waypoint {
	return svcwaypoints.Waypoint{
		ID:        m.ID,
		Name:      m.Name,
		Latitude:  m.Latitude,
		Longitude: m.Longitude,
		Grid:      m.Grid,
		Airport:   m.Airport,
	}
}

func toWaypointSlice(rows models.WaypointSlice) []svcwaypoints.Waypoint {
	out := make([]svcwaypoints.Waypoint, len(rows))
	for i, m := range rows {
		out[i] = toWaypoint(m)
	}
	return out
}

const bulkChunkSize = 1000

func deduplicateByNameAndLocation(inputs []svcwaypoints.UpsertWaypointInput) []svcwaypoints.UpsertWaypointInput {
	type key struct {
		name    string
		lat, lon int64
	}
	seen := make(map[key]struct{}, len(inputs))
	out := make([]svcwaypoints.UpsertWaypointInput, 0, len(inputs))
	for _, inp := range inputs {
		k := key{inp.Name, int64(math.Round(inp.Latitude * 1e4)), int64(math.Round(inp.Longitude * 1e4))}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, inp)
	}
	return out
}

func (r *WaypointRepo) BulkUpsert(ctx context.Context, inputs []svcwaypoints.UpsertWaypointInput) error {
	inputs = deduplicateByNameAndLocation(inputs)
	for i := 0; i < len(inputs); i += bulkChunkSize {
		end := i + bulkChunkSize
		if end > len(inputs) {
			end = len(inputs)
		}
		if err := r.bulkUpsertChunk(ctx, inputs[i:end]); err != nil {
			return fmt.Errorf("chunk starting at %d: %w", i, err)
		}
	}
	return nil
}

// RebuildEdges truncates and repopulates waypoint_edges from the current
// waypoints table using a PostGIS spatial join (500 km radius, 3 neighbours, non-grid only).
// Called by the fetch-waypoints job after every bulk upsert.
func (r *WaypointRepo) RebuildEdges(ctx context.Context) error {
	// Backfill location for any rows missing it (idempotent).
	if _, err := r.db.ExecContext(ctx, `
		UPDATE waypoints
		SET location = ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography
		WHERE location IS NULL
	`); err != nil {
		return err
	}

	// Ensure the GiST index exists — fast no-op if already present.
	if _, err := r.db.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS waypoints_location_gist ON waypoints USING GIST (location)`,
	); err != nil {
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		TRUNCATE waypoint_edges;

		INSERT INTO waypoint_edges (from_name, to_name, distance_m)
		SELECT a.name, near.name, near.dist
		FROM (
		    SELECT DISTINCT ON (name) name, location
		    FROM waypoints
		    WHERE grid = false
		    ORDER BY name, updated_at DESC
		) a
		CROSS JOIN LATERAL (
		    SELECT b.name, ST_Distance(a.location, b.location) AS dist
		    FROM (
		        SELECT DISTINCT ON (name) name, location
		        FROM waypoints
		        WHERE grid = false
		        ORDER BY name, updated_at DESC
		    ) b
		    WHERE b.name != a.name
		      AND ST_DWithin(a.location, b.location, 500000)
		    ORDER BY dist
		    LIMIT 3
		) near;
	`)
	return err
}

func (r *WaypointRepo) bulkUpsertChunk(ctx context.Context, inputs []svcwaypoints.UpsertWaypointInput) error {
	var sb strings.Builder
	args := make([]any, 0, len(inputs)*5)

	sb.WriteString("INSERT INTO waypoints (name, latitude, longitude, grid, airport, location) VALUES ")
	for i, input := range inputs {
		if i > 0 {
			sb.WriteByte(',')
		}
		n := i*5 + 1
		// location is derived from the same lat/lon params — no extra arg needed.
		fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,ST_SetSRID(ST_MakePoint($%d,$%d),4326)::geography)", n, n+1, n+2, n+3, n+4, n+2, n+1)
		args = append(args, input.Name, input.Latitude, input.Longitude, input.Grid, input.Airport)
	}
	sb.WriteString(` ON CONFLICT (name, ROUND(latitude::numeric, 4), ROUND(longitude::numeric, 4)) DO UPDATE SET ` +
		`"grid" = EXCLUDED."grid", ` +
		`"airport" = EXCLUDED."airport", ` +
		`"location" = EXCLUDED."location", ` +
		`"updated_at" = NOW()`)

	_, err := r.db.ExecContext(ctx, sb.String(), args...)
	return err
}

