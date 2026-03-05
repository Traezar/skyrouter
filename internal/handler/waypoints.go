package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"skyrouter/internal/service/waypoints"
)

type WaypointHandler struct {
	svc *waypoints.WaypointService
}

func NewWaypointHandler(svc *waypoints.WaypointService) *WaypointHandler {
	return &WaypointHandler{svc: svc}
}

func (h *WaypointHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListWaypoints)
	r.Get("/{id}", h.GetWaypoint)
	return r
}

func (h *WaypointHandler) ListWaypoints(w http.ResponseWriter, r *http.Request) {
	waypoints, err := h.svc.ListWaypoints(r.Context())
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(waypoints)
}

func (h *WaypointHandler) GetWaypoint(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	wp, err := h.svc.GetWaypoint(r.Context(), int32(id))
	if err != nil {
		if errors.Is(err, waypoints.ErrNotFound) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wp)
}
