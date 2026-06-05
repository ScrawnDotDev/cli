package startcmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	apperr "github.com/ScrawnDotDev/scrawn-cli/internal/apperr"
	"github.com/ScrawnDotDev/scrawn-cli/internal/cmd"
	"github.com/ScrawnDotDev/scrawn-cli/internal/setup"
)

func init() {
	cmd.Register("start", func() cmd.Command { return &StartCommand{} })
}

var heading = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var success = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
var step = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))

type StartCommand struct{}

func (c *StartCommand) Name() string     { return "start" }
func (c *StartCommand) Description() string { return "run the Scrawn stack via docker compose" }

func (c *StartCommand) Run(ctx *cmd.Context, args []string) error {
	flags := parseFlags(args)
	if flags == nil {
		return nil
	}
	if flags.help {
		printHelp()
		return nil
	}

	return runStart(flags.dev)
}

type startFlags struct {
	help bool
	dev  bool
}

func parseFlags(args []string) *startFlags {
	flags := &startFlags{}

	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.BoolVar(&flags.help, "h", false, "help")
	fs.BoolVar(&flags.help, "help", false, "help")
	fs.BoolVar(&flags.dev, "dev", false, "start infra only + run migrations")

	fs.SetOutput(os.NewFile(1, "/dev/null"))
	if err := fs.Parse(args); err != nil {
		return nil
	}

	return flags
}

func printHelp() {
	fmt.Println()
	fmt.Println(heading.Render("Usage:") + " scrawn start [flags]")
	fmt.Println()
	fmt.Println("Run the Scrawn stack (postgres + server + dashboard) via docker compose")
	fmt.Println()
	fmt.Println(heading.Render("Flags:"))
	fmt.Println("  -h, --help  Show this help")
	fmt.Println("  --dev       Start infra only (db + clickhouse), run migrations, then exit")
	fmt.Println()
	fmt.Println(heading.Render("Examples:"))
	fmt.Println("  scrawn start         # Start full stack")
	fmt.Println("  scrawn start --dev   # Start infra only + run migrations")
	fmt.Println("  scrawn stop          # Stop, preserve data")
	fmt.Println("  scrawn reset         # Stop, wipe volumes")
}

func runStart(dev bool) error {
	composeFile := setup.ScrawnComposeFile

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		fmt.Println()
		fmt.Println(success.Render("✖"), "Need a scrawn docker compose.. run scrawn init first")
		fmt.Println()
		return nil
	}

	checkDeps()

	fmt.Println()
	fmt.Println(step.Render("==>"), "Starting Scrawn stack...")

	profile := "production"
	if dev {
		profile = "clickhouse"
	}

	if err := composeRun(composeFile, "--profile", profile, "up", "-d"); err != nil {
		return &apperr.CommandError{
			Summary: "failed to start containers",
			Detail:  err.Error(),
		}
	}

	hmacSecret, scrawnKey := readEnvFile()

	if hmacSecret != "" && scrawnKey != "" {
		fmt.Println(step.Render("==>"), "Provisioning dashboard...")

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Wait for Postgres to accept connections
		for {
			_, err := execOnDB(composeFile, "pg_isready", "-U", "postgres")
			if err == nil {
				break
			}
			select {
			case <-ctx.Done():
				fmt.Println(muted.Render("   Could not create dashboard database — DB not ready in time"))
				goto keyDone
			default:
				time.Sleep(2 * time.Second)
			}
		}

		for {
			output, err := execOnDB(composeFile, "psql", "-U", "postgres", "-d", "postgres", "-c", "CREATE DATABASE dashboard")
			if err == nil {
				break
			}
			if strings.Contains(strings.ToLower(string(output)), "already exists") {
				break
			}
			select {
			case <-ctx.Done():
				fmt.Println(muted.Render("   Could not create dashboard database — DB not ready in time"))
				goto keyDone
			default:
				time.Sleep(2 * time.Second)
			}
		}

		fmt.Println(success.Render("✔"), "Dashboard database ready")

		if dev {
			fmt.Println(step.Render("==>"), "Running server migrations...")
			if err := composeRun(composeFile, "run", "--rm", "--entrypoint", "sh", "server", "-c", "for i in 1 2 3 4 5; do bunx drizzle-kit push --force && break; sleep 3; done"); err != nil {
				fmt.Println(muted.Render("   Server migration failed:"), err)
				goto keyDone
			}
			fmt.Println(success.Render("✔"), "Server schema up to date")
		}

		apiKeyHash := setup.HashAPIKey(scrawnKey, hmacSecret)
		insertSQL := fmt.Sprintf("INSERT INTO api_keys (id, name, key, role, created_at, expires_at, revoked, revoked_at) VALUES ('%s', 'Dashboard Key', '%s', 'dashboard', NOW(), NOW() + INTERVAL '365 days', false, NULL) ON CONFLICT DO NOTHING;",
			uuid.NewString(), apiKeyHash)

		for {
			output, err := execOnDB(composeFile, "psql", "-U", "postgres", "-d", "scrawn", "-c", insertSQL)
			if err == nil {
				fmt.Println(success.Render("✔"), "Dashboard API key provisioned")
				break
			}
			select {
			case <-ctx.Done():
				fmt.Println(muted.Render("   Could not provision dashboard key:"), string(output))
				goto keyDone
			default:
				time.Sleep(2 * time.Second)
			}
		}

		if dev {
			fmt.Println(step.Render("==>"), "Running dashboard migrations...")
			if err := composeRun(composeFile, "run", "--rm", "--entrypoint", "sh", "dashboard", "-c", "for i in 1 2 3 4 5; do bunx drizzle-kit push --force && break; sleep 3; done"); err != nil {
				fmt.Println(muted.Render("   Dashboard migration failed:"), err)
				goto keyDone
			}
			fmt.Println(success.Render("✔"), "Dashboard schema up to date")
		}
	}

keyDone:
	fmt.Println()
	if dev {
		fmt.Println(success.Render("✔"), "Scrawn dev stack ready")
		fmt.Println(muted.Render("   Postgres:  postgres://postgres:postgres@localhost:5432"))
		fmt.Println(muted.Render("   Dashboard DB: postgres://postgres:postgres@localhost:5432/dashboard"))
		fmt.Println(muted.Render("   Clickhouse: http://localhost:8123"))
	} else {
		fmt.Println(success.Render("✔"), "Scrawn stack started in the background")
		fmt.Println(muted.Render("   API: http://localhost:" + setup.EnvoyPort))
		fmt.Println(muted.Render("   Dashboard: http://localhost:3000"))
	}
	fmt.Println()
	return nil
}

func composeRun(composeFile string, args ...string) error {
	base := []string{"compose", "--env-file", setup.ScrawnEnvFile, "-f", composeFile}
	cmd := exec.Command("docker", append(base, args...)...)
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func execOnDB(composeFile string, args ...string) ([]byte, error) {
	base := []string{"compose", "--env-file", setup.ScrawnEnvFile, "-f", composeFile, "exec", "-T", "db"}
	cmd := exec.Command("docker", append(base, args...)...)
	cmd.Dir = "."
	return cmd.CombinedOutput()
}

func execOnDBStdin(composeFile string, stdin string, args ...string) ([]byte, error) {
	base := []string{"compose", "--env-file", setup.ScrawnEnvFile, "-f", composeFile, "exec", "-T", "db"}
	cmd := exec.Command("docker", append(base, args...)...)
	cmd.Dir = "."
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.CombinedOutput()
}

func readEnvFile() (string, string) {
	data, err := os.ReadFile(setup.ScrawnEnvFile)
	if err != nil {
		return "", ""
	}
	var hmacSecret, scrawnKey string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "HMAC_SECRET=") {
			hmacSecret = strings.TrimPrefix(line, "HMAC_SECRET=")
		} else if strings.HasPrefix(line, "SCRAWN_KEY=") {
			scrawnKey = strings.TrimPrefix(line, "SCRAWN_KEY=")
		}
	}
	return hmacSecret, scrawnKey
}

func checkDeps() {
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err != nil {
		fmt.Println(muted.Render("Docker Compose is required — install it from https://docs.docker.com/compose"))
		os.Exit(1)
	}
}
