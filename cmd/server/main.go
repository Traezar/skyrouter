package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	bobpgx "github.com/stephenafamo/bob/drivers/pgx"

	"skyrouter/internal/config"
	"skyrouter/internal/db"
	"skyrouter/internal/handler"
	repoFlights "skyrouter/internal/repo/flights"
	waypointrepo "skyrouter/internal/repo/waypoints"
	svcflights "skyrouter/internal/service/flights"
	"skyrouter/internal/service/waypoints"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	database, err := db.Connect(cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	slog.Info("database connected", "host", cfg.Database.Host, "name", cfg.Database.Name)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	exec := bobpgx.NewPool(database)

	waypointSvc := waypoints.NewWaypointService(waypointrepo.NewWaypointRepo(exec))
	r.Mount("/waypoints", handler.NewWaypointHandler(waypointSvc).Routes())

	flightSvc := svcflights.NewFlightService(repoFlights.NewFlightRepo(exec))
	r.Mount("/flights", handler.NewFlightHandler(flightSvc).Routes())

	// Liveness: process is alive — no dependency checks.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Readiness: dependencies are reachable — used by k8s to gate traffic.
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := database.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"status":"not ready","db":"unreachable"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
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
