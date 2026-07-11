package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/egoralekseychik/job-tracker-api/internal/config"
	"github.com/egoralekseychik/job-tracker-api/internal/db/query"
	"github.com/egoralekseychik/job-tracker-api/internal/model"
	"github.com/egoralekseychik/job-tracker-api/internal/token"
)

const bcryptCost = 12

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Statuses seeded for every new user.
var defaultStatuses = []struct {
	name     string
	color    string
	isSystem bool
}{
	{"Applied", "#3b82f6", false},
	{"Interview", "#f59e0b", false},
	{"Accepted", "#22c55e", false},
	{"Rejected", "#ef4444", false},
	{"Lost", "#6b7280", true},
}

type AuthService struct {
	pool   *pgxpool.Pool
	q      *query.Queries
	tokens *token.Manager
	cfg    *config.Config
}

func NewAuthService(pool *pgxpool.Pool, tokens *token.Manager, cfg *config.Config) *AuthService {
	return &AuthService{pool: pool, q: query.New(pool), tokens: tokens, cfg: cfg}
}

func (s *AuthService) Register(ctx context.Context, email, password, displayName, timezone string) (*model.AuthResponse, error) {
	if !emailRe.MatchString(email) {
		return nil, validationf("invalid email format")
	}
	if len(password) < 8 {
		return nil, validationf("password must be at least 8 characters")
	}
	if displayName == "" {
		return nil, validationf("display_name is required")
	}
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, validationf("invalid IANA timezone %q", timezone)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var user query.User
	// Retry calendar token generation on the (unlikely) unique collision.
	for attempt := 0; ; attempt++ {
		calToken, err := token.GenerateCalendarToken()
		if err != nil {
			return nil, fmt.Errorf("generate calendar token: %w", err)
		}
		user, err = s.registerTx(ctx, email, string(hash), displayName, timezone, calToken)
		if err == nil {
			break
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "users_email_key" {
				return nil, ErrEmailTaken
			}
			if pgErr.ConstraintName == "users_calendar_token_key" && attempt < 5 {
				continue
			}
		}
		return nil, fmt.Errorf("register: %w", err)
	}

	return s.issueTokens(ctx, user)
}

func (s *AuthService) registerTx(ctx context.Context, email, passwordHash, displayName, timezone, calToken string) (query.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return query.User{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := s.q.WithTx(tx)
	user, err := qtx.CreateUser(ctx, query.CreateUserParams{
		Email:         email,
		PasswordHash:  passwordHash,
		DisplayName:   displayName,
		Timezone:      timezone,
		CalendarToken: calToken,
	})
	if err != nil {
		return query.User{}, err
	}
	for i, st := range defaultStatuses {
		if _, err := qtx.CreateStatus(ctx, query.CreateStatusParams{
			UserID:   user.ID,
			Name:     st.name,
			Color:    st.color,
			Position: int32(i),
			IsSystem: st.isSystem,
		}); err != nil {
			return query.User{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return query.User{}, err
	}
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*model.AuthResponse, error) {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return s.issueTokens(ctx, user)
}

// Refresh rotates a refresh token: the old one is deleted and a new pair issued.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*model.AuthResponse, error) {
	hash := s.tokens.HashRefreshToken(refreshToken)
	row, err := s.q.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if time.Now().After(row.ExpiresAt) {
		_, _ = s.q.DeleteRefreshTokenByHash(ctx, hash)
		return nil, ErrUnauthorized
	}
	if _, err := s.q.DeleteRefreshTokenByHash(ctx, hash); err != nil {
		return nil, err
	}
	user, err := s.q.GetUserByID(ctx, row.UserID)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	_, err := s.q.DeleteRefreshTokenByHash(ctx, s.tokens.HashRefreshToken(refreshToken))
	return err
}

// ForgotPassword issues a reset token. It never reveals whether the email exists.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	reset, err := s.tokens.IssueResetToken(user.ID)
	if err != nil {
		return err
	}
	link := fmt.Sprintf("/reset-password?token=%s", reset)
	if s.cfg.AppBaseURL != "" {
		link = s.cfg.AppBaseURL + link
	}
	if s.cfg.SMTPConfigured() {
		if err := s.sendResetEmail(user.Email, link); err != nil {
			log.Printf("password reset email to %s failed: %v", user.Email, err)
			return fmt.Errorf("send reset email: %w", err)
		}
		return nil
	}
	log.Printf("password reset link for %s: %s", user.Email, link)
	return nil
}

func (s *AuthService) sendResetEmail(to, link string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Password reset\r\n\r\nReset your password: %s\r\n",
		s.cfg.SMTPFrom, to, link)
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
	return smtp.SendMail(addr, auth, s.cfg.SMTPFrom, []string{to}, []byte(msg))
}

func (s *AuthService) ResetPassword(ctx context.Context, resetToken, newPassword string) error {
	if len(newPassword) < 8 {
		return validationf("password must be at least 8 characters")
	}
	userID, err := s.tokens.ValidateResetToken(resetToken)
	if err != nil {
		return ErrUnauthorized
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}
	if err := s.q.UpdateUserPassword(ctx, query.UpdateUserPasswordParams{
		ID:           userID,
		PasswordHash: string(hash),
	}); err != nil {
		return err
	}
	// Invalidate all sessions after a password reset.
	return s.q.DeleteRefreshTokensForUser(ctx, userID)
}

func (s *AuthService) issueTokens(ctx context.Context, user query.User) (*model.AuthResponse, error) {
	access, err := s.tokens.IssueAccessToken(user.ID)
	if err != nil {
		return nil, err
	}
	refresh, hash, expiresAt, err := s.tokens.IssueRefreshToken()
	if err != nil {
		return nil, err
	}
	if err := s.q.CreateRefreshToken(ctx, query.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	}); err != nil {
		return nil, err
	}
	return &model.AuthResponse{
		User: UserToModel(user),
		AuthTokens: model.AuthTokens{
			AccessToken:  access,
			RefreshToken: refresh,
		},
	}, nil
}

// UserToModel converts a DB user row to the API model.
func UserToModel(u query.User) model.User {
	return model.User{
		ID:               u.ID,
		Email:            u.Email,
		DisplayName:      u.DisplayName,
		Timezone:         u.Timezone,
		ArchiveAfterDays: int(u.ArchiveAfterDays),
		CreatedAt:        u.CreatedAt,
		UpdatedAt:        u.UpdatedAt,
	}
}
