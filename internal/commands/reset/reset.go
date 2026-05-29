package resetcmd

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
	cmd.Register("reset", func() cmd.Command { return &ResetCommand{} })
}

var heading = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var success = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
var step = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))

type ResetCommand struct{}

func (c *ResetCommand) Name() string        { return "reset" }
func (c *ResetCommand) Description() string { return "reset the Scrawn stack (wipes volumes)" }

func (c *ResetCommand) Run(ctx *cmd.Context, args []string) error {
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
	fmt.Println(step.Render("==>"), "Resetting Scrawn stack...")
	fmt.Println(muted.Render("   This will delete all data!"))

	cmd := exec.Command("docker", "compose", "--env-file", "scrawn.env", "-f", composeFile, "--profile", "clickhouse", "down", "--volumes")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return &apperr.CommandError{
			Summary: "failed to reset containers",
			Detail:  err.Error(),
		}
	}

	fmt.Println(success.Render("✔"), "Scrawn stack reset")
	fmt.Println()
	return nil
}

type resetFlags struct {
	help bool
}

func parseFlags(args []string) *resetFlags {
	flags := &resetFlags{}
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
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
	fmt.Println(heading.Render("Usage:") + " scrawn reset")
	fmt.Println()
	fmt.Println("Reset the Scrawn stack. All volumes are wiped.")
	fmt.Println("Run 'scrawn init' then 'scrawn start' to set up fresh.")
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
