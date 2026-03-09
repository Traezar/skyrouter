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

// gridPattern matches waypoint names that are exactly 4 digits followed by 'E', e.g. "5790E".
var gridPattern = regexp.MustCompile(`^\d{4}E$`)

type apiWaypoint struct {
	Name      string
	Latitude  float64
	Longitude float64
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func Run(ctx context.Context, deps job.Repos) error {
	endpointURL := os.Getenv("WAYPOINTS_ENDPOINT")
	if endpointURL == "" {
		return fmt.Errorf("WAYPOINTS_ENDPOINT env var is required")
	}

	apiKey := os.Getenv("WAYPOINTS_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("WAYPOINTS_API_KEY env var is required")
	}

	fetched, err := fetchWaypoints(ctx, endpointURL, apiKey)
	if err != nil {
		return fmt.Errorf("fetch waypoints: %w", err)
	}
	slog.Info("fetched waypoints", "count", len(fetched), "url", endpointURL)

	inputs := make([]svcwaypoints.UpsertWaypointInput, len(fetched))
	for i, wp := range fetched {
		inputs[i] = svcwaypoints.UpsertWaypointInput{
			Name:      wp.Name,
			Latitude:  wp.Latitude,
			Longitude: wp.Longitude,
			Grid:      gridPattern.MatchString(wp.Name),
		}
	}

	if err := deps.Waypoints.BulkUpsert(ctx, inputs); err != nil {
		return fmt.Errorf("bulk upsert: %w", err)
	}
	slog.Info("sync complete", "total", len(inputs))
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
// e.g. "D090O (42.80,142.01)" or "5790W (-57.00,-90.00)".
func parseWaypoint(s string) (apiWaypoint, error) {
	var wp apiWaypoint
	n, err := fmt.Sscanf(s, "%s (%f,%f)", &wp.Name, &wp.Latitude, &wp.Longitude)
	if err != nil || n != 3 {
		return wp, fmt.Errorf("parse waypoint %q: expected format \"NAME (lat,lon)\"", s)
	}
	return wp, nil
}
