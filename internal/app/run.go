package app

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/ScrawnDotDev/scrawn-cli/internal/apperr"
	"github.com/ScrawnDotDev/scrawn-cli/internal/setup"
	"github.com/ScrawnDotDev/scrawn-cli/internal/ui"
)

func Run(args []string) error {
	if len(args) == 0 {
		return &apperr.CommandError{
			Summary: "missing command",
			Detail:  "use `scrawn init`, `scrawn init server`, or `scrawn init dashboard`",
		}
	}

	switch args[0] {
	case "init":
		return handleInit(args[1:])
	default:
		return &apperr.CommandError{
			Summary: "unknown command",
			Detail:  fmt.Sprintf("`%s` is not supported yet", args[0]),
		}
	}
}

func handleInit(args []string) error {
	if len(args) == 0 {
		cfg, err := collectConfig("all", "", "init server")
		if err != nil {
			return err
		}
		return setupServerAndDashboard(cfg)
	}

	kind := strings.ToLower(strings.TrimSpace(args[0]))
	folderArg := ""
	if len(args) > 1 {
		folderArg = args[1]
	}

	switch kind {
	case "dashboard":
		return handleDashboardOnly(folderArg)
	case "server":
		cfg, err := collectConfig(kind, folderArg, "init server")
		if err != nil {
			return err
		}

		result, err := setup.SetupServer(cfg, ui.Step, ui.MarkOK)
		if err != nil {
			return err
		}

		ui.RenderSuccess(result, kind)
		return nil
	case "all":
		cfg, err := collectConfig(kind, folderArg, "init server")
		if err != nil {
			return err
		}
		return setupServerAndDashboard(cfg)
	default:
		return &apperr.CommandError{
			Summary: "invalid init target",
			Detail:  "use `server`, `dashboard`, or leave empty for both",
		}
	}
}

func setupServerAndDashboard(cfg setup.Config) error {
	result, err := setup.SetupServer(cfg, ui.Step, ui.MarkOK)
	if err != nil {
		return err
	}

	ui.RenderSuccess(result, "server")
	ui.RenderDashboardStub(result.TargetPath)
	return nil
}

func handleDashboardOnly(folder string) error {
	target := strings.TrimSpace(folder)
	if target == "" {
		target = "."
	}

	ui.RenderDashboardIntent(target)
	return nil
}

func collectConfig(kind string, folderArg string, wizardTitle string) (setup.Config, error) {
	pmOptions, err := setup.AvailablePackageManagers()
	if err != nil {
		return setup.Config{}, err
	}

	cfg := setup.Config{
		Kind:        kind,
		TargetInput: strings.TrimSpace(folderArg),
		UserIDType:  "uuid",
	}
	if cfg.TargetInput == "" {
		cfg.TargetInput = "./scrawn-server"
	}

	defaultPM := pmOptions[0]
	if contains(pmOptions, "bun") {
		defaultPM = "bun"
	}

	values, err := ui.RunWizard(
		wizardTitle,
		"",
		[]ui.WizardField{
			{Key: "packageManager", Label: "Package Manager", Description: "Which package manager do you want to use?", Type: ui.FieldSelect, Options: pmOptions, DefaultValue: defaultPM},
			{Key: "targetPath", Label: "Server Directory", Description: "Where should the server be created?", Type: ui.FieldInput, DefaultValue: cfg.TargetInput, Validate: validateTargetInput},
			{Key: "userIdType", Label: "User ID Type", Description: "What type of user IDs does your app use?", Type: ui.FieldSelect, Options: []string{"uuid", "bigint", "int"}, DefaultValue: cfg.UserIDType},
			{Key: "hmacSecret", Label: "HMAC Secret", Description: "Leave empty to auto-generate a secure key", Type: ui.FieldInput, AllowBlank: true},
			{Key: "databaseURL", Label: "PostgreSQL", Description: "What's your DATABASE_URL connection string?", Type: ui.FieldInput, DefaultValue: "postgresql://user:password@host:5432/scrawn", Validate: validateDatabaseURL},
			{Key: "redisURL", Label: "Redis", Description: "What's your REDIS_URL connection string?", Type: ui.FieldInput, DefaultValue: "redis://localhost:6379", Validate: validateRedisURL},
			{Key: "lemonAPIKey", Label: "Lemon Squeezy API Key", Description: "Optional - for payment processing", Type: ui.FieldInput, AllowBlank: true},
			{Key: "lemonStoreID", Label: "Lemon Squeezy Store ID", Description: "Optional", Type: ui.FieldInput, AllowBlank: true},
			{Key: "lemonVariantID", Label: "Lemon Squeezy Variant ID", Description: "Optional", Type: ui.FieldInput, AllowBlank: true},
			{Key: "lemonWebhookSecret", Label: "Lemon Squeezy Webhook Secret", Description: "Optional", Type: ui.FieldInput, AllowBlank: true},
		},
	)
	if err != nil {
		return setup.Config{}, promptError(err)
	}

	cfg.PackageManager = strings.TrimSpace(values["packageManager"])
	cfg.TargetInput = strings.TrimSpace(values["targetPath"])
	cfg.UserIDType = strings.TrimSpace(values["userIdType"])
	cfg.HMACSecret = strings.TrimSpace(values["hmacSecret"])
	if cfg.HMACSecret == "" {
		generated, genErr := setup.GenerateHMACSecret()
		if genErr != nil {
			return setup.Config{}, &apperr.CommandError{Summary: "failed to generate HMAC secret", Detail: genErr.Error()}
		}
		cfg.HMACSecret = generated
	}
	cfg.DatabaseURL = strings.TrimSpace(values["databaseURL"])
	cfg.RedisURL = strings.TrimSpace(values["redisURL"])
	cfg.LemonSqueezyAPIKey = strings.TrimSpace(values["lemonAPIKey"])
	cfg.LemonSqueezyStoreID = strings.TrimSpace(values["lemonStoreID"])
	cfg.LemonSqueezyVariantID = strings.TrimSpace(values["lemonVariantID"])
	cfg.LemonSqueezyWebhookSecret = strings.TrimSpace(values["lemonWebhookSecret"])

	resolved, err := setup.ResolveTargetPath(cfg.TargetInput)
	if err != nil {
		return setup.Config{}, err
	}
	cfg.TargetPath = resolved

	return cfg, nil
}

func promptError(err error) error {
	if errors.Is(err, ui.ErrPromptInterrupted) || errors.Is(err, io.EOF) {
		return &apperr.CommandError{Summary: "setup cancelled", Detail: "the prompt session was interrupted"}
	}
	return err
}

func validateTargetInput(input string) error {
	if strings.TrimSpace(input) == "" {
		return errors.New("enter a folder path or use .")
	}
	return nil
}

func validateDatabaseURL(input string) error {
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

func validateRedisURL(input string) error {
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

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
