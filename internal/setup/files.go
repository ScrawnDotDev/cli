package setup

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ScrawnDotDev/scrawn-cli/internal/apperr"
)

func GenerateHMACSecret() (string, error) {
	bytes := make([]byte, 48)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func patchUserIDType(target string, userIDType string) error {
	identifiersPath := filepath.Join(target, "src", "config", "identifiers.ts")
	contents, err := os.ReadFile(identifiersPath)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to read identifiers config", Detail: err.Error()}
	}

	re := regexp.MustCompile(`export const USER_ID_CONFIG = ID_CONFIGS\.(uuid|bigint|int);`)
	if !re.Match(contents) {
		return &apperr.CommandError{Summary: "failed to configure user ID type", Detail: "could not find the USER_ID_CONFIG assignment"}
	}

	replacement := fmt.Sprintf("export const USER_ID_CONFIG = ID_CONFIGS.%s;", userIDType)
	updated := re.ReplaceAllString(string(contents), replacement)

	if err := os.WriteFile(identifiersPath, []byte(updated), 0o644); err != nil {
		return &apperr.CommandError{Summary: "failed to write identifiers config", Detail: err.Error()}
	}

	return nil
}

func writeEnvFile(cfg Config) error {
	content := strings.Join([]string{
		fmt.Sprintf("HMAC_SECRET=%s", quoteEnvValue(cfg.HMACSecret)),
		fmt.Sprintf("DATABASE_URL=%s", quoteEnvValue(cfg.DatabaseURL)),
		fmt.Sprintf("CLICKHOUSE_URL=%s", quoteEnvValue(cfg.ClickhouseURL)),
		fmt.Sprintf("APP_URL=%s", quoteEnvValue(cfg.AppURL)),
		fmt.Sprintf("SENTRY_DSN=%s", quoteEnvValue(cfg.SentryDSN)),
		fmt.Sprintf("DODO_PAYMENTS_LIVE_API_KEY=%s", quoteEnvValue(cfg.DodoLiveAPIKey)),
		fmt.Sprintf("DODO_PAYMENTS_TEST_API_KEY=%s", quoteEnvValue(cfg.DodoTestAPIKey)),
		fmt.Sprintf("DODO_PAYMENTS_PRODUCT_ID=%s", quoteEnvValue(cfg.DodoProductID)),
		fmt.Sprintf("DODO_PAYMENTS_WEBHOOK_SECRET=%s", quoteEnvValue(cfg.DodoWebhookSecret)),
		fmt.Sprintf("REDIS_URL=%s", quoteEnvValue(cfg.RedisURL)),
		"",
	}, "\n")

	envPath := filepath.Join(cfg.TargetPath, ".env.local")
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		return &apperr.CommandError{Summary: "failed to write .env.local", Detail: err.Error()}
	}

	return nil
}

func quoteEnvValue(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return fmt.Sprintf("\"%s\"", escaped)
}
