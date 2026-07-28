package handler

import (
	"net/http"

	"github.com/yaourtv/sapling-api/internal/middleware"
	"github.com/yaourtv/sapling-api/internal/service"
)

type StatusHandler struct {
	statuses *service.StatusService
}

func NewStatusHandler(statuses *service.StatusService) *StatusHandler {
	return &StatusHandler{statuses: statuses}
}

func (h *StatusHandler) List(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.statuses.List(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": statuses})
}

func (h *StatusHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.CreateStatusInput
	if err := decodeJSON(r, &in); err != nil {
		writeServiceError(w, err)
		return
	}
	status, err := h.statuses.Create(r.Context(), middleware.UserID(r.Context()), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, status)
}

func (h *StatusHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	var in service.UpdateStatusInput
	if err := decodeJSON(r, &in); err != nil {
		writeServiceError(w, err)
		return
	}
	status, err := h.statuses.Update(r.Context(), middleware.UserID(r.Context()), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *StatusHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	confirm := r.URL.Query().Get("confirm") == "true"
	if err := h.statuses.Delete(r.Context(), middleware.UserID(r.Context()), id, confirm); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
