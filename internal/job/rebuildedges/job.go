package rebuildedges

import (
	"context"
	"log/slog"

	"skyrouter/internal/job"
)

func Run(ctx context.Context, deps job.Repos) error {
	slog.Info("rebuilding waypoint edges")
	if err := deps.Waypoints.RebuildEdges(ctx); err != nil {
		return err
	}
	slog.Info("waypoint edges rebuilt")
	return nil
}
