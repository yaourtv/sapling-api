// Package model defines the API-facing domain types, decoupled from DB row types.
package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID `json:"id"`
	Email            string    `json:"email"`
	DisplayName      string    `json:"display_name"`
	Timezone         string    `json:"timezone"`
	ArchiveAfterDays int       `json:"archive_after_days"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthResponse struct {
	User User `json:"user"`
	AuthTokens
}

type Status struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Color    string    `json:"color"`
	Position int       `json:"position"`
	IsSystem bool      `json:"is_system"`
}

// StatusRef is the embedded status summary on an application.
type StatusRef struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Color string    `json:"color"`
}

type Application struct {
	ID               uuid.UUID `json:"id"`
	Company          string    `json:"company"`
	Position         *string   `json:"position"`
	Link             *string   `json:"link"`
	Description      *string   `json:"description"`
	Status           StatusRef `json:"status"`
	AppliedAt        time.Time `json:"applied_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	LastStatusChange time.Time `json:"last_status_change"`
}

type StatusHistoryEntry struct {
	ID         uuid.UUID  `json:"id"`
	StatusID   *uuid.UUID `json:"status_id"`
	StatusName string     `json:"status_name"`
	ChangedAt  time.Time  `json:"changed_at"`
}

type Event struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type AvailabilityWindow struct {
	ID           uuid.UUID `json:"id"`
	IsRecurring  bool      `json:"is_recurring"`
	DayOfWeek    *int      `json:"day_of_week,omitempty"`   // 0=Mon … 6=Sun
	SpecificDate *string   `json:"specific_date,omitempty"` // YYYY-MM-DD
	StartTime    string    `json:"start_time"`              // HH:MM, UTC
	EndTime      string    `json:"end_time"`                // HH:MM, UTC
}

type Invitation struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	Description    *string   `json:"description"`
	ProposedStarts time.Time `json:"proposed_starts"`
	ProposedEnds   time.Time `json:"proposed_ends"`
	CreatedAt      time.Time `json:"created_at"`
}

// SlotType classifies a public calendar slot.
type SlotType string

const (
	SlotBusy      SlotType = "busy"
	SlotTentative SlotType = "tentative"
	SlotAvailable SlotType = "available"
)

type Slot struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Type     SlotType  `json:"type"`
}

type PublicCalendar struct {
	Timezone string `json:"timezone"`
	Slots    []Slot `json:"slots"`
}
