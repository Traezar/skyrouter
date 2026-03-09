package waypoints

import (
	"context"
	"fmt"
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

func deduplicateByName(inputs []svcwaypoints.UpsertWaypointInput) []svcwaypoints.UpsertWaypointInput {
	seen := make(map[string]struct{}, len(inputs))
	out := make([]svcwaypoints.UpsertWaypointInput, 0, len(inputs))
	for _, inp := range inputs {
		if _, ok := seen[inp.Name]; ok {
			continue
		}
		seen[inp.Name] = struct{}{}
		out = append(out, inp)
	}
	return out
}

func (r *WaypointRepo) BulkUpsert(ctx context.Context, inputs []svcwaypoints.UpsertWaypointInput) error {
	inputs = deduplicateByName(inputs)
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

func (r *WaypointRepo) bulkUpsertChunk(ctx context.Context, inputs []svcwaypoints.UpsertWaypointInput) error {
	var sb strings.Builder
	args := make([]any, 0, len(inputs)*4)

	sb.WriteString("INSERT INTO waypoints (name, latitude, longitude, grid) VALUES ")
	for i, input := range inputs {
		if i > 0 {
			sb.WriteByte(',')
		}
		n := i*4 + 1
		fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d)", n, n+1, n+2, n+3)
		args = append(args, input.Name, input.Latitude, input.Longitude, input.Grid)
	}
	sb.WriteString(` ON CONFLICT (name) DO UPDATE SET ` +
		`"latitude" = EXCLUDED."latitude", ` +
		`"longitude" = EXCLUDED."longitude", ` +
		`"grid" = EXCLUDED."grid", ` +
		`"updated_at" = NOW()`)

	_, err := r.db.ExecContext(ctx, sb.String(), args...)
	return err
}

