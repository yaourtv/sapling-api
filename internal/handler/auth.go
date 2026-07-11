package handler

import (
	"net/http"
	"time"

	"github.com/egoralekseychik/job-tracker-api/internal/service"
)

const refreshCookieName = "refresh_token"

type AuthHandler struct {
	auth       *service.AuthService
	refreshTTL time.Duration
}

func NewAuthHandler(auth *service.AuthService, refreshTTL time.Duration) *AuthHandler {
	return &AuthHandler{auth: auth, refreshTTL: refreshTTL}
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Timezone    string `json:"timezone"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, err)
		return
	}
	resp, err := h.auth.Register(r.Context(), req.Email, req.Password, req.DisplayName, req.Timezone)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	h.setRefreshCookie(w, resp.RefreshToken)
	writeJSON(w, http.StatusCreated, resp)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, err)
		return
	}
	resp, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	h.setRefreshCookie(w, resp.RefreshToken)
	writeJSON(w, http.StatusOK, resp)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// refreshTokenFromRequest reads the token from the JSON body or, failing
// that, the HttpOnly cookie.
func refreshTokenFromRequest(r *http.Request) string {
	var req refreshRequest
	// Body is optional here; ignore decode errors.
	_ = decodeJSON(r, &req)
	if req.RefreshToken != "" {
		return req.RefreshToken
	}
	if c, err := r.Cookie(refreshCookieName); err == nil {
		return c.Value
	}
	return ""
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	tokenStr := refreshTokenFromRequest(r)
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}
	resp, err := h.auth.Refresh(r.Context(), tokenStr)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	h.setRefreshCookie(w, resp.RefreshToken)
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if tokenStr := refreshTokenFromRequest(r); tokenStr != "" {
		if err := h.auth.Logout(r.Context(), tokenStr); err != nil {
			writeServiceError(w, err)
			return
		}
	}
	h.clearRefreshCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, err)
		return
	}
	if err := h.auth.ForgotPassword(r.Context(), req.Email); err != nil {
		writeServiceError(w, err)
		return
	}
	// Same response whether or not the email exists.
	writeJSON(w, http.StatusOK, map[string]string{"status": "if the email exists, a reset link has been sent"})
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, err)
		return
	}
	if err := h.auth.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/api/auth",
		MaxAge:   int(h.refreshTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})
}

func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})
}
