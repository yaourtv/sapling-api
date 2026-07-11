package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const resetPurpose = "password_reset"

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

// Manager issues and validates the various tokens used by the API.
type Manager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewManager(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

// IssueAccessToken returns a signed JWT with sub/exp/iat claims.
func (m *Manager) IssueAccessToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"iat": now.Unix(),
		"exp": now.Add(m.accessTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.accessSecret)
}

// ValidateAccessToken parses the JWT and returns the user ID from the sub claim.
func (m *Manager) ValidateAccessToken(tokenString string) (uuid.UUID, error) {
	return m.validateJWT(tokenString, m.accessSecret, "")
}

// IssueRefreshToken returns an opaque random token and the hash to store.
func (m *Manager) IssueRefreshToken() (token string, hash string, expiresAt time.Time, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", time.Time{}, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, m.HashRefreshToken(token), time.Now().Add(m.refreshTTL), nil
}

// HashRefreshToken derives the storable HMAC-SHA256 hash of an opaque refresh token.
func (m *Manager) HashRefreshToken(token string) string {
	mac := hmac.New(sha256.New, m.refreshSecret)
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

// RefreshTTL exposes the configured refresh token lifetime (for cookies).
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// IssueResetToken returns a short-lived JWT for password reset links.
func (m *Manager) IssueResetToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":     userID.String(),
		"iat":     now.Unix(),
		"exp":     now.Add(time.Hour).Unix(),
		"purpose": resetPurpose,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.accessSecret)
}

// ValidateResetToken parses a reset JWT and returns the user ID.
func (m *Manager) ValidateResetToken(tokenString string) (uuid.UUID, error) {
	return m.validateJWT(tokenString, m.accessSecret, resetPurpose)
}

func (m *Manager) validateJWT(tokenString string, secret []byte, purpose string) (uuid.UUID, error) {
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return uuid.Nil, ErrExpiredToken
		}
		return uuid.Nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return uuid.Nil, ErrInvalidToken
	}
	if purpose != "" {
		if p, _ := claims["purpose"].(string); p != purpose {
			return uuid.Nil, ErrInvalidToken
		}
	} else if _, hasPurpose := claims["purpose"]; hasPurpose {
		// A reset token must never pass as an access token.
		return uuid.Nil, ErrInvalidToken
	}
	sub, _ := claims["sub"].(string)
	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return userID, nil
}

const calendarTokenAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// GenerateCalendarToken returns a random 6-char [A-Za-z0-9] string.
func GenerateCalendarToken() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 6)
	for i, b := range buf {
		out[i] = calendarTokenAlphabet[int(b)%len(calendarTokenAlphabet)]
	}
	return string(out), nil
}
