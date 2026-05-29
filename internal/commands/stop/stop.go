package stopcmd

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
	cmd.Register("stop", func() cmd.Command { return &StopCommand{} })
}

var heading = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var success = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
var step = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))

type StopCommand struct{}

func (c *StopCommand) Name() string        { return "stop" }
func (c *StopCommand) Description() string { return "stop the Scrawn stack (preserves data)" }

func (c *StopCommand) Run(ctx *cmd.Context, args []string) error {
	flags := parseFlags(args)
	if flags == nil {
		return nil
	}
	if flags.help {
		printHelp()
		return nil
	}

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

	cmd := exec.Command("docker", "compose", "--env-file", "scrawn.env", "-f", composeFile, "--profile", "clickhouse", "down")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return &apperr.CommandError{
			Summary: "failed to stop containers",
			Detail:  err.Error(),
		}
	}

	fmt.Println(success.Render("✔"), "Scrawn stack stopped")
	fmt.Println()
	return nil
}

type stopFlags struct {
	help bool
}

func parseFlags(args []string) *stopFlags {
	flags := &stopFlags{}
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
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
	fmt.Println(heading.Render("Usage:") + " scrawn stop")
	fmt.Println()
	fmt.Println("Stop the Scrawn stack. Data is preserved.")
	fmt.Println("Run 'scrawn start' to bring it back up.")
	fmt.Println()
	fmt.Println(heading.Render("Flags:"))
	fmt.Println("  -h, --help  Show this help")
}

func checkDeps() {
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err != nil {
		fmt.Println(muted.Render("Docker Compose is required — install it from https://docs.docker.com/compose"))
		os.Exit(1)
	}
}
