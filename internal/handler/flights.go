package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"skyrouter/internal/service/flights"
)

type FlightHandler struct {
	svc *flights.FlightService
}

func NewFlightHandler(svc *flights.FlightService) *FlightHandler {
	return &FlightHandler{svc: svc}
}

func (h *FlightHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListFlights)
	return r
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
