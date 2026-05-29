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

	return runStart()
}

type startFlags struct {
	help bool
}

func parseFlags(args []string) *startFlags {
	flags := &startFlags{}

	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.BoolVar(&flags.help, "h", false, "help")
	fs.BoolVar(&flags.help, "help", false, "help")

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
	fmt.Println()
	fmt.Println(heading.Render("Examples:"))
	fmt.Println("  scrawn start         # Start in background")
	fmt.Println("  scrawn stop          # Stop, preserve data")
	fmt.Println("  scrawn reset         # Stop, wipe volumes")
}

func runStart() error {
	composeFile := setup.DockerComposeFileName

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		fmt.Println()
		fmt.Println(success.Render("✖"), "Need a scrawn docker compose.. run scrawn init first")
		fmt.Println()
		return nil
	}

	checkDeps()

	fmt.Println()
	fmt.Println(step.Render("==>"), "Starting Scrawn stack...")

	cmd := exec.Command("docker", "compose", "--env-file", "scrawn.env", "-f", composeFile, "--profile", "clickhouse", "up", "-d")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
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

		for {
			dbErr := setup.EnsureDashboardDB("postgres://postgres:postgres@localhost:5432/postgres")
			if dbErr == nil {
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

		for {
			err := setup.InsertDashboardKey("postgres://postgres:postgres@localhost:5432/scrawn", hmacSecret, scrawnKey)
			if err == nil {
				fmt.Println(success.Render("✔"), "Dashboard API key provisioned")
				break
			}
			select {
			case <-ctx.Done():
				fmt.Println(muted.Render("   Could not provision dashboard key — DB not ready in time"))
				fmt.Println(muted.Render("   Run 'docker compose exec db pg_isready -U postgres' to check"))
				goto keyDone
			default:
				time.Sleep(2 * time.Second)
			}
		}
	}

keyDone:
	fmt.Println()
	fmt.Println(success.Render("✔"), "Scrawn stack started in the background")
	fmt.Println(muted.Render("   gRPC:  http://localhost:" + setup.GRPCPort))
	fmt.Println(muted.Render("   HTTP:  http://localhost:" + setup.HTTPPort))
	fmt.Println()
	return nil
}

func readEnvFile() (string, string) {
	data, err := os.ReadFile("scrawn.env")
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
