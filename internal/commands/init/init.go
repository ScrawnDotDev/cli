package initcmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ScrawnDotDev/scrawn-cli/internal/cmd"
	"github.com/ScrawnDotDev/scrawn-cli/internal/setup"
	"github.com/ScrawnDotDev/scrawn-cli/internal/ui"
)

var registry = make(map[string]func() cmd.Command)

func init() {
	cmd.Register("init", func() cmd.Command { return &InitCommand{} })
	registry["server"] = func() cmd.Command { return &ServerCommand{} }
	registry["dashboard"] = func() cmd.Command { return &DashboardCommand{} }
}

var heading = ui.Heading

type InitCommand struct{}

func (c *InitCommand) Name() string     { return "init" }
func (c *InitCommand) Description() string { return "initialize a new project (server, dashboard)" }

func (c *InitCommand) Run(ctx *cmd.Context, args []string) error {
	if len(args) == 0 {
		return runInteractive(ctx)
	}

	first := args[0]
	if first == "-h" || first == "--help" {
		runInitHelp()
		return nil
	}

	sub := strings.ToLower(strings.TrimSpace(first))
	subCmd, ok := registry[sub]
	if !ok {
		return &cmd.CommandError{
			Summary: "invalid init target",
			Detail:  "use `server`, `dashboard`, or leave empty for both",
		}
	}

	return subCmd().Run(ctx, args[1:])
}

func runInitHelp() {
	fmt.Println()
	fmt.Println(heading.Render("Usage:") + " scrawn init [server|dashboard] [flags]")
	fmt.Println()
	fmt.Println("Initialize a new Scrawn project")
	fmt.Println()
	fmt.Println(heading.Render("Subcommands:"))
	fmt.Println("  server     Initialize a new Scrawn server")
	fmt.Println("  dashboard Initialize a new Scrawn dashboard")
	fmt.Println()
	fmt.Println(heading.Render("Flags:"))
	fmt.Println("  -h, --help   Show this help")
	fmt.Println()
	fmt.Println(heading.Render("Examples:"))
	fmt.Println("  scrawn init                Interactive wizard")
	fmt.Println("  scrawn init server        Interactive server setup")
	fmt.Println("  scrawn init server -d pg://... -r redis://...  Non-interactive")
	fmt.Println()
	fmt.Println("Use 'scrawn init <subcommand> -h' for more information.")
}

type ServerCommand struct{}

func (c *ServerCommand) Name() string     { return "server" }
func (c *ServerCommand) Description() string { return "initialize a new Scrawn server" }

type serverFlags struct {
	pkgManager      string
	path           string
	userIdType     string
	dbUrl           string
	redisUrl        string
	lemonApiKey     string
	lemonStoreId   string
	lemonVariantId  string
	lemonWebhookSec string
	noInteractive  bool
	help           bool
}

func (c *ServerCommand) Run(ctx *cmd.Context, args []string) error {
	flags := parseServerFlags(args)
	if flags == nil {
		return nil
	}
	if flags.help {
		serverHelp()
		return nil
	}

	cfg, err := buildServerConfig(flags)
	if err != nil {
		return err
	}

	if cfg == nil {
		return nil
	}

	result, err := setup.SetupServer(*cfg, ui.Step, ui.MarkOK)
	if err != nil {
		return err
	}

	ui.RenderSuccess(result, "server")
	return nil
}

func serverHelp() {
	fmt.Println()
	fmt.Println(heading.Render("Usage:") + " scrawn init server [flags]")
	fmt.Println()
	fmt.Println("Initialize a new Scrawn server")
	fmt.Println()
	fmt.Println(heading.Render("Flags:"))
	fmt.Println("  --package-manager, --pkg  Package manager (bun, npm)")
	fmt.Println("  -p, --path              Server directory (default: ./scrawn-server)")
	fmt.Println("  --user-id-type          User ID type (uuid, bigint, int)")
	fmt.Println("  -d, --db-url            PostgreSQL connection string")
	fmt.Println("  -r, --redis-url        Redis connection string")
	fmt.Println("  --lemon-api-key        Lemon Squeezy API key (optional)")
	fmt.Println("  --lemon-store-id       Lemon Squeezy store ID (optional)")
	fmt.Println("  --lemon-variant-id    Lemon Squeezy variant ID (optional)")
	fmt.Println("  --lemon-webhook-secret  Lemon Squeezy webhook secret (optional)")
	fmt.Println("  --no-interactive     Exit with error if required flags missing")
	fmt.Println("  -h, --help            Show this help")
	fmt.Println()
	fmt.Println(heading.Render("Examples:"))
	fmt.Println("  scrawn init server                       Interactive wizard")
	fmt.Println("  scrawn init server -d pg://... -r redis://...  Non-interactive")
	fmt.Println("  scrawn init server --pkg npm              Use npm")
}

func dashboardHelp() {
	fmt.Println()
	fmt.Println(heading.Render("Usage:") + " scrawn init dashboard [path]")
	fmt.Println()
	fmt.Println("Initialize a new Scrawn dashboard")
	fmt.Println()
	fmt.Println(heading.Render("Arguments:"))
	fmt.Println("  path    Target directory (default: current directory)")
	fmt.Println()
	fmt.Println(heading.Render("Flags:"))
	fmt.Println("  -h, --help   Show this help")
}

func parseServerFlags(args []string) *serverFlags {
	flags := &serverFlags{}

	fs := flag.NewFlagSet("init server", flag.ContinueOnError)
	fs.BoolVar(&flags.help, "h", false, "help")
	fs.BoolVar(&flags.help, "help", false, "help")
	fs.StringVar(&flags.pkgManager, "package-manager", "", "pkg")
	fs.StringVar(&flags.pkgManager, "pkg", "", "pkg")
	fs.StringVar(&flags.path, "p", "", "path")
	fs.StringVar(&flags.path, "path", "", "path")
	fs.StringVar(&flags.userIdType, "user-id-type", "", "user id type")
	fs.StringVar(&flags.dbUrl, "d", "", "db url")
	fs.StringVar(&flags.dbUrl, "db-url", "", "db url")
	fs.StringVar(&flags.redisUrl, "r", "", "redis url")
	fs.StringVar(&flags.redisUrl, "redis-url", "", "redis url")
	fs.StringVar(&flags.lemonApiKey, "lemon-api-key", "", "lemon api key")
	fs.StringVar(&flags.lemonStoreId, "lemon-store-id", "", "lemon store id")
	fs.StringVar(&flags.lemonVariantId, "lemon-variant-id", "", "lemon variant id")
	fs.StringVar(&flags.lemonWebhookSec, "lemon-webhook-secret", "", "lemon webhook secret")
	fs.BoolVar(&flags.noInteractive, "no-interactive", false, "non-interactive")

	fs.SetOutput(os.NewFile(1, "/dev/null"))
	if err := fs.Parse(args); err != nil {
		return nil
	}

	if flags.help {
		return flags
	}

	return flags
}

func buildServerConfig(flags *serverFlags) (*setup.Config, error) {
	needsWizard := false

	if flags.noInteractive {
		if flags.dbUrl == "" {
			return nil, &cmd.CommandError{
				Summary: "missing required flag",
				Detail:  "--db-url is required (use --no-interactive to require all flags)",
			}
		}
		if flags.redisUrl == "" {
			return nil, &cmd.CommandError{
				Summary: "missing required flag",
				Detail:  "--redis-url is required (use --no-interactive to require all flags)",
			}
		}
	} else {
		if flags.dbUrl == "" || flags.redisUrl == "" {
			needsWizard = true
		}
	}

	if needsWizard {
		return collectServerConfigWizard(flags)
	}

	cfg := &setup.Config{
		Kind:        "server",
		TargetInput: flags.path,
	}

	if cfg.TargetInput == "" {
		cfg.TargetInput = "./scrawn-server"
	}
	if flags.pkgManager != "" {
		cfg.PackageManager = flags.pkgManager
	} else {
		cfg.PackageManager = "bun"
	}
	if flags.userIdType != "" {
		cfg.UserIDType = flags.userIdType
	} else {
		cfg.UserIDType = "uuid"
	}
	cfg.DatabaseURL = flags.dbUrl
	cfg.RedisURL = flags.redisUrl
	cfg.LemonSqueezyAPIKey = flags.lemonApiKey
	cfg.LemonSqueezyStoreID = flags.lemonStoreId
	cfg.LemonSqueezyVariantID = flags.lemonVariantId
	cfg.LemonSqueezyWebhookSecret = flags.lemonWebhookSec

	generated, genErr := setup.GenerateHMACSecret()
	if genErr != nil {
		return nil, &cmd.CommandError{Summary: "failed to generate HMAC secret", Detail: genErr.Error()}
	}
	cfg.HMACSecret = generated

	resolved, err := setup.ResolveTargetPath(cfg.TargetInput)
	if err != nil {
		return nil, err
	}
	cfg.TargetPath = resolved

	return cfg, nil
}

type DashboardCommand struct{}

func (c *DashboardCommand) Name() string     { return "dashboard" }
func (c *DashboardCommand) Description() string { return "initialize a new Scrawn dashboard" }

func (c *DashboardCommand) Run(ctx *cmd.Context, args []string) error {
	flags := parseDashboardFlags(args)
	if flags == nil {
		return nil
	}
	if flags.help {
		dashboardHelp()
		return nil
	}

	target := flags.path
	if target == "" {
		target = "."
	}

	ui.RenderDashboardIntent(target)
	return nil
}

type dashboardFlags struct {
	path string
	help bool
}

func parseDashboardFlags(args []string) *dashboardFlags {
	flags := &dashboardFlags{}

	fs := flag.NewFlagSet("init dashboard", flag.ContinueOnError)
	fs.BoolVar(&flags.help, "h", false, "help")
	fs.BoolVar(&flags.help, "help", false, "help")
	fs.StringVar(&flags.path, "path", "", "path")

	fs.SetOutput(os.NewFile(1, "/dev/null"))
	if err := fs.Parse(args); err != nil {
		return nil
	}

	if fs.NArg() > 0 && flags.path == "" {
		flags.path = fs.Arg(0)
	}

	if flags.help {
		return flags
	}

	return flags
}

func runInteractive(ctx *cmd.Context) error {
	cfg, err := collectServerConfigWizard(nil)
	if err != nil {
		return err
	}

	result, err := setup.SetupServer(*cfg, ui.Step, ui.MarkOK)
	if err != nil {
		return err
	}

	ui.RenderSuccess(result, "server")
	ui.RenderDashboardStub(result.TargetPath)
	return nil
}

func collectServerConfigWizard(flags *serverFlags) (*setup.Config, error) {
	pmOptions, err := setup.AvailablePackageManagers()
	if err != nil {
		return nil, err
	}

	defaultPM := pmOptions[0]
	for _, pm := range pmOptions {
		if pm == "bun" {
			defaultPM = "bun"
			break
		}
	}

	fields := []ui.WizardField{
		{Key: "packageManager", Label: "Package Manager", Description: "Which package manager do you want to use?", Type: ui.FieldSelect, Options: pmOptions, DefaultValue: defaultPM},
		{Key: "targetPath", Label: "Server Directory", Description: "Where should the server be created?", Type: ui.FieldInput, DefaultValue: "./scrawn-server", Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("enter a folder path or use .")
			}
			return nil
		}},
		{Key: "userIdType", Label: "User ID Type", Description: "What type of user IDs does your app use?", Type: ui.FieldSelect, Options: []string{"uuid", "bigint", "int"}, DefaultValue: "uuid"},
		{Key: "databaseURL", Label: "PostgreSQL", Description: "What's your DATABASE_URL connection string?", Type: ui.FieldInput, DefaultValue: "postgresql://user:password@host:5432/scrawn", Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("DATABASE_URL is required")
			}
			return nil
		}},
		{Key: "redisURL", Label: "Redis", Description: "What's your REDIS_URL connection string?", Type: ui.FieldInput, DefaultValue: "redis://localhost:6379", Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("REDIS_URL is required")
			}
			return nil
		}},
		{Key: "lemonAPIKey", Label: "Lemon Squeezy API Key", Description: "Optional - for payment processing", Type: ui.FieldInput, AllowBlank: true},
		{Key: "lemonStoreID", Label: "Lemon Squeezy Store ID", Description: "Optional", Type: ui.FieldInput, AllowBlank: true},
		{Key: "lemonVariantID", Label: "Lemon Squeezy Variant ID", Description: "Optional", Type: ui.FieldInput, AllowBlank: true},
		{Key: "lemonWebhookSecret", Label: "Lemon Squeezy Webhook Secret", Description: "Optional", Type: ui.FieldInput, AllowBlank: true},
	}

	values, err := ui.RunWizard("init server", "", fields)
	if err != nil {
		return nil, translateError(err)
	}

	cfg := &setup.Config{
		Kind:        "server",
		TargetInput: values["targetPath"],
		UserIDType:  values["userIdType"],
	}

	cfg.PackageManager = values["packageManager"]
	cfg.UserIDType = values["userIdType"]

	generated, genErr := setup.GenerateHMACSecret()
	if genErr != nil {
		return nil, &cmd.CommandError{Summary: "failed to generate HMAC secret", Detail: genErr.Error()}
	}
	cfg.HMACSecret = generated
	cfg.DatabaseURL = values["databaseURL"]
	cfg.RedisURL = values["redisURL"]
	cfg.LemonSqueezyAPIKey = values["lemonAPIKey"]
	cfg.LemonSqueezyStoreID = values["lemonStoreID"]
	cfg.LemonSqueezyVariantID = values["lemonVariantID"]
	cfg.LemonSqueezyWebhookSecret = values["lemonWebhookSecret"]

	resolved, err := setup.ResolveTargetPath(cfg.TargetInput)
	if err != nil {
		return nil, err
	}
	cfg.TargetPath = resolved

	return cfg, nil
}

func translateError(err error) error {
	if err == ui.ErrPromptInterrupted {
		return &cmd.CommandError{Summary: "setup cancelled", Detail: "the prompt session was interrupted"}
	}
	return err
}