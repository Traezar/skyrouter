package waypoints

import (
	"context"
	"database/sql"

	"github.com/stephenafamo/scan"

	svcwaypoints "skyrouter/internal/service/waypoints"
	"skyrouter/models"
)

// WaypointRepository is the interface this package satisfies.
type WaypointRepository interface {
	List(ctx context.Context) (models.WaypointSlice, error)
	GetByID(ctx context.Context, id int32) (*models.Waypoint, error)
	BulkUpsert(ctx context.Context, inputs []svcwaypoints.UpsertWaypointInput) error
}

// Executor is the database interface WaypointRepo depends on.
// Any bob.Executor satisfies this interface.
type Executor interface {
	QueryContext(ctx context.Context, query string, args ...any) (scan.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
