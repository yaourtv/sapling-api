package service

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// ValidationError carries a user-facing message for a 400 response.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func validationf(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

// StatusInUseError is returned when deleting a status still used by
// applications without confirmation (409 Conflict).
type StatusInUseError struct {
	AffectedCount int64     `json:"affected_count"`
	LostStatusID  uuid.UUID `json:"lost_status_id"`
}

func (e *StatusInUseError) Error() string {
	return fmt.Sprintf("status in use by %d applications", e.AffectedCount)
}
