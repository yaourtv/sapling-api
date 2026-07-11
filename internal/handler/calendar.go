package handler

import (
	"net/http"
	"time"

	"github.com/egoralekseychik/job-tracker-api/internal/middleware"
	"github.com/egoralekseychik/job-tracker-api/internal/service"
)

type CalendarHandler struct {
	calendar *service.CalendarService
}

func NewCalendarHandler(calendar *service.CalendarService) *CalendarHandler {
	return &CalendarHandler{calendar: calendar}
}

// parseRange reads optional ?from=&to= RFC3339 query params, defaulting to
// [now-7d, now+28d).
func parseRange(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
	to := now.AddDate(0, 0, 28).Truncate(24 * time.Hour)
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, time.Time{}, &service.ValidationError{Message: "from must be RFC3339"}
		}
		from = t.UTC()
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, time.Time{}, &service.ValidationError{Message: "to must be RFC3339"}
		}
		to = t.UTC()
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, &service.ValidationError{Message: "to must be after from"}
	}
	return from, to, nil
}

func (h *CalendarHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRange(r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	events, err := h.calendar.ListEvents(r.Context(), middleware.UserID(r.Context()), from, to)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": events})
}

func (h *CalendarHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var in service.EventInput
	if err := decodeJSON(r, &in); err != nil {
		writeServiceError(w, err)
		return
	}
	event, err := h.calendar.CreateEvent(r.Context(), middleware.UserID(r.Context()), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (h *CalendarHandler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	var in service.EventInput
	if err := decodeJSON(r, &in); err != nil {
		writeServiceError(w, err)
		return
	}
	event, err := h.calendar.UpdateEvent(r.Context(), middleware.UserID(r.Context()), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (h *CalendarHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.calendar.DeleteEvent(r.Context(), middleware.UserID(r.Context()), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CalendarHandler) ListAvailability(w http.ResponseWriter, r *http.Request) {
	windows, err := h.calendar.ListAvailability(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": windows})
}

func (h *CalendarHandler) CreateAvailability(w http.ResponseWriter, r *http.Request) {
	var in service.AvailabilityInput
	if err := decodeJSON(r, &in); err != nil {
		writeServiceError(w, err)
		return
	}
	window, err := h.calendar.CreateAvailability(r.Context(), middleware.UserID(r.Context()), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, window)
}

func (h *CalendarHandler) DeleteAvailability(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.calendar.DeleteAvailability(r.Context(), middleware.UserID(r.Context()), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CalendarHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	invitations, err := h.calendar.ListInvitations(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": invitations})
}

func (h *CalendarHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	event, err := h.calendar.AcceptInvitation(r.Context(), middleware.UserID(r.Context()), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (h *CalendarHandler) DeclineInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.calendar.DeclineInvitation(r.Context(), middleware.UserID(r.Context()), id); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "declined"})
}
