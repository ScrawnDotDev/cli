package startcmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

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

	if flags.stop {
		return runStop()
	}

	return runStart()
}

type startFlags struct {
	stop bool
	help bool
}

func parseFlags(args []string) *startFlags {
	flags := &startFlags{}

	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.BoolVar(&flags.help, "h", false, "help")
	fs.BoolVar(&flags.help, "help", false, "help")
	fs.BoolVar(&flags.stop, "stop", false, "stop containers")

	fs.SetOutput(os.NewFile(1, "/dev/null"))
	if err := fs.Parse(args); err != nil {
		return nil
	}

	if flags.help {
		printHelp()
		return nil
	}

	return flags
}

func printHelp() {
	fmt.Println()
	fmt.Println(heading.Render("Usage:") + " scrawn start [flags]")
	fmt.Println()
	fmt.Println("Run the Scrawn stack (postgres + server) via docker compose")
	fmt.Println()
	fmt.Println(heading.Render("Flags:"))
	fmt.Println("  --stop      Stop and clean up all containers")
	fmt.Println("  -h, --help  Show this help")
	fmt.Println()
	fmt.Println(heading.Render("Examples:"))
	fmt.Println("  scrawn start             # Start in background")
	fmt.Println("  scrawn start --stop      # Stop all containers")
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

	cmd := exec.Command("docker", "compose", "-f", composeFile, "--profile", "clickhouse", "up", "-d")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return &apperr.CommandError{
			Summary: "failed to start containers",
			Detail:  err.Error(),
		}
	}

	fmt.Println(success.Render("✔"), "Scrawn stack started in the background")
	fmt.Println(muted.Render("   gRPC: http://localhost:" + setup.GRPCPort))
	fmt.Println()
	return nil
}

func runStop() error {
	composeFile := setup.DockerComposeFileName

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		fmt.Println()
		fmt.Println(success.Render("✖"), "Need a scrawn docker compose.. run scrawn init first")
		fmt.Println()
		return nil
	}

	checkDeps()

	fmt.Println()
	fmt.Println(step.Render("==>"), "Stopping containers...")

	cmd := exec.Command("docker", "compose", "-f", composeFile, "--profile", "clickhouse", "down", "--volumes")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return &apperr.CommandError{
			Summary: "failed to stop containers",
			Detail:  err.Error(),
		}
	}

	fmt.Println(success.Render("✔"), "Scrawn stack stopped and cleaned up")
	fmt.Println()
	return nil
}

func checkDeps() {
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err != nil {
		fmt.Println(muted.Render("Docker Compose is required — install it from https://docs.docker.com/compose"))
		os.Exit(1)
	}
}
