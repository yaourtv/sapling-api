package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yaourtv/sapling-api/internal/middleware"
	"github.com/yaourtv/sapling-api/internal/service"
)

type ApplicationHandler struct {
	apps *service.ApplicationService
}

func NewApplicationHandler(apps *service.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{apps: apps}
}

// pathUUID parses a UUID URL parameter; writes a 400 and returns false on failure.
func pathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

func (h *ApplicationHandler) List(w http.ResponseWriter, r *http.Request) {
	includeLost := r.URL.Query().Get("include_lost") == "true"
	apps, err := h.apps.List(r.Context(), middleware.UserID(r.Context()), includeLost)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": apps})
}

func (h *ApplicationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.CreateApplicationInput
	if err := decodeJSON(r, &in); err != nil {
		writeServiceError(w, err)
		return
	}
	app, err := h.apps.Create(r.Context(), middleware.UserID(r.Context()), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (h *ApplicationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	app, err := h.apps.Get(r.Context(), middleware.UserID(r.Context()), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	var in service.UpdateApplicationInput
	if err := decodeJSON(r, &in); err != nil {
		writeServiceError(w, err)
		return
	}
	app, err := h.apps.Update(r.Context(), middleware.UserID(r.Context()), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.apps.Delete(r.Context(), middleware.UserID(r.Context()), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ApplicationHandler) History(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	history, err := h.apps.History(r.Context(), middleware.UserID(r.Context()), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": history})
}

type changeStatusRequest struct {
	StatusID uuid.UUID `json:"status_id"`
}

func (h *ApplicationHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	var req changeStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, err)
		return
	}
	app, err := h.apps.ChangeStatus(r.Context(), middleware.UserID(r.Context()), id, req.StatusID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}
