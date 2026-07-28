package service

import (
	"context"
	"errors"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yaourtv/sapling-api/internal/db/query"
	"github.com/yaourtv/sapling-api/internal/model"
)

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type StatusService struct {
	pool *pgxpool.Pool
	q    *query.Queries
}

func NewStatusService(pool *pgxpool.Pool) *StatusService {
	return &StatusService{pool: pool, q: query.New(pool)}
}

func (s *StatusService) List(ctx context.Context, userID uuid.UUID) ([]model.Status, error) {
	rows, err := s.q.ListStatuses(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Status, 0, len(rows))
	for _, r := range rows {
		out = append(out, statusToModel(r))
	}
	return out, nil
}

type CreateStatusInput struct {
	Name     string `json:"name"`
	Color    string `json:"color"`
	Position *int   `json:"position"`
}

func (s *StatusService) Create(ctx context.Context, userID uuid.UUID, in CreateStatusInput) (*model.Status, error) {
	if in.Name == "" {
		return nil, validationf("name is required")
	}
	if in.Color == "" {
		in.Color = "#6b7280"
	}
	if !hexColorRe.MatchString(in.Color) {
		return nil, validationf("color must be a hex color like #aabbcc")
	}
	position := 0
	if in.Position != nil {
		position = *in.Position
	} else {
		existing, err := s.q.ListStatuses(ctx, userID)
		if err != nil {
			return nil, err
		}
		position = len(existing)
	}
	row, err := s.q.CreateStatus(ctx, query.CreateStatusParams{
		UserID:   userID,
		Name:     in.Name,
		Color:    in.Color,
		Position: int32(position),
		IsSystem: false,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, validationf("a status named %q already exists", in.Name)
		}
		return nil, err
	}
	m := statusToModel(row)
	return &m, nil
}

type UpdateStatusInput struct {
	Name     *string `json:"name"`
	Color    *string `json:"color"`
	Position *int    `json:"position"`
}

func (s *StatusService) Update(ctx context.Context, userID, statusID uuid.UUID, in UpdateStatusInput) (*model.Status, error) {
	current, err := s.q.GetStatus(ctx, query.GetStatusParams{ID: statusID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	params := query.UpdateStatusParams{
		ID:       statusID,
		UserID:   userID,
		Name:     current.Name,
		Color:    current.Color,
		Position: current.Position,
	}
	if in.Name != nil {
		if *in.Name == "" {
			return nil, validationf("name cannot be empty")
		}
		if current.IsSystem && *in.Name != current.Name {
			return nil, ErrForbidden
		}
		params.Name = *in.Name
	}
	if in.Color != nil {
		if !hexColorRe.MatchString(*in.Color) {
			return nil, validationf("color must be a hex color like #aabbcc")
		}
		params.Color = *in.Color
	}
	if in.Position != nil {
		params.Position = int32(*in.Position)
	}
	row, err := s.q.UpdateStatus(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, validationf("a status named %q already exists", params.Name)
		}
		return nil, err
	}
	m := statusToModel(row)
	return &m, nil
}

// Delete enforces the spec's rules: system statuses are forbidden; statuses
// still in use return StatusInUseError unless confirm is set, in which case
// affected applications are moved to "Lost" (with history entries).
func (s *StatusService) Delete(ctx context.Context, userID, statusID uuid.UUID, confirm bool) error {
	status, err := s.q.GetStatus(ctx, query.GetStatusParams{ID: statusID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status.IsSystem {
		return ErrForbidden
	}

	count, err := s.q.CountApplicationsWithStatus(ctx, statusID)
	if err != nil {
		return err
	}
	if count > 0 && !confirm {
		lost, err := s.q.GetLostStatus(ctx, userID)
		if err != nil {
			return err
		}
		return &StatusInUseError{AffectedCount: count, LostStatusID: lost.ID}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	if count > 0 {
		lost, err := qtx.GetLostStatus(ctx, userID)
		if err != nil {
			return err
		}
		moved, err := qtx.ReassignApplicationsStatus(ctx, query.ReassignApplicationsStatusParams{
			NewStatusID: lost.ID,
			UserID:      userID,
			OldStatusID: statusID,
		})
		if err != nil {
			return err
		}
		for _, appID := range moved {
			if _, err := qtx.InsertStatusHistory(ctx, query.InsertStatusHistoryParams{
				ApplicationID: appID,
				StatusID:      &lost.ID,
				StatusName:    lost.Name,
			}); err != nil {
				return err
			}
		}
	}
	if _, err := qtx.DeleteStatus(ctx, query.DeleteStatusParams{ID: statusID, UserID: userID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func statusToModel(s query.Status) model.Status {
	return model.Status{
		ID:       s.ID,
		Name:     s.Name,
		Color:    s.Color,
		Position: int(s.Position),
		IsSystem: s.IsSystem,
	}
}
