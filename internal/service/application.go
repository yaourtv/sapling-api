package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/egoralekseychik/job-tracker-api/internal/db/query"
	"github.com/egoralekseychik/job-tracker-api/internal/model"
)

type ApplicationService struct {
	pool *pgxpool.Pool
	q    *query.Queries
}

func NewApplicationService(pool *pgxpool.Pool) *ApplicationService {
	return &ApplicationService{pool: pool, q: query.New(pool)}
}

type CreateApplicationInput struct {
	Company     string     `json:"company"`
	Position    *string    `json:"position"`
	Link        *string    `json:"link"`
	Description *string    `json:"description"`
	StatusID    uuid.UUID  `json:"status_id"`
	AppliedAt   *time.Time `json:"applied_at"`
}

func (s *ApplicationService) Create(ctx context.Context, userID uuid.UUID, in CreateApplicationInput) (*model.Application, error) {
	if in.Company == "" {
		return nil, validationf("company is required")
	}
	if in.StatusID == uuid.Nil {
		return nil, validationf("status_id is required")
	}
	status, err := s.q.GetStatus(ctx, query.GetStatusParams{ID: in.StatusID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, validationf("status_id does not reference one of your statuses")
		}
		return nil, err
	}
	appliedAt := time.Now().UTC()
	if in.AppliedAt != nil {
		appliedAt = in.AppliedAt.UTC()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	app, err := qtx.CreateApplication(ctx, query.CreateApplicationParams{
		UserID:          userID,
		Company:         in.Company,
		Position:        in.Position,
		Link:            in.Link,
		Description:     in.Description,
		CurrentStatusID: in.StatusID,
		AppliedAt:       appliedAt,
	})
	if err != nil {
		return nil, err
	}
	history, err := qtx.InsertStatusHistory(ctx, query.InsertStatusHistoryParams{
		ApplicationID: app.ID,
		StatusID:      &status.ID,
		StatusName:    status.Name,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	m := applicationToModel(app, status.Name, status.Color, history.ChangedAt)
	return &m, nil
}

func (s *ApplicationService) List(ctx context.Context, userID uuid.UUID, includeLost bool) ([]model.Application, error) {
	rows, err := s.q.ListApplications(ctx, query.ListApplicationsParams{
		UserID:      userID,
		IncludeLost: includeLost,
	})
	if err != nil {
		return nil, err
	}
	out := make([]model.Application, 0, len(rows))
	for _, r := range rows {
		out = append(out, applicationToModel(r.Application, r.StatusName, r.StatusColor, r.LastStatusChange))
	}
	return out, nil
}

func (s *ApplicationService) Get(ctx context.Context, userID, appID uuid.UUID) (*model.Application, error) {
	r, err := s.q.GetApplication(ctx, query.GetApplicationParams{ID: appID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	m := applicationToModel(r.Application, r.StatusName, r.StatusColor, r.LastStatusChange)
	return &m, nil
}

type UpdateApplicationInput struct {
	Company     *string    `json:"company"`
	Position    *string    `json:"position"`
	Link        *string    `json:"link"`
	Description *string    `json:"description"`
	AppliedAt   *time.Time `json:"applied_at"`
}

func (s *ApplicationService) Update(ctx context.Context, userID, appID uuid.UUID, in UpdateApplicationInput) (*model.Application, error) {
	current, err := s.q.GetApplication(ctx, query.GetApplicationParams{ID: appID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	params := query.UpdateApplicationParams{
		ID:          appID,
		UserID:      userID,
		Company:     current.Application.Company,
		Position:    current.Application.Position,
		Link:        current.Application.Link,
		Description: current.Application.Description,
		AppliedAt:   current.Application.AppliedAt,
	}
	if in.Company != nil {
		if *in.Company == "" {
			return nil, validationf("company cannot be empty")
		}
		params.Company = *in.Company
	}
	if in.Position != nil {
		params.Position = in.Position
	}
	if in.Link != nil {
		params.Link = in.Link
	}
	if in.Description != nil {
		params.Description = in.Description
	}
	if in.AppliedAt != nil {
		params.AppliedAt = in.AppliedAt.UTC()
	}
	app, err := s.q.UpdateApplication(ctx, params)
	if err != nil {
		return nil, err
	}
	m := applicationToModel(app, current.StatusName, current.StatusColor, current.LastStatusChange)
	m.UpdatedAt = app.UpdatedAt
	return &m, nil
}

// ChangeStatus updates current_status_id and appends a history entry with a
// status_name snapshot.
func (s *ApplicationService) ChangeStatus(ctx context.Context, userID, appID, statusID uuid.UUID) (*model.Application, error) {
	status, err := s.q.GetStatus(ctx, query.GetStatusParams{ID: statusID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, validationf("status_id does not reference one of your statuses")
		}
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	app, err := qtx.UpdateApplicationStatus(ctx, query.UpdateApplicationStatusParams{
		ID:              appID,
		UserID:          userID,
		CurrentStatusID: statusID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	history, err := qtx.InsertStatusHistory(ctx, query.InsertStatusHistoryParams{
		ApplicationID: appID,
		StatusID:      &status.ID,
		StatusName:    status.Name,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	m := applicationToModel(app, status.Name, status.Color, history.ChangedAt)
	return &m, nil
}

func (s *ApplicationService) Delete(ctx context.Context, userID, appID uuid.UUID) error {
	n, err := s.q.DeleteApplication(ctx, query.DeleteApplicationParams{ID: appID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ApplicationService) History(ctx context.Context, userID, appID uuid.UUID) ([]model.StatusHistoryEntry, error) {
	// Ensure the application exists and belongs to the user (distinguishes 404 from empty history).
	if _, err := s.q.GetApplication(ctx, query.GetApplicationParams{ID: appID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rows, err := s.q.ListStatusHistory(ctx, query.ListStatusHistoryParams{ApplicationID: appID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]model.StatusHistoryEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.StatusHistoryEntry{
			ID:         r.ID,
			StatusID:   r.StatusID,
			StatusName: r.StatusName,
			ChangedAt:  r.ChangedAt,
		})
	}
	return out, nil
}

func applicationToModel(a query.Application, statusName, statusColor string, lastStatusChange time.Time) model.Application {
	return model.Application{
		ID:          a.ID,
		Company:     a.Company,
		Position:    a.Position,
		Link:        a.Link,
		Description: a.Description,
		Status: model.StatusRef{
			ID:    a.CurrentStatusID,
			Name:  statusName,
			Color: statusColor,
		},
		AppliedAt:        a.AppliedAt,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
		LastStatusChange: lastStatusChange,
	}
}
