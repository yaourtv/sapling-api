package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	DatabaseURL        string
	AccessTokenSecret  string
	RefreshTokenSecret string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	CORSOrigin         string
	Port               string

	SMTPHost   string
	SMTPPort   int
	SMTPUser   string
	SMTPPass   string
	SMTPFrom   string
	AppBaseURL string
}

// Load reads configuration from the environment. Required variables missing
// cause an error listing every missing key.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		AccessTokenSecret:  os.Getenv("ACCESS_TOKEN_SECRET"),
		RefreshTokenSecret: os.Getenv("REFRESH_TOKEN_SECRET"),
		CORSOrigin:         os.Getenv("CORS_ORIGIN"),
		Port:               getenvDefault("PORT", "8080"),
		SMTPHost:           os.Getenv("SMTP_HOST"),
		SMTPUser:           os.Getenv("SMTP_USER"),
		SMTPPass:           os.Getenv("SMTP_PASS"),
		SMTPFrom:           os.Getenv("SMTP_FROM"),
		AppBaseURL:         os.Getenv("APP_BASE_URL"),
	}

	var missing []string
	for _, kv := range []struct{ key, val string }{
		{"DATABASE_URL", cfg.DatabaseURL},
		{"ACCESS_TOKEN_SECRET", cfg.AccessTokenSecret},
		{"REFRESH_TOKEN_SECRET", cfg.RefreshTokenSecret},
		{"CORS_ORIGIN", cfg.CORSOrigin},
	} {
		if kv.val == "" {
			missing = append(missing, kv.key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	var err error
	if cfg.AccessTokenTTL, err = parseDuration("ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return nil, err
	}
	if cfg.RefreshTokenTTL, err = parseDuration("REFRESH_TOKEN_TTL", 720*time.Hour); err != nil {
		return nil, err
	}

	smtpPort := getenvDefault("SMTP_PORT", "587")
	if cfg.SMTPPort, err = strconv.Atoi(smtpPort); err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT %q: %w", smtpPort, err)
	}

	return cfg, nil
}

// SMTPConfigured reports whether email delivery is enabled.
func (c *Config) SMTPConfigured() bool {
	return c.SMTPHost != ""
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return d, nil
}
