package tagcmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ScrawnDotDev/scrawn-cli/internal/cmd"
	"github.com/ScrawnDotDev/scrawn-cli/internal/ui"
)

func init() {
	cmd.Register("tag", func() cmd.Command { return &TagCommand{} })
}

// ---- types ----

type scrawnConfigData struct {
	Directory string `json:"directory"`
	EnvFile   string `json:"envFile"`
	EnvKey    string `json:"envKey"`
	ServerURL string `json:"serverUrl"`
}

type projectConfigFile struct {
	Scrawn scrawnConfigData `json:"scrawn"`
}

type scrawnConfig struct {
	Directory  string
	EnvFile    string
	EnvKey     string
	ServerURL  string
	ProjectDir string
}

type tagsResponseBody struct {
	Tags []string `json:"tags"`
}

func defaultScrawnConfig() scrawnConfig {
	return scrawnConfig{
		Directory:  ".scrawn",
		EnvFile:    ".env",
		EnvKey:     "SCRAWN_API_KEY",
		ServerURL:  "https://api.scrawn.dev",
		ProjectDir: ".",
	}
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
	fmt.Println("Pull tags from the Scrawn server and generate .scrawn/tags.ts")
	fmt.Println()
	fmt.Println(ui.Heading.Render("Configuration:"))
	fmt.Println("  Reads scrawn.config.json from the project root (or uses defaults)")
	fmt.Println("  Reads the API key from the configured env file")
}

// ---- sync implementation ----

func runSync(args []string) error {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			showSyncHelp()
			return nil
		}
	}

	cfg, err := loadScrawnConfig()
	if err != nil {
		return &cmd.CommandError{
			Summary: "failed to load config",
			Detail:  err.Error(),
		}
	}

	apiKey, err := readEnvValue(cfg.ProjectDir, cfg.EnvFile, cfg.EnvKey)
	if err != nil {
		return &cmd.CommandError{
			Summary: "failed to read API key",
			Detail:  err.Error(),
		}
	}

	if apiKey == "" {
		return &cmd.CommandError{
			Summary: "API key not found",
			Detail:  fmt.Sprintf("%s is not set in %s", cfg.EnvKey, filepath.Join(cfg.ProjectDir, cfg.EnvFile)),
		}
	}

	tags, err := fetchTagsFromServer(cfg.ServerURL, apiKey)
	if err != nil {
		return &cmd.CommandError{
			Summary: "failed to fetch tags",
			Detail:  err.Error(),
		}
	}

	outputDir := filepath.Join(cfg.ProjectDir, cfg.Directory)
	outputPath := filepath.Join(outputDir, "tags.ts")
	if err := writeTagsFile(outputPath, tags); err != nil {
		return &cmd.CommandError{
			Summary: "failed to generate tags file",
			Detail:  err.Error(),
		}
	}

	fmt.Println()
	ui.MarkOK("tags synced", fmt.Sprintf("%s (%d tags)", outputPath, len(tags)))
	return nil
}

// ---- config ----

func loadScrawnConfig() (scrawnConfig, error) {
	cfg := defaultScrawnConfig()

	configPath := findConfigFile()
	if configPath == "" {
		return cfg, nil
	}

	cfg.ProjectDir = filepath.Dir(configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("could not read %s: %w", configPath, err)
	}

	var fileCfg projectConfigFile
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return cfg, fmt.Errorf("invalid config in %s: %w", configPath, err)
	}

	if fileCfg.Scrawn.Directory != "" {
		cfg.Directory = fileCfg.Scrawn.Directory
	}
	if fileCfg.Scrawn.EnvFile != "" {
		cfg.EnvFile = fileCfg.Scrawn.EnvFile
	}
	if fileCfg.Scrawn.EnvKey != "" {
		cfg.EnvKey = fileCfg.Scrawn.EnvKey
	}
	if fileCfg.Scrawn.ServerURL != "" {
		cfg.ServerURL = fileCfg.Scrawn.ServerURL
	}

	return cfg, nil
}

func findConfigFile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		configPath := filepath.Join(dir, "scrawn.config.json")
		if _, statErr := os.Stat(configPath); statErr == nil {
			return configPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// ---- env file reader ----

func readEnvValue(projectDir string, envFile string, key string) (string, error) {
	envPath := filepath.Join(projectDir, envFile)

	f, err := os.Open(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("env file not found: %s", envPath)
		}
		return "", fmt.Errorf("could not read %s: %w", envPath, err)
	}
	defer f.Close()

	return scanEnvValue(f, key)
}

func scanEnvValue(r io.Reader, key string) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		if strings.TrimSpace(parts[0]) == key {
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			return val, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", nil
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
		`import { tag as _tag } from "@scrawn/core";`,
		`import type { TagExpr } from "@scrawn/core";`,
		"",
	}

	if len(tags) > 0 {
		quoted := make([]string, len(tags))
		for i, t := range tags {
			quoted[i] = fmt.Sprintf(`"%s"`, t)
		}
		lines = append(lines, fmt.Sprintf("export type ScrawnTag = %s;", strings.Join(quoted, " | ")))
	} else {
		lines = append(lines, "export type ScrawnTag = never;")
	}

	lines = append(lines,
		"",
		"export function tag(name: ScrawnTag): TagExpr {",
		"  return _tag(name);",
		"}",
		"",
	)

	_, err := io.WriteString(w, strings.Join(lines, "\n"))
	return err
}
