package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/egoralekseychik/job-tracker-api/internal/db/query"
	"github.com/egoralekseychik/job-tracker-api/internal/model"
	"github.com/egoralekseychik/job-tracker-api/internal/token"
)

type UserService struct {
	q *query.Queries
}

func NewUserService(pool *pgxpool.Pool) *UserService {
	return &UserService{q: query.New(pool)}
}

func (s *UserService) Get(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	m := UserToModel(u)
	return &m, nil
}

type UpdateProfileInput struct {
	DisplayName      *string `json:"display_name"`
	Timezone         *string `json:"timezone"`
	ArchiveAfterDays *int    `json:"archive_after_days"`
}

func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, in UpdateProfileInput) (*model.User, error) {
	current, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	params := query.UpdateUserProfileParams{
		ID:               userID,
		DisplayName:      current.DisplayName,
		Timezone:         current.Timezone,
		ArchiveAfterDays: current.ArchiveAfterDays,
	}
	if in.DisplayName != nil {
		if *in.DisplayName == "" {
			return nil, validationf("display_name cannot be empty")
		}
		params.DisplayName = *in.DisplayName
	}
	if in.Timezone != nil {
		if _, err := time.LoadLocation(*in.Timezone); err != nil {
			return nil, validationf("invalid IANA timezone %q", *in.Timezone)
		}
		params.Timezone = *in.Timezone
	}
	if in.ArchiveAfterDays != nil {
		if *in.ArchiveAfterDays < 0 {
			return nil, validationf("archive_after_days must be >= 0")
		}
		params.ArchiveAfterDays = int32(*in.ArchiveAfterDays)
	}
	u, err := s.q.UpdateUserProfile(ctx, params)
	if err != nil {
		return nil, err
	}
	m := UserToModel(u)
	return &m, nil
}

func (s *UserService) CalendarToken(ctx context.Context, userID uuid.UUID) (string, error) {
	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return u.CalendarToken, nil
}

// RegenerateCalendarToken replaces the public calendar token. Existing
// invitations are untouched (tied to user_id); the old URL stops resolving.
func (s *UserService) RegenerateCalendarToken(ctx context.Context, userID uuid.UUID) (string, error) {
	for attempt := 0; ; attempt++ {
		calToken, err := token.GenerateCalendarToken()
		if err != nil {
			return "", err
		}
		_, err = s.q.UpdateUserCalendarToken(ctx, query.UpdateUserCalendarTokenParams{
			ID:            userID,
			CalendarToken: calToken,
		})
		if err == nil {
			return calToken, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && attempt < 5 {
			continue
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("regenerate calendar token: %w", err)
	}
}
