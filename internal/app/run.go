package app

import (
	"fmt"

	"github.com/ScrawnDotDev/scrawn-cli/internal/cmd"
	"github.com/ScrawnDotDev/scrawn-cli/internal/ui"

	_ "github.com/ScrawnDotDev/scrawn-cli/internal/commands/init"
	_ "github.com/ScrawnDotDev/scrawn-cli/internal/commands/start"
	_ "github.com/ScrawnDotDev/scrawn-cli/internal/commands/stop"
	_ "github.com/ScrawnDotDev/scrawn-cli/internal/commands/reset"
	_ "github.com/ScrawnDotDev/scrawn-cli/internal/commands/tag"
)

var Version = "dev"

var heading = ui.Heading

func Run(args []string) error {
	if len(args) == 0 {
		return &cmd.CommandError{
			Summary: "missing command",
			Detail:  fmt.Sprintf("available commands: %s", cmdNames()),
		}
	}

	first := args[0]

	if first == "-h" || first == "--help" {
		return runHelp()
	}
	if first == "-v" || first == "--version" {
		return runVersion()
	}
	if first == "--help" && len(args) > 1 {
		return runSubcommandHelp(args[1])
	}

	name := args[0]
	command, ok := cmd.Get(name)
	if !ok {
		return &cmd.CommandError{
			Summary: "unknown command",
			Detail:  fmt.Sprintf("`%s` is not supported. available: %s", name, cmdNames()),
		}
	}

	ctx := &cmd.Context{}
	if err := command.Run(ctx, args[1:]); err != nil {
		return err
	}

	return nil
}

func runHelp() error {
	fmt.Println(heading.Render("Scrawn CLI"), Version)
	fmt.Println()
	fmt.Println(heading.Render("Usage:") + " scrawn <command> [flags]")
	fmt.Println()
	fmt.Println(heading.Render("Commands:"))
	for _, name := range cmd.Names() {
		c, _ := cmd.Get(name)
		fmt.Printf("  %s  %s\n", name, c.Description())
	}
	fmt.Println()
	fmt.Println(heading.Render("Flags:"))
	fmt.Println("  -h, --help     Show help")
	fmt.Println("  -v, --version  Show version")
	return nil
}

func runVersion() error {
	fmt.Println(Version)
	return nil
}

func runSubcommandHelp(name string) error {
	command, ok := cmd.Get(name)
	if !ok {
		return &cmd.CommandError{
			Summary: "unknown command",
			Detail:  fmt.Sprintf("`%s` is not a valid command", name),
		}
	}
	fmt.Printf("Usage: scrawn %s [flags]\n", name)
	fmt.Printf("\n%s\n", command.Description())
	return nil
}

func cmdNames() string {
	names := cmd.Names()
	if len(names) == 0 {
		return "(none)"
	}
	return fmt.Sprintf("%q", names[0])
}