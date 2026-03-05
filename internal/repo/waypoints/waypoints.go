package waypoints

import (
	"context"
	"fmt"
	"strings"

	svcwaypoints "skyrouter/internal/service/waypoints"
	"skyrouter/models"
)

type WaypointRepo struct {
	db Executor
}

func NewWaypointRepo(db Executor) *WaypointRepo {
	return &WaypointRepo{db: db}
}

func (r *WaypointRepo) List(ctx context.Context) (models.WaypointSlice, error) {
	return models.Waypoints.Query().All(ctx, r.db)
}

func (r *WaypointRepo) GetByID(ctx context.Context, id int32) (*models.Waypoint, error) {
	return models.FindWaypoint(ctx, r.db, id)
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
	args := make([]any, 0, len(inputs)*3)

	sb.WriteString("INSERT INTO waypoints (name, latitude, longitude) VALUES ")
	for i, input := range inputs {
		if i > 0 {
			sb.WriteByte(',')
		}
		n := i*3 + 1
		fmt.Fprintf(&sb, "($%d,$%d,$%d)", n, n+1, n+2)
		args = append(args, input.Name, input.Latitude, input.Longitude)
	}
	sb.WriteString(` ON CONFLICT (name) DO UPDATE SET ` +
		`"latitude" = EXCLUDED."latitude", ` +
		`"longitude" = EXCLUDED."longitude", ` +
		`"updated_at" = NOW()`)

	_, err := r.db.ExecContext(ctx, sb.String(), args...)
	return err
}

