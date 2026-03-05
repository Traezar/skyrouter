package main

import (
	"context"
	"log/slog"
	"os"
	"sort"
	"strings"

	bobpgx "github.com/stephenafamo/bob/drivers/pgx"

	"skyrouter/internal/config"
	"skyrouter/internal/db"
	"skyrouter/internal/job"
	"skyrouter/internal/job/fetchwaypoints"
	repoWaypoints "skyrouter/internal/repo/waypoints"
)

var registry = map[string]job.Runner{
	"fetch-waypoints": fetchwaypoints.Run,
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if len(os.Args) < 2 {
		slog.Error("job name required", "available", availableJobs())
		os.Exit(1)
	}

	jobName := os.Args[1]
	runner, ok := registry[jobName]
	if !ok {
		slog.Error("unknown job", "name", jobName, "available", availableJobs())
		os.Exit(1)
	}

	cfg := config.Load()

	pool, err := db.Connect(cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	deps := job.Repos{
		Waypoints: repoWaypoints.NewWaypointRepo(bobpgx.NewPool(pool)),
	}

	ctx := context.Background()

	slog.Info("running job", "name", jobName)
	if err := runner(ctx, deps); err != nil {
		slog.Error("job failed", "name", jobName, "error", err)
		os.Exit(1)
	}
	slog.Info("job complete", "name", jobName)
}

func availableJobs() string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
