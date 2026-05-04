package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	apperr "github.com/ScrawnDotDev/scrawn-cli/internal/apperr"
)

var sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("221"))
var mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var subtleRuleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
var spinChars = []string{"·", "◦", "○", "◦"}

func runWithSpinner(label string, fn func() error) error {
	fmt.Printf("%s ", label)
	var lastIdx int
	ticker := time.NewTicker(100 * time.Millisecond)
	done := make(chan error, 1)

	go func() {
		done <- fn()
	}()

	for {
		select {
		case err := <-done:
			ticker.Stop()
			fmt.Print("\r" + strings.Repeat(" ", len(label)+3) + "\r")
			if err != nil {
				fmt.Printf("%s %s\n", failureLabel(), err.Error())
				return err
			}
			return nil
		case <-ticker.C:
			fmt.Printf("\r%s %s", spinnerStyle.Render(spinChars[lastIdx]), label)
			lastIdx = (lastIdx + 1) % len(spinChars)
		}
	}
}

func failureLabel() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("error")
}

func AvailablePackageManagers() ([]string, error) {
	options := []string{}
	for _, candidate := range []string{"bun", "npm"} {
		if _, err := exec.LookPath(candidate); err == nil {
			options = append(options, candidate)
		}
	}

	if len(options) == 0 {
		return nil, &apperr.CommandError{
			Summary: "no supported package manager found",
			Detail:  "install Bun or npm and run the command again",
		}
	}

	return options, nil
}

func ResolveTargetPath(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		trimmed = "."
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", &apperr.CommandError{Summary: "invalid target path", Detail: err.Error()}
	}
	return absolute, nil
}

func EnsureTargetIsSafe(target string) error {
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if mkErr := os.MkdirAll(target, 0o755); mkErr != nil {
				return &apperr.CommandError{Summary: "failed to create target directory", Detail: mkErr.Error()}
			}
			return nil
		}
		return &apperr.CommandError{Summary: "failed to inspect target directory", Detail: err.Error()}
	}

	if !info.IsDir() {
		return &apperr.CommandError{Summary: "invalid target", Detail: "the target path exists and is not a directory"}
	}

	entries, readErr := os.ReadDir(target)
	if readErr != nil {
		return &apperr.CommandError{Summary: "failed to inspect target directory", Detail: readErr.Error()}
	}
	if len(entries) > 0 {
		return &apperr.CommandError{
			Summary: "target directory is not empty",
			Detail:  fmt.Sprintf("choose an empty folder or a new path instead of %s", target),
		}
	}

	return nil
}

func SetupServer(cfg Config, step func(string), markOK func(string, string)) (Result, error) {
	fmt.Println(sectionStyle.Render("Scrawn CLI"))
	fmt.Println(mutedStyle.Render("> init server"))
	fmt.Println(subtleRuleStyle.Render(strings.Repeat("-", 72)))

	if err := EnsureTargetIsSafe(cfg.TargetPath); err != nil {
		return Result{}, err
	}

	step("Pulling the latest Scrawn backend")
	if err := scaffoldRepo(cfg.TargetPath); err != nil {
		return Result{}, err
	}
	markOK("Scaffold downloaded", cfg.TargetPath)

	step("Switching user ID configuration")
	if err := patchUserIDType(cfg.TargetPath, cfg.UserIDType); err != nil {
		return Result{}, err
	}
	markOK("Configured user ID type", cfg.UserIDType)

	step("Writing environment configuration")
	if err := writeEnvFile(cfg); err != nil {
		return Result{}, err
	}
	markOK("Saved .env.local", filepath.Join(cfg.TargetPath, ".env.local"))

	step(fmt.Sprintf("Installing dependencies with %s", cfg.PackageManager))
	if err := runWithSpinner("Installing dependencies", func() error {
		return installDependencies(cfg.TargetPath, cfg.PackageManager)
	}); err != nil {
		return Result{}, err
	}
	markOK("Dependencies installed", cfg.PackageManager)

	step("Running Drizzle migrations")
	if err := runWithSpinner("Running migrations", func() error {
		return runDrizzlePush(cfg.TargetPath, cfg.PackageManager)
	}); err != nil {
		return Result{}, err
	}
	markOK("Database schema is ready", "drizzle-kit push")

	step("Generating and inserting dashboard API key")
	apiKeyName, apiKeyPlaintext, err := insertDashboardAPIKey(cfg.DatabaseURL, cfg.HMACSecret)
	if err != nil {
		return Result{}, err
	}
	markOK("Dashboard API key inserted", apiKeyName)

	step("Booting the server and checking health")
	if err := smokeTestServer(cfg.TargetPath, cfg.PackageManager); err != nil {
		return Result{}, err
	}
	markOK("Server responded successfully", DefaultHTTPURL)

	return Result{
		TargetPath: cfg.TargetPath,
		APIKey:     apiKeyPlaintext,
		APIKeyName: apiKeyName,
		UsedPM:     cfg.PackageManager,
	}, nil
}

func installDependencies(target string, packageManager string) error {
	var command []string
	switch packageManager {
	case "bun":
		command = []string{"bun", "install"}
	case "npm":
		command = []string{"npm", "install"}
	default:
		return &apperr.CommandError{Summary: "unsupported package manager", Detail: packageManager}
	}

	if _, err := runCommand(target, 10*time.Minute, command[0], command[1:]...); err != nil {
		return &apperr.CommandError{Summary: "dependency installation failed", Detail: err.Error()}
	}
	return nil
}

func runDrizzlePush(target string, packageManager string) error {
	var command []string
	switch packageManager {
	case "bun":
		command = []string{"bunx", "drizzle-kit", "push"}
	case "npm":
		command = []string{"npx", "drizzle-kit", "push"}
	default:
		return &apperr.CommandError{Summary: "unsupported package manager", Detail: packageManager}
	}

	if _, err := runCommand(target, 5*time.Minute, command[0], command[1:]...); err != nil {
		return &apperr.CommandError{Summary: "database migration failed", Detail: err.Error()}
	}
	return nil
}

func runCommand(dir string, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	combined := collectLogs(stdout, stderr)
	if ctx.Err() == context.DeadlineExceeded {
		return combined, &apperr.CommandError{Summary: fmt.Sprintf("command timed out: %s %s", name, strings.Join(args, " ")), Detail: combined}
	}
	if err != nil {
		if combined == "" {
			combined = err.Error()
		}
		return combined, &apperr.CommandError{Summary: fmt.Sprintf("command failed: %s %s", name, strings.Join(args, " ")), Detail: combined}
	}

	return combined, nil
}

func collectLogs(stdout *bytes.Buffer, stderr *bytes.Buffer) string {
	parts := []string{}
	if trimmed := strings.TrimSpace(stdout.String()); trimmed != "" {
		parts = append(parts, "stdout:\n"+trimmed)
	}
	if trimmed := strings.TrimSpace(stderr.String()); trimmed != "" {
		parts = append(parts, "stderr:\n"+trimmed)
	}
	return strings.Join(parts, "\n\n")
}
