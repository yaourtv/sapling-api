package token

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestManager(accessTTL time.Duration) *Manager {
	return NewManager("access-secret", "refresh-secret", accessTTL, 720*time.Hour)
}

func TestAccessTokenRoundTrip(t *testing.T) {
	m := newTestManager(15 * time.Minute)
	userID := uuid.New()

	tok, err := m.IssueAccessToken(userID)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, err := m.ValidateAccessToken(tok)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got != userID {
		t.Errorf("got user %s, want %s", got, userID)
	}
}

func TestAccessTokenExpired(t *testing.T) {
	m := newTestManager(-time.Minute)
	tok, err := m.IssueAccessToken(uuid.New())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := m.ValidateAccessToken(tok); err != ErrExpiredToken {
		t.Errorf("got %v, want ErrExpiredToken", err)
	}
}

func TestAccessTokenWrongSecret(t *testing.T) {
	m := newTestManager(15 * time.Minute)
	other := NewManager("different-secret", "refresh-secret", 15*time.Minute, time.Hour)
	tok, _ := m.IssueAccessToken(uuid.New())
	if _, err := other.ValidateAccessToken(tok); err == nil {
		t.Error("token validated with wrong secret")
	}
}

func TestResetTokenNotUsableAsAccessToken(t *testing.T) {
	m := newTestManager(15 * time.Minute)
	userID := uuid.New()
	reset, err := m.IssueResetToken(userID, PasswordFingerprint("hash-of-the-old-password"))
	if err != nil {
		t.Fatalf("issue reset: %v", err)
	}
	if _, err := m.ValidateAccessToken(reset); err == nil {
		t.Error("reset token accepted as access token")
	}
	got, fingerprint, err := m.ValidateResetToken(reset)
	if err != nil {
		t.Fatalf("validate reset: %v", err)
	}
	if got != userID {
		t.Errorf("got %s, want %s", got, userID)
	}
	if fingerprint != PasswordFingerprint("hash-of-the-old-password") {
		t.Errorf("fingerprint round-trip failed: got %q", fingerprint)
	}

	access, _ := m.IssueAccessToken(userID)
	if _, _, err := m.ValidateResetToken(access); err == nil {
		t.Error("access token accepted as reset token")
	}
}

// The fingerprint is what makes a reset link single-use: once the password
// changes, the tag the token carries no longer matches the account.
func TestPasswordFingerprintChangesWithTheHash(t *testing.T) {
	before := PasswordFingerprint("$2a$12$oldhash")
	after := PasswordFingerprint("$2a$12$newhash")
	if before == after {
		t.Error("fingerprint did not change with the password hash")
	}
	if before != PasswordFingerprint("$2a$12$oldhash") {
		t.Error("fingerprint is not stable for the same hash")
	}
	if len(before) != 16 {
		t.Errorf("fingerprint length = %d, want 16", len(before))
	}
	if strings.Contains(before, "oldhash") {
		t.Error("fingerprint leaks the hash it was derived from")
	}
}

func TestRefreshTokenHashStable(t *testing.T) {
	m := newTestManager(15 * time.Minute)
	tok, hash, expires, err := m.IssueRefreshToken()
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if m.HashRefreshToken(tok) != hash {
		t.Error("hash not reproducible")
	}
	if !expires.After(time.Now().Add(719 * time.Hour)) {
		t.Error("expiry not honoring TTL")
	}
}

func TestGenerateCalendarToken(t *testing.T) {
	re := regexp.MustCompile(`^[A-Za-z0-9]{6}$`)
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok, err := GenerateCalendarToken()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if !re.MatchString(tok) {
			t.Fatalf("token %q does not match [A-Za-z0-9]{6}", tok)
		}
		seen[tok] = true
	}
	if len(seen) < 90 {
		t.Errorf("suspiciously many collisions: %d unique of 100", len(seen))
	}
}
