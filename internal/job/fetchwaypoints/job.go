package fetchwaypoints

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"time"

	"skyrouter/internal/job"
	svcwaypoints "skyrouter/internal/service/waypoints"
)

// allLetters matches waypoint names consisting entirely of letters — these are not grid waypoints.
var allLetters = regexp.MustCompile(`^[A-Za-z]+$`)

type apiWaypoint struct {
	Name      string
	Latitude  float64
	Longitude float64
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func Run(ctx context.Context, deps job.Repos) error {
	apiKey := os.Getenv("WAYPOINTS_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("WAYPOINTS_API_KEY env var is required")
	}

	fixesURL := os.Getenv("WAYPOINTS_ENDPOINT")
	if fixesURL == "" {
		return fmt.Errorf("WAYPOINTS_ENDPOINT env var is required")
	}

	navaidsURL := os.Getenv("NAVAIDS_ENDPOINT")
	if navaidsURL == "" {
		return fmt.Errorf("NAVAIDS_ENDPOINT env var is required")
	}

	airportsURL := os.Getenv("AIRPORTS_ENDPOINT")
	if airportsURL == "" {
		return fmt.Errorf("AIRPORTS_ENDPOINT env var is required")
	}

	fixes, err := fetchWaypoints(ctx, fixesURL, apiKey)
	if err != nil {
		return fmt.Errorf("fetch fixes: %w", err)
	}
	slog.Info("fetched fixes", "count", len(fixes))

	navaids, err := fetchWaypoints(ctx, navaidsURL, apiKey)
	if err != nil {
		return fmt.Errorf("fetch navaids: %w", err)
	}
	slog.Info("fetched navaids", "count", len(navaids))

	airports, err := fetchWaypoints(ctx, airportsURL, apiKey)
	if err != nil {
		return fmt.Errorf("fetch airports: %w", err)
	}
	slog.Info("fetched airports", "count", len(airports))

	all := make([]svcwaypoints.UpsertWaypointInput, 0, len(fixes)+len(navaids)+len(airports))
	for _, wp := range fixes {
		all = append(all, svcwaypoints.UpsertWaypointInput{
			Name:      wp.Name,
			Latitude:  wp.Latitude,
			Longitude: wp.Longitude,
			Grid:      !allLetters.MatchString(wp.Name),
		})
	}
	for _, wp := range navaids {
		all = append(all, svcwaypoints.UpsertWaypointInput{
			Name:      wp.Name,
			Latitude:  wp.Latitude,
			Longitude: wp.Longitude,
			Grid:      false,
		})
	}
	for _, wp := range airports {
		all = append(all, svcwaypoints.UpsertWaypointInput{
			Name:      wp.Name,
			Latitude:  wp.Latitude,
			Longitude: wp.Longitude,
			Airport:   true,
		})
	}

	if err := deps.Waypoints.BulkUpsert(ctx, all); err != nil {
		return fmt.Errorf("bulk upsert: %w", err)
	}
	slog.Info("sync complete", "total", len(all))
	return nil
}

func fetchWaypoints(ctx context.Context, url, apiKey string) ([]apiWaypoint, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	var raw []string
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	waypoints := make([]apiWaypoint, 0, len(raw))
	for _, s := range raw {
		wp, err := parseWaypoint(s)
		if err != nil {
			return nil, err
		}
		waypoints = append(waypoints, wp)
	}
	return waypoints, nil
}

// parseWaypoint parses strings in the format "NAME (lat,lon)",
// e.g. "D090O (42.80,142.01)" or "CHW (48.48,0.99)".
func parseWaypoint(s string) (apiWaypoint, error) {
	var wp apiWaypoint
	n, err := fmt.Sscanf(s, "%s (%f,%f)", &wp.Name, &wp.Latitude, &wp.Longitude)
	if err != nil || n != 3 {
		return wp, fmt.Errorf("parse waypoint %q: expected format \"NAME (lat,lon)\"", s)
	}
	return wp, nil
}
