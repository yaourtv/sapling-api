package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/egoralekseychik/job-tracker-api/internal/middleware"
	"github.com/egoralekseychik/job-tracker-api/internal/token"
)

// Handlers bundles every HTTP handler needed to build the router.
type Handlers struct {
	Auth        *AuthHandler
	User        *UserHandler
	Application *ApplicationHandler
	Status      *StatusHandler
	Calendar    *CalendarHandler
	Public      *PublicHandler
}

// NewRouter wires all routes and middleware.
func NewRouter(h Handlers, tokens *token.Manager, corsOrigin string) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.CORS(corsOrigin))

	authLimiter := middleware.NewRateLimiter(20, 10)   // credential endpoints
	publicLimiter := middleware.NewRateLimiter(60, 30) // unauthenticated calendar

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Use(authLimiter.Middleware)
			r.Post("/register", h.Auth.Register)
			r.Post("/login", h.Auth.Login)
			r.Post("/refresh", h.Auth.Refresh)
			r.Post("/logout", h.Auth.Logout)
			r.Post("/forgot-password", h.Auth.ForgotPassword)
			r.Post("/reset-password", h.Auth.ResetPassword)
		})

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(tokens))

			r.Route("/user", func(r chi.Router) {
				r.Get("/me", h.User.Me)
				r.Patch("/me", h.User.UpdateMe)
				r.Get("/calendar-token", h.User.CalendarToken)
				r.Post("/calendar-token/regenerate", h.User.RegenerateCalendarToken)
			})

			r.Route("/applications", func(r chi.Router) {
				r.Get("/", h.Application.List)
				r.Post("/", h.Application.Create)
				r.Get("/{id}", h.Application.Get)
				r.Patch("/{id}", h.Application.Update)
				r.Delete("/{id}", h.Application.Delete)
				r.Get("/{id}/history", h.Application.History)
				r.Patch("/{id}/status", h.Application.ChangeStatus)
			})

			r.Route("/statuses", func(r chi.Router) {
				r.Get("/", h.Status.List)
				r.Post("/", h.Status.Create)
				r.Patch("/{id}", h.Status.Update)
				r.Delete("/{id}", h.Status.Delete)
			})

			r.Route("/calendar", func(r chi.Router) {
				r.Get("/events", h.Calendar.ListEvents)
				r.Post("/events", h.Calendar.CreateEvent)
				r.Patch("/events/{id}", h.Calendar.UpdateEvent)
				r.Delete("/events/{id}", h.Calendar.DeleteEvent)
				r.Get("/availability", h.Calendar.ListAvailability)
				r.Post("/availability", h.Calendar.CreateAvailability)
				r.Delete("/availability/{id}", h.Calendar.DeleteAvailability)
				r.Get("/invitations", h.Calendar.ListInvitations)
				r.Post("/invitations/{id}/accept", h.Calendar.AcceptInvitation)
				r.Post("/invitations/{id}/decline", h.Calendar.DeclineInvitation)
			})
		})

		// Public calendar (unauthenticated).
		r.Route("/c/{token}", func(r chi.Router) {
			r.Use(publicLimiter.Middleware)
			r.Get("/", h.Public.GetCalendar)
			r.Post("/invite", h.Public.SubmitInvitation)
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return r
}
