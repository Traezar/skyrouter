package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	bobpgx "github.com/stephenafamo/bob/drivers/pgx"

	"skyrouter/internal/config"
	"skyrouter/internal/db"
	"skyrouter/internal/graph"
	"skyrouter/internal/handler"
	repoFlights "skyrouter/internal/repo/flights"
	waypointrepo "skyrouter/internal/repo/waypoints"
	svcflights "skyrouter/internal/service/flights"
	"skyrouter/internal/service/waypoints"
)

func main() {
	var logLevel slog.Level
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	cfg := config.Load()

	pool, err := db.Connect(cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected", "host", cfg.Database.Host, "name", cfg.Database.Name)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Health checks have no logger middleware — keeps them out of CloudWatch.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"status":"not ready","db":"unreachable"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	exec := bobpgx.NewPool(pool)

	// Graph cache — reloads the pre-built waypoint_edges table every 24 hours.
	graphCache := graph.NewCache(pool, 24*time.Hour)

	// API routes get request logging.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Logger)

		waypointSvc := waypoints.NewWaypointService(waypointrepo.NewWaypointRepo(exec))
		r.Mount("/waypoints", handler.NewWaypointHandler(waypointSvc).Routes())

		flightSvc := svcflights.NewFlightService(repoFlights.NewFlightRepo(exec))
		r.Mount("/flights", handler.NewFlightHandler(flightSvc, graphCache).Routes())
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}
}
