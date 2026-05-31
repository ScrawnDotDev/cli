package setup

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/ScrawnDotDev/scrawn-cli/internal/apperr"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func insertDashboardAPIKey(databaseURL string, hmacSecret string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return "", "", &apperr.CommandError{Summary: "failed to connect to Postgres", Detail: err.Error()}
	}
	defer conn.Close(ctx)

	baseName := "Dashboard Key"
	for attempt := 0; attempt < 3; attempt++ {
		name := baseName
		if attempt > 0 {
			name = fmt.Sprintf("%s %d", baseName, time.Now().UnixNano())
		}

		apiKey, err := generateDashboardAPIKey()
		if err != nil {
			return "", "", &apperr.CommandError{Summary: "failed to generate API key", Detail: err.Error()}
		}

		apiKeyHash := HashAPIKey(apiKey, hmacSecret)
		createdAt := time.Now().UTC()
		expiresAt := createdAt.Add(365 * 24 * time.Hour)
		role := "dashboard"

		_, execErr := conn.Exec(ctx,
			`INSERT INTO api_keys (id, name, key, role, created_at, expires_at, revoked, revoked_at)
			 VALUES ($1, $2, $3, $4, $5, $6, false, NULL)`,
			uuid.NewString(), name, apiKeyHash, role, createdAt, expiresAt,
		)
		if execErr == nil {
			return name, apiKey, nil
		}

		lower := strings.ToLower(execErr.Error())
		if strings.Contains(lower, "duplicate") || strings.Contains(lower, "unique") {
			continue
		}

		return "", "", &apperr.CommandError{Summary: "failed to insert dashboard API key", Detail: execErr.Error()}
	}

	return "", "", &apperr.CommandError{Summary: "failed to insert dashboard API key", Detail: "could not generate a unique key record after multiple attempts"}
}

func generateDashboardAPIKey() (string, error) {
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(randomBytes)
	replacer := strings.NewReplacer("+", "a", "/", "b", "=", "c")
	normalized := replacer.Replace(encoded)
	if len(normalized) > 32 {
		normalized = normalized[:32]
	}

	return "scrn_dash_" + normalized, nil
}

func EnsureDashboardDB(databaseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to connect to Postgres", Detail: err.Error()}
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, "CREATE DATABASE dashboard")
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "already exists") || strings.Contains(lower, "duplicate_database") {
			return nil
		}
		return &apperr.CommandError{Summary: "failed to create dashboard database", Detail: err.Error()}
	}

	return nil
}

func InsertDashboardKey(databaseURL string, hmacSecret string, apiKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to connect to Postgres", Detail: err.Error()}
	}
	defer conn.Close(ctx)

	apiKeyHash := HashAPIKey(apiKey, hmacSecret)
	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(365 * 24 * time.Hour)

	_, execErr := conn.Exec(ctx,
		`INSERT INTO api_keys (id, name, key, role, created_at, expires_at, revoked, revoked_at)
		 VALUES ($1, $2, $3, $4, $5, $6, false, NULL)
		 ON CONFLICT DO NOTHING`,
		uuid.NewString(), "Dashboard Key", apiKeyHash, "dashboard", createdAt, expiresAt,
	)
	if execErr != nil {
		return &apperr.CommandError{Summary: "failed to insert dashboard API key", Detail: execErr.Error()}
	}

	return nil
}

func HashAPIKey(apiKey string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(apiKey))
	return fmt.Sprintf("%x", h.Sum(nil))
}
