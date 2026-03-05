package job

import (
	"context"

	repoWaypoints "skyrouter/internal/repo/waypoints"
)

// Runner is the standard signature every job must implement.
type Runner func(ctx context.Context, deps Repos) error

// Repos holds shared resources available to all jobs.
type Repos struct {
	Waypoints repoWaypoints.WaypointRepository
}
