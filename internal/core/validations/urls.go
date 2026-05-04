package validations

import (
	"errors"
	"net/url"
	"strings"
)

func ValidateDatabaseURL(input string) error {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return errors.New("DATABASE_URL is required")
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("use a valid PostgreSQL connection string")
	}
	if !strings.HasPrefix(u.Scheme, "postgres") {
		return errors.New("DATABASE_URL must start with postgres:// or postgresql://")
	}
	return nil
}

func ValidateRedisURL(input string) error {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return errors.New("REDIS_URL is required")
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" {
		return errors.New("use a valid Redis URL")
	}
	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return errors.New("REDIS_URL must start with redis:// or rediss://")
	}
	return nil
}

func ValidateTargetInput(input string) error {
	if strings.TrimSpace(input) == "" {
		return errors.New("enter a folder path or use .")
	}
	return nil
}