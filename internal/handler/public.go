package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yaourtv/sapling-api/internal/service"
)

// PublicHandler serves the unauthenticated calendar endpoints (/api/c/:token).
type PublicHandler struct {
	calendar *service.CalendarService
}

func NewPublicHandler(calendar *service.CalendarService) *PublicHandler {
	return &PublicHandler{calendar: calendar}
}

func (h *PublicHandler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	from, to, err := parseRange(r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	cal, err := h.calendar.PublicCalendar(r.Context(), token, from, to)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cal)
}

func (h *PublicHandler) SubmitInvitation(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	var in service.PublicInviteInput
	if err := decodeJSON(r, &in); err != nil {
		writeServiceError(w, err)
		return
	}
	invitation, err := h.calendar.SubmitInvitation(r.Context(), token, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invitation)
}
