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
	fmt.Println(ui.Heading.Render("Usage:") + " scrawn tag sync")
	fmt.Println()
	fmt.Println("Pull tags from the Scrawn server and generate scrawn/tags.ts")
	fmt.Println()
	fmt.Println(ui.Heading.Render("Configuration:"))
	fmt.Println("  Reads scrawn.config.ts from the project root")
	fmt.Println("  Requires bun or node+tsx to evaluate the config")
	fmt.Println("  Environment variables are loaded from .env.local/.env automatically")
}

// ---- sync implementation ----

func runSync(args []string) error {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			showSyncHelp()
			return nil
		}
	}

	// 1. Find scrawn.config.ts
	configPath, configDir := findConfigFile()
	if configPath == "" {
		return &cmd.CommandError{
			Summary: "config file not found",
			Detail:  "scrawn.config.ts not found in current or parent directories. Create one with scrawnConfig({...}).",
		}
	}

	// 2. Detect runtime (bun, or node+tsx)
	runtime, err := detectRuntime()
	if err != nil {
		return &cmd.CommandError{
			Summary: "no JavaScript runtime found",
			Detail:  "bun or node with tsx is required to evaluate scrawn.config.ts. Install bun or node+tsx.",
		}
	}

	// 3. For node: load env file so config.ts can read env vars
	if runtime == "node" {
		loadEnvFiles(configDir)
	}

	// 4. Evaluate scrawn.config.ts via the runtime
	var cfg cliConfig
	if err := ui.SpinnerTask(fmt.Sprintf("Reading config (%s)", filepath.Base(configPath)), func() error {
		var evalErr error
		cfg, evalErr = evalTsConfig(runtime, configPath, configDir)
		return evalErr
	}); err != nil {
		return &cmd.CommandError{
			Summary: "failed to read config",
			Detail:  err.Error(),
		}
	}

	if cfg.HTTPURL == "" {
		cfg.HTTPURL = "http://localhost:8070"
	}
	if cfg.Directory == "" {
		cfg.Directory = "scrawn"
	}

	if cfg.APIKey == "" {
		return &cmd.CommandError{
			Summary: "API key not found",
			Detail:  "SCRAWN_KEY is not set. Make sure it is defined in your .env.local or environment.",
		}
	}

	// 5. Fetch tags from server
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

	// 6. Generate tags.ts
	outputDir := filepath.Join(configDir, cfg.Directory)
	tagsPath := filepath.Join(outputDir, "tags.ts")

	if err := ui.SpinnerTask(fmt.Sprintf("Writing %s", filepath.Join(cfg.Directory, "tags.ts")), func() error {
		return writeTagsFile(tagsPath, tags)
	}); err != nil {
		return &cmd.CommandError{
			Summary: "failed to generate tags file",
			Detail:  err.Error(),
		}
	}

	fmt.Println()
	ui.MarkOK("tags synced", fmt.Sprintf("%s (%d tags)", tagsPath, len(tags)))
	return nil
}

// ---- runtime detection ----

func detectRuntime() (string, error) {
	if _, err := exec.LookPath("bun"); err == nil {
		return "bun", nil
	}
	if _, err := exec.LookPath("node"); err == nil {
		if _, err := exec.LookPath("tsx"); err == nil {
			return "node", nil
		}
		if _, err := exec.LookPath("npx"); err == nil {
			return "node", nil
		}
	}
	return "", fmt.Errorf("neither bun nor node+tsx found")
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

func findConfigFile() (string, string) {
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

// ---- file generator ----

func writeTagsFile(filePath string, tags []string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create directory %s: %w", dir, err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("could not create file %s: %w", filePath, err)
	}
	defer f.Close()

	return writeTagsContent(f, tags)
}

func writeTagsContent(w io.Writer, tags []string) error {
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

	lines = append(lines,
		"export type ScrawnTag = (typeof TAGS)[number];",
		"",
	)

	_, err := io.WriteString(w, strings.Join(lines, "\n"))
	return err
}
