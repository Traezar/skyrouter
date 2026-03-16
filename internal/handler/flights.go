package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"skyrouter/internal/graph"
	"skyrouter/internal/service/flights"
)

type FlightHandler struct {
	svc        *flights.FlightService
	graphCache *graph.Cache
}

func NewFlightHandler(svc *flights.FlightService, graphCache *graph.Cache) *FlightHandler {
	return &FlightHandler{svc: svc, graphCache: graphCache}
}

func (h *FlightHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListFlights)
	r.Get("/{id}", h.GetFlight)
	r.Get("/{id}/alternatives", h.Alternatives)
	return r
}

func (h *FlightHandler) GetFlight(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	flight, err := h.svc.GetFlight(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		slog.Error("GetFlight failed", "id", id, "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flight)
}

func (h *FlightHandler) ListFlights(w http.ResponseWriter, r *http.Request) {
	filter := flights.ListFlightsFilter{
		Callsign:             r.URL.Query().Get("callsign"),
		DepartureAerodrome:   r.URL.Query().Get("departure"),
		DestinationAerodrome: r.URL.Query().Get("destination"),
		Operator:             r.URL.Query().Get("operator"),
	}

	if v := r.URL.Query().Get("date_from"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			http.Error(w, `{"error":"invalid date_from, expected YYYY-MM-DD"}`, http.StatusBadRequest)
			return
		}
		filter.DateFrom = &t
	}
	if v := r.URL.Query().Get("date_to"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			http.Error(w, `{"error":"invalid date_to, expected YYYY-MM-DD"}`, http.StatusBadRequest)
			return
		}
		filter.DateTo = &t
	}

	result, err := h.svc.ListFlights(r.Context(), filter)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type alternativeWaypoint struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type alternativeRoute struct {
	Rank            int                   `json:"rank"`
	TotalDistanceKm float64               `json:"totalDistanceKm"`
	Waypoints       []alternativeWaypoint `json:"waypoints"`
}

func (h *FlightHandler) Alternatives(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	k := 3
	if v := r.URL.Query().Get("k"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 4 {
			k = n
		}
	}

	flight, err := h.svc.GetFlight(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		slog.Error("Alternatives: GetFlight failed", "id", id, "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	if len(flight.Route) < 2 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
		return
	}

	from := flight.Route[0].WaypointName
	to := flight.Route[len(flight.Route)-1].WaypointName

	g, err := h.graphCache.Get(r.Context())
	if err != nil {
		slog.Error("Alternatives: graph build failed", "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	paths := graph.Yen(g, from, to, k)

	result := make([]alternativeRoute, 0, len(paths))
	for i, p := range paths {
		ar := alternativeRoute{
			Rank:            i + 1,
			TotalDistanceKm: p.TotalDist / 1000,
		}
		for _, name := range p.Nodes {
			node, ok := g.Node(name)
			if !ok {
				continue
			}
			ar.Waypoints = append(ar.Waypoints, alternativeWaypoint{
				Name:      node.Name,
				Latitude:  node.Lat,
				Longitude: node.Lon,
			})
		}
		result = append(result, ar)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
