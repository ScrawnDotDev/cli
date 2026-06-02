package tagcmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ScrawnDotDev/scrawn-cli/internal/cmd"
	"github.com/ScrawnDotDev/scrawn-cli/internal/ui"
)

func init() {
	cmd.Register("tag", func() cmd.Command { return &TagCommand{} })
}

// ---- types ----

type cliConfig struct {
	APIKey    string `json:"apiKey"`
	GrpcURL   string `json:"grpcUrl"`
	HTTPURL   string `json:"httpUrl"`
	Directory string `json:"directory"`
}

type tagsResponseBody struct {
	Tags []string `json:"tags"`
}

type expressionsResponseBody struct {
	Expressions []string `json:"expressions"`
}

// ---- command ----

type TagCommand struct{}

func (c *TagCommand) Name() string        { return "tag" }
func (c *TagCommand) Description() string { return "manage Scrawn tags" }

func (c *TagCommand) Run(ctx *cmd.Context, args []string) error {
	if len(args) == 0 {
		return &cmd.CommandError{
			Summary: "missing subcommand",
			Detail:  "available: sync",
		}
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "sync":
		return runSync(args[1:])
	case "-h", "--help":
		showTagHelp()
		return nil
	default:
		return &cmd.CommandError{
			Summary: "invalid tag subcommand",
			Detail:  fmt.Sprintf("unknown subcommand: %s. available: sync", args[0]),
		}
	}
}

func showTagHelp() {
	fmt.Println()
	fmt.Println(ui.Heading.Render("Usage:") + " scrawn tag <subcommand> [flags]")
	fmt.Println()
	fmt.Println(ui.Heading.Render("Subcommands:"))
	fmt.Println("  sync    Pull tags from the server and generate type-safe definitions")
	fmt.Println()
	fmt.Println(ui.Heading.Render("Examples:"))
	fmt.Println("  scrawn tag sync")
}

func showSyncHelp() {
	fmt.Println()
	fmt.Println(ui.Heading.Render("Usage:") + " scrawn tag sync [flags]")
	fmt.Println()
	fmt.Println(ui.Heading.Render("Flags:"))
	fmt.Println("  --config <path>    Path to scrawn.config.ts (default: walk up from cwd)")
	fmt.Println("  --api-key <key>    API key (inline override or skip config entirely)")
	fmt.Println("  --http-url <url>   HTTP URL (inline override or skip config entirely)")
	fmt.Println("  --directory <dir>  Output directory (default: scrawn/)")
	fmt.Println()
	fmt.Println(ui.Heading.Render("Examples:"))
	fmt.Println("  scrawn tag sync")
	fmt.Println("  scrawn tag sync --api-key scrn_live_... --http-url http://localhost:8060")
	fmt.Println("  scrawn tag sync --config ./packages/my-app/scrawn.config.ts")
}

// ---- sync implementation ----

func runSync(args []string) error {
	var configPath, apiKey, httpUrl, directory string
	hasAPIKey, hasHTTPURL := false, false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}
		case "--api-key":
			if i+1 < len(args) {
				apiKey = args[i+1]
				hasAPIKey = true
				i++
			}
		case "--http-url":
			if i+1 < len(args) {
				httpUrl = args[i+1]
				hasHTTPURL = true
				i++
			}
		case "--directory":
			if i+1 < len(args) {
				directory = args[i+1]
				i++
			}
		case "-h", "--help":
			showSyncHelp()
			return nil
		}
	}

	// If apiKey AND httpUrl provided inline, skip config + runtime entirely
	if hasAPIKey && hasHTTPURL {
		cfg := cliConfig{
			APIKey:    apiKey,
			HTTPURL:   httpUrl,
			Directory: directory,
		}
		if cfg.Directory == "" {
			cfg.Directory = "scrawn"
		}
		if cfg.HTTPURL == "" {
			cfg.HTTPURL = "http://localhost:8060"
		}
		return syncFromConfig(cfg, configPath)
	}

	// Find scrawn.config.ts
	configPathFound, configDir := findConfigFile(configPath)
	if configPathFound == "" {
		return &cmd.CommandError{
			Summary: "config file not found",
			Detail:  "scrawn.config.ts not found in current or parent directories. Create one with scrawnConfig({...}), or pass --api-key <key> --http-url <url> inline.",
		}
	}

	// Detect runtime
	runtime, err := detectRuntime()
	if err != nil {
		return &cmd.CommandError{
			Summary: "no JavaScript runtime found",
			Detail:  err.Error() + ". Alternatively, pass --api-key <key> --http-url <url> inline to skip config evaluation.",
		}
	}

	// Load env files for node
	if runtime == "node" {
		loadEnvFiles(configDir)
	}

	// Evaluate config
	var cfg cliConfig
	if err := ui.SpinnerTask(fmt.Sprintf("Reading config (%s)", filepath.Base(configPathFound)), func() error {
		var evalErr error
		cfg, evalErr = evalTsConfig(runtime, configPathFound, configDir)
		return evalErr
	}); err != nil {
		return &cmd.CommandError{
			Summary: "failed to read config",
			Detail:  err.Error() + ". Try passing --api-key <key> --http-url <url> inline instead.",
		}
	}

	// Apply inline overrides
	if hasAPIKey {
		cfg.APIKey = apiKey
	}
	if hasHTTPURL {
		cfg.HTTPURL = httpUrl
	}
	if directory != "" {
		cfg.Directory = directory
	}

	if cfg.HTTPURL == "" {
		cfg.HTTPURL = "http://localhost:8060"
	}
	if cfg.Directory == "" {
		cfg.Directory = "scrawn"
	}

	if cfg.APIKey == "" {
		return &cmd.CommandError{
			Summary: "API key not found",
			Detail:  "SCRAWN_KEY is not set in config or environment. Pass --api-key <key> inline.",
		}
	}

	return syncFromConfig(cfg, configPathFound)
}

func syncFromConfig(cfg cliConfig, configFile string) error {
	configDir := filepath.Dir(configFile)

	// 1. Fetch tags
	var tags []string
	if err := ui.SpinnerTask("Fetching tags from server", func() error {
		var fetchErr error
		tags, fetchErr = fetchTagsFromServer(cfg.HTTPURL, cfg.APIKey)
		return fetchErr
	}); err != nil {
		return &cmd.CommandError{
			Summary: "failed to fetch tags",
			Detail:  err.Error(),
		}
	}

	// 2. Fetch expressions
	var expressions []string
	if err := ui.SpinnerTask("Fetching expressions from server", func() error {
		var fetchErr error
		expressions, fetchErr = fetchExpressionsFromServer(cfg.HTTPURL, cfg.APIKey)
		return fetchErr
	}); err != nil {
		return &cmd.CommandError{
			Summary: "failed to fetch expressions",
			Detail:  err.Error(),
		}
	}

	// 3. Generate pricerefs.ts
	outputDir := filepath.Join(configDir, cfg.Directory)
	outputPath := filepath.Join(outputDir, "pricerefs.ts")

	if err := ui.SpinnerTask(fmt.Sprintf("Writing %s", filepath.Join(cfg.Directory, "pricerefs.ts")), func() error {
		return writePricerefsFile(outputPath, tags, expressions)
	}); err != nil {
		return &cmd.CommandError{
			Summary: "failed to generate pricerefs file",
			Detail:  err.Error(),
		}
	}

	fmt.Println()
	ui.MarkOK("synced", fmt.Sprintf("%s (%d tags, %d expressions)", outputPath, len(tags), len(expressions)))
	return nil
}

// ---- runtime detection ----

func detectRuntime() (string, error) {
	if _, err := exec.LookPath("npx"); err == nil {
		return "node", nil
	}
	if _, err := exec.LookPath("bun"); err == nil {
		return "bun", nil
	}
	if _, err := exec.LookPath("node"); err == nil {
		if _, err := exec.LookPath("tsx"); err == nil {
			return "node", nil
		}
	}
	return "", fmt.Errorf("no JavaScript runtime found. Install npm/npx, bun, or tsx to evaluate scrawn.config.ts")
}

// ---- env file loading (for node fallback) ----

func loadEnvFiles(projectDir string) {
	for _, name := range []string{".env.local", ".env"} {
		envPath := filepath.Join(projectDir, name)
		f, err := os.Open(envPath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
		f.Close()
	}
}

// ---- config file resolution ----

func findConfigFile(explicitPath string) (string, string) {
	if explicitPath != "" {
		if _, err := os.Stat(explicitPath); err != nil {
			return "", ""
		}
		return explicitPath, filepath.Dir(explicitPath)
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", ""
	}

	for {
		configPath := filepath.Join(dir, "scrawn.config.ts")
		if _, statErr := os.Stat(configPath); statErr == nil {
			return configPath, dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ""
}

// ---- config evaluation ----

func evalTsConfig(runtime string, configPath string, configDir string) (cliConfig, error) {
	var cfg cliConfig

	// The eval script: import the config and print it as JSON
	relPath, _ := filepath.Rel(configDir, configPath)
	relPath = filepath.ToSlash(relPath)

	var cmd *exec.Cmd

	switch runtime {
	case "bun":
		script := fmt.Sprintf(
			`import c from './%s'; console.log(JSON.stringify(c));`,
			strings.TrimSuffix(relPath, ".ts"),
		)
		cmd = exec.Command("bun", "-e", script)
		cmd.Dir = configDir

	case "node":
		// Try npx tsx first, then tsx directly
		tsxBin := "npx"
		tsxArgs := []string{"--yes", "tsx", "-e"}
		if _, err := exec.LookPath("tsx"); err == nil {
			tsxBin = "tsx"
			tsxArgs = []string{"-e"}
		}

		script := fmt.Sprintf(
			`import c from './%s'; console.log(JSON.stringify(c));`,
			strings.TrimSuffix(relPath, ".ts"),
		)
		args := append(tsxArgs, script)
		cmd = exec.Command(tsxBin, args...)
		cmd.Dir = configDir
		// Pass current env (which includes loaded .env vars)
		cmd.Env = os.Environ()

	default:
		return cfg, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return cfg, fmt.Errorf("could not evaluate scrawn.config.ts: %s\n%s", err, string(output))
	}

	// Parse the last line as JSON (ignore bun's debug output)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	jsonLine := lines[len(lines)-1]

	if err := json.Unmarshal([]byte(jsonLine), &cfg); err != nil {
		return cfg, fmt.Errorf("could not parse config output: %s\n%s", err, string(output))
	}

	return cfg, nil
}

// ---- HTTP client ----

func fetchTagsFromServer(serverURL string, apiKey string) ([]string, error) {
	url := strings.TrimRight(serverURL, "/") + "/api/v1/tags"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach server at %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid API key (server returned 401)")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result tagsResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("could not parse server response: %w", err)
	}

	return result.Tags, nil
}

func fetchExpressionsFromServer(serverURL string, apiKey string) ([]string, error) {
	url := strings.TrimRight(serverURL, "/") + "/api/v1/expressions"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach server at %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid API key (server returned 401)")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result expressionsResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("could not parse server response: %w", err)
	}

	return result.Expressions, nil
}

// ---- file generator ----

func writePricerefsFile(filePath string, tags []string, expressions []string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create directory %s: %w", dir, err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("could not create file %s: %w", filePath, err)
	}
	defer f.Close()

	return writePricerefsContent(f, tags, expressions)
}

func writePricerefsContent(w io.Writer, tags []string, expressions []string) error {
	lines := []string{
		"// Generated by scrawn tag sync. Do not edit manually.",
		"",
	}

	if len(tags) > 0 {
		quoted := make([]string, len(tags))
		for i, t := range tags {
			quoted[i] = fmt.Sprintf(`"%s"`, t)
		}
		lines = append(lines,
			fmt.Sprintf("export const TAGS = [%s] as const;", strings.Join(quoted, ", ")),
		)
	} else {
		lines = append(lines, "export const TAGS = [] as const;")
	}

	lines = append(lines, "export type ScrawnTag = (typeof TAGS)[number];", "")

	if len(expressions) > 0 {
		quoted := make([]string, len(expressions))
		for i, e := range expressions {
			quoted[i] = fmt.Sprintf(`"%s"`, e)
		}
		lines = append(lines,
			fmt.Sprintf("export const EXPRESSIONS = [%s] as const;", strings.Join(quoted, ", ")),
		)
	} else {
		lines = append(lines, "export const EXPRESSIONS = [] as const;")
	}

	lines = append(lines,
		"export type ScrawnExpr = (typeof EXPRESSIONS)[number];",
		"",
	)

	_, err := io.WriteString(w, strings.Join(lines, "\n"))
	return err
}
