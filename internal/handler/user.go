package handler

import (
	"net/http"

	"github.com/yaourtv/sapling-api/internal/middleware"
	"github.com/yaourtv/sapling-api/internal/service"
)

type UserHandler struct {
	users *service.UserService
}

func NewUserHandler(users *service.UserService) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.users.Get(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var in service.UpdateProfileInput
	if err := decodeJSON(r, &in); err != nil {
		writeServiceError(w, err)
		return
	}
	user, err := h.users.UpdateProfile(r.Context(), middleware.UserID(r.Context()), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) CalendarToken(w http.ResponseWriter, r *http.Request) {
	tok, err := h.users.CalendarToken(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": tok, "url": "/" + tok})
}

func (h *UserHandler) RegenerateCalendarToken(w http.ResponseWriter, r *http.Request) {
	tok, err := h.users.RegenerateCalendarToken(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": tok, "url": "/" + tok})
}
