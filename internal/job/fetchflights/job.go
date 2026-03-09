package fetchflights

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aarondl/opt/null"
	"github.com/lib/pq"

	"skyrouter/internal/job"
	repoflights "skyrouter/internal/repo/flights"
)

// API response structs — field names match the upstream JSON exactly.

type apiFlight struct {
	ID                     string         `json:"_id"`
	MessageType            string         `json:"messageType"`
	AircraftIdentification string         `json:"aircraftIdentification"`
	FlightType             *string        `json:"flightType"`
	AircraftOperating      *string        `json:"aircraftOperating"`
	SRC                    *string        `json:"src"`
	Remark                 *string        `json:"remark"`
	Aircraft               apiAircraft    `json:"aircraft"`
	Departure              apiDeparture   `json:"departure"`
	Arrival                apiArrival     `json:"arrival"`
	Enroute                *apiEnroute    `json:"enroute"`
	FiledRoute             *apiFiledRoute `json:"filedRoute"`
	ReceptionTime          *string        `json:"receptionTime"`
	LastUpdatedTimeStamp   *string        `json:"lastUpdatedTimeStamp"`
}

type apiAircraft struct {
	AircraftType         *string `json:"aircraftType"`
	WakeTurbulence       *string `json:"wakeTurbulence"`
	AircraftRegistration *string `json:"aircraftRegistration"`
	AircraftAddress      *string `json:"aircraftAddress"`
}

type apiDeparture struct {
	DepartureAerodrome    string  `json:"departureAerodrome"`
	DateOfFlight          string  `json:"dateOfFlight"`          // YYYY-MM-DD
	EstimatedOffBlockTime *string `json:"estimatedOffBLockTime"` // HH:MM:SS (note API typo: BL)
	TimeOfFlight          *int64  `json:"timeOfFlight"`          // unix timestamp → scheduled_departure_at
}

type apiArrival struct {
	DestinationAerodrome string   `json:"destinationAerodrome"`
	AlternativeAerodrome []string `json:"alternativeAerodrome"`
	TimeOfArrival        *int64   `json:"timeOfArrival"` // unix timestamp → scheduled_arrival_at
}

type apiEnroute struct {
	AlternativeEnRouteAerodrome *string `json:"alternativeEnRouteAerodrome"`
	CurrentModeACode            *string `json:"currentModeACode"`
}

type apiFiledRoute struct {
	FlightRuleCategory        *string           `json:"flightRuleCategory"`
	CruisingSpeed             *string           `json:"cruisingSpeed"`
	CruisingLevel             *string           `json:"cruisingLevel"`
	RouteText                 *string           `json:"routeText"`
	TotalEstimatedElapsedTime *string           `json:"totalEstimatedElapsedTime"`
	OtherEstimatedElapsedTime []string          `json:"otherEstimatedElapsedTime"`
	RouteElement              []apiRouteElement `json:"routeElement"`
}

type apiRouteElement struct {
	Position    apiPosition `json:"position"`
	SeqNum      int         `json:"seqNum"`
	Airway      *string     `json:"airway"`
	AirwayType  *string     `json:"airwayType"`
	ChangeSpeed *string     `json:"changeSpeed"`
	ChangeLevel *string     `json:"changeLevel"`
}

type apiPosition struct {
	DesignatedPoint *string `json:"designatedPoint"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func Run(ctx context.Context, deps job.Repos) error {
	endpointURL := os.Getenv("FLIGHTS_ENDPOINT")
	if endpointURL == "" {
		return fmt.Errorf("FLIGHTS_ENDPOINT env var is required")
	}

	apiKey := os.Getenv("FLIGHTS_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("FLIGHTS_API_KEY env var is required")
	}

	fetched, err := fetchFlights(ctx, endpointURL, apiKey)
	if err != nil {
		return fmt.Errorf("fetch flights: %w", err)
	}
	slog.Info("fetched flights", "count", len(fetched))

	inputs, err := toInputs(fetched)
	if err != nil {
		return fmt.Errorf("map inputs: %w", err)
	}

	if err := deps.Flights.UpsertFlights(ctx, inputs); err != nil {
		return fmt.Errorf("upsert flights: %w", err)
	}
	slog.Info("sync complete", "total", len(inputs))
	return nil
}

func fetchFlights(ctx context.Context, url, apiKey string) ([]apiFlight, error) {
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

	var flights []apiFlight
	if err := json.NewDecoder(resp.Body).Decode(&flights); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return flights, nil
}

func toInputs(raw []apiFlight) ([]repoflights.UpsertFlightInput, error) {
	inputs := make([]repoflights.UpsertFlightInput, 0, len(raw))
	skipped := 0
	for _, f := range raw {
		if f.ID == "" || f.Departure.DateOfFlight == "" {
			slog.Warn("skipping flight with missing required fields",
				"id", f.ID,
				"date_of_flight", f.Departure.DateOfFlight,
				"callsign", f.AircraftIdentification,
			)
			skipped++
			continue
		}
		inp, err := mapFlight(f)
		if err != nil {
			return nil, fmt.Errorf("flight %q: %w", f.ID, err)
		}
		inputs = append(inputs, inp)
	}
	if skipped > 0 {
		slog.Warn("skipped flights with missing required fields", "count", skipped)
	}
	return inputs, nil
}

func mapFlight(f apiFlight) (repoflights.UpsertFlightInput, error) {
	date, err := time.Parse("2006-01-02", f.Departure.DateOfFlight)
	if err != nil {
		return repoflights.UpsertFlightInput{}, fmt.Errorf("parse date_of_flight %q: %w", f.Departure.DateOfFlight, err)
	}

	inp := repoflights.UpsertFlightInput{
		SourceID:    f.ID,
		MessageType: f.MessageType,
		Callsign:    f.AircraftIdentification,

		FlightType: null.FromPtr(f.FlightType),
		Operator:   null.FromPtr(f.AircraftOperating),
		SRC:        null.FromPtr(f.SRC),
		Remark:     null.FromPtr(f.Remark),

		AircraftType:         null.FromPtr(f.Aircraft.AircraftType),
		WakeTurbulence:       null.FromPtr(f.Aircraft.WakeTurbulence),
		AircraftRegistration: null.FromPtr(f.Aircraft.AircraftRegistration),
		AircraftAddress:      null.FromPtr(f.Aircraft.AircraftAddress),

		DepartureAerodrome:   f.Departure.DepartureAerodrome,
		DateOfFlight:         date,
		DestinationAerodrome: f.Arrival.DestinationAerodrome,
	}

	if len(f.Arrival.AlternativeAerodrome) > 0 {
		inp.AlternateAerodromes = null.From(pq.StringArray(f.Arrival.AlternativeAerodrome))
	}

	// estimated_off_block_time is a TIME value ("HH:MM:SS")
	if s := f.Departure.EstimatedOffBlockTime; s != nil && *s != "" {
		t, err := time.Parse("15:04:05", *s)
		if err != nil {
			return inp, fmt.Errorf("parse estimated_off_block_time %q: %w", *s, err)
		}
		inp.EstimatedOffBlockTime = null.From(t)
	}

	// timeOfFlight / timeOfArrival are unix timestamps
	if ts := f.Departure.TimeOfFlight; ts != nil && *ts > 0 {
		inp.ScheduledDepartureAt = null.From(time.Unix(*ts, 0).UTC())
	}
	if ts := f.Arrival.TimeOfArrival; ts != nil && *ts > 0 {
		inp.ScheduledArrivalAt = null.From(time.Unix(*ts, 0).UTC())
	}

	if f.Enroute != nil {
		inp.AlternateEnroute = null.FromPtr(f.Enroute.AlternativeEnRouteAerodrome)
		inp.ModeACode = null.FromPtr(f.Enroute.CurrentModeACode)
	}

	if err := parseRFC3339(f.ReceptionTime, &inp.ReceptionTime); err != nil {
		return inp, fmt.Errorf("receptionTime: %w", err)
	}
	if err := parseRFC3339(f.LastUpdatedTimeStamp, &inp.LastUpdatedAt); err != nil {
		return inp, fmt.Errorf("lastUpdatedTimeStamp: %w", err)
	}

	if f.FiledRoute != nil {
		inp.Route = mapRoute(f.FiledRoute)
	}
	return inp, nil
}

func mapRoute(r *apiFiledRoute) repoflights.RouteInput {
	route := repoflights.RouteInput{
		FlightRules:      null.FromPtr(r.FlightRuleCategory),
		CruisingSpeed:    null.FromPtr(r.CruisingSpeed),
		CruisingLevel:    null.FromPtr(r.CruisingLevel),
		RouteText:        null.FromPtr(r.RouteText),
		TotalElapsedTime: null.FromPtr(r.TotalEstimatedElapsedTime),
	}
	if len(r.OtherEstimatedElapsedTime) > 0 {
		route.FirEstimates = null.From(pq.StringArray(r.OtherEstimatedElapsedTime))
	}
	for _, el := range r.RouteElement {
		name := ""
		if el.Position.DesignatedPoint != nil {
			name = *el.Position.DesignatedPoint
		}
		route.Elements = append(route.Elements, repoflights.RouteElementInput{
			SeqNum:       el.SeqNum,
			WaypointName: name,
			Airway:       null.FromPtr(el.Airway),
			AirwayType:   null.FromPtr(el.AirwayType),
			ChangeSpeed:  null.FromPtr(el.ChangeSpeed),
			ChangeLevel:  null.FromPtr(el.ChangeLevel),
		})
	}
	return route
}

// parseRFC3339 parses an optional RFC3339 timestamp (with optional sub-second precision).
func parseRFC3339(s *string, dst *null.Val[time.Time]) error {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, *s)
	if err != nil {
		return err
	}
	*dst = null.From(t)
	return nil
}
