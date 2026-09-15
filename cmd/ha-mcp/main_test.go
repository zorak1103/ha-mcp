// Package main provides tests for the ha-mcp server CLI.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"

	"github.com/zorak1103/ha-mcp/internal/config"
)

func TestNewApp(t *testing.T) {
	// Not parallel: uses global viper instance
	app := NewApp()

	if app == nil {
		t.Fatal("NewApp() returned nil")
	}

	if app.rootCmd == nil {
		t.Error("NewApp() did not create rootCmd")
	}

	if app.rootCmd.Use != "ha-mcp" {
		t.Errorf("rootCmd.Use = %q, want %q", app.rootCmd.Use, "ha-mcp")
	}
}

func TestBuildRootCmd(t *testing.T) {
	// Not parallel: uses global viper instance via RunE
	app := &App{}
	cmd := app.buildRootCmd()

	if cmd == nil {
		t.Fatal("buildRootCmd() returned nil")
	}

	if cmd.Use != "ha-mcp" {
		t.Errorf("Use = %q, want %q", cmd.Use, "ha-mcp")
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.Long == "" {
		t.Error("Long description is empty")
	}

	if cmd.RunE == nil {
		t.Error("RunE is nil")
	}
}

func TestSetupFlags(t *testing.T) {
	// Not parallel: uses global viper instance
	app := &App{}
	app.rootCmd = &cobra.Command{Use: "test"}
	app.setupFlags()

	tests := []struct {
		name     string
		flagName string
	}{
		{"config flag", "config"},
		{"ha-url flag", "ha-url"},
		{"ha-token flag", "ha-token"},
		{"port flag", "port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := app.rootCmd.PersistentFlags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
			}
		})
	}
}

func TestAddCommands(t *testing.T) {
	// Not parallel: creates commands that may use viper
	app := &App{}
	app.rootCmd = &cobra.Command{Use: "test"}
	app.addCommands()

	commands := app.rootCmd.Commands()

	if len(commands) != 2 {
		t.Errorf("expected 2 subcommands, got %d", len(commands))
	}

	expectedCommands := map[string]bool{
		"config": false,
		"init":   false,
	}

	for _, cmd := range commands {
		if _, ok := expectedCommands[cmd.Use]; ok {
			expectedCommands[cmd.Use] = true
		}
	}

	for name, found := range expectedCommands {
		if !found {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestBuildConfigCmd(t *testing.T) {
	// Not parallel: command may use viper
	app := &App{}
	cmd := app.buildConfigCmd()

	if cmd == nil {
		t.Fatal("buildConfigCmd() returned nil")
	}

	if cmd.Use != "config" {
		t.Errorf("Use = %q, want %q", cmd.Use, "config")
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.RunE == nil {
		t.Error("RunE is nil")
	}
}

func TestBuildInitCmd(t *testing.T) {
	// Not parallel: command may use viper
	app := &App{}
	cmd := app.buildInitCmd()

	if cmd == nil {
		t.Fatal("buildInitCmd() returned nil")
	}

	if cmd.Use != "init" {
		t.Errorf("Use = %q, want %q", cmd.Use, "init")
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.RunE == nil {
		t.Error("RunE is nil")
	}
}

func TestWriteConfigFile(t *testing.T) {
	tests := []struct {
		name        string
		fileExists  bool
		content     []byte
		wantCreated bool
		wantErr     bool
	}{
		{
			name:        "creates new file",
			fileExists:  false,
			content:     []byte("test content"),
			wantCreated: true,
			wantErr:     false,
		},
		{
			name:        "skips existing file",
			fileExists:  true,
			content:     []byte("new content"),
			wantCreated: false,
			wantErr:     false,
		},
		{
			name:        "handles empty content",
			fileExists:  false,
			content:     []byte{},
			wantCreated: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()
			filename := filepath.Join(tmpDir, "test-config.yaml")

			// Pre-create file if needed
			if tt.fileExists {
				if err := os.WriteFile(filename, []byte("existing"), 0600); err != nil {
					t.Fatalf("failed to create existing file: %v", err)
				}
			}

			app := &App{}
			created, err := app.writeConfigFile(&cobra.Command{}, filename, tt.content)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("writeConfigFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check created flag
			if created != tt.wantCreated {
				t.Errorf("writeConfigFile() created = %v, want %v", created, tt.wantCreated)
			}

			// Verify file content if created
			if tt.wantCreated && !tt.wantErr {
				content, err := os.ReadFile(filename) //nolint:gosec // Test file path is controlled
				if err != nil {
					t.Errorf("failed to read created file: %v", err)
				}
				if diff := cmp.Diff(tt.content, content); diff != "" {
					t.Errorf("file content mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestWriteConfigFile_InvalidPath(t *testing.T) {
	app := &App{}
	// Use invalid path that cannot be written to
	_, err := app.writeConfigFile(&cobra.Command{}, "/nonexistent/path/config.yaml", []byte("content"))

	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestRunInit(t *testing.T) {
	// Save current directory and change to temp dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore directory: %v", err)
		}
	}()

	app := &App{}
	err = app.runInit(&cobra.Command{}, nil)

	if err != nil {
		t.Errorf("runInit() error = %v", err)
	}

	// Check that files were created
	expectedFiles := []string{"config.yaml", ".env"}
	for _, filename := range expectedFiles {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Errorf("expected file %q was not created", filename)
		}
	}
}

func TestRunInit_FilesExist(t *testing.T) {
	// Save current directory and change to temp dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore directory: %v", err)
		}
	}()

	// Pre-create files
	if err := os.WriteFile("config.yaml", []byte("existing"), 0600); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}
	if err := os.WriteFile(".env", []byte("existing"), 0600); err != nil {
		t.Fatalf("failed to create .env: %v", err)
	}

	app := &App{}
	err = app.runInit(&cobra.Command{}, nil)

	if err != nil {
		t.Errorf("runInit() error = %v", err)
	}

	// Verify files were not overwritten
	content, _ := os.ReadFile("config.yaml")
	if string(content) != "existing" {
		t.Error("config.yaml was overwritten")
	}
}

func TestApplyFlagOverrides(t *testing.T) {
	tests := []struct {
		name     string
		haURL    string
		haToken  string
		port     int
		readOnly bool
		wantURL  string
		wantTok  string
		wantPort int
		wantRO   bool
	}{
		{"no flags set keeps config values", "", "", 0, false, "http://yaml:8123", "yaml-token", 8080, false},
		{"all flags set override config", "http://flag:8123", "flag-token", 9999, true, "http://flag:8123", "flag-token", 9999, true},
		{"only port set", "", "", 9999, false, "http://yaml:8123", "yaml-token", 9999, false},
		{"only token set", "", "flag-token", 0, false, "http://yaml:8123", "flag-token", 8080, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{haURL: tt.haURL, haToken: tt.haToken, port: tt.port, readOnly: tt.readOnly}
			cfg := &config.Config{}
			cfg.HomeAssistant.URL = "http://yaml:8123"
			cfg.HomeAssistant.Token = "yaml-token"
			cfg.Server.Port = 8080
			cfg.Server.ReadOnly = false

			app.applyFlagOverrides(cfg)

			if cfg.HomeAssistant.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", cfg.HomeAssistant.URL, tt.wantURL)
			}
			if cfg.HomeAssistant.Token != tt.wantTok {
				t.Errorf("Token = %q, want %q", cfg.HomeAssistant.Token, tt.wantTok)
			}
			if cfg.Server.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", cfg.Server.Port, tt.wantPort)
			}
			if cfg.Server.ReadOnly != tt.wantRO {
				t.Errorf("ReadOnly = %v, want %v", cfg.Server.ReadOnly, tt.wantRO)
			}
		})
	}
}

func TestFlagOverridesReachLoadedConfig(t *testing.T) {
	// End-to-end: CLI flags must override values that came from the config file.
	// Regression: flags were bound to the global viper instance, which
	// config.Load never reads, so --port etc. were silently ignored.
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `homeassistant:
  url: "http://yaml.local:8123"
  token: "yaml-token"
server:
  port: 8080
logging:
  level: info
`
	if err := os.WriteFile(configFile, []byte(configContent), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	app := &App{haURL: "http://flag.local:8123", port: 9999, readOnly: true}
	app.applyFlagOverrides(cfg)

	if cfg.HomeAssistant.URL != "http://flag.local:8123" {
		t.Errorf("URL = %q, want flag value", cfg.HomeAssistant.URL)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("Port = %d, want 9999", cfg.Server.Port)
	}
	if cfg.Server.ReadOnly != true {
		t.Errorf("ReadOnly = %v, want true", cfg.Server.ReadOnly)
	}
	if cfg.HomeAssistant.Token != "yaml-token" {
		t.Errorf("Token = %q, want config value (flag empty)", cfg.HomeAssistant.Token)
	}
}

func TestRootCmdVersion(t *testing.T) {
	app := NewApp()
	if app.rootCmd.Version == "" {
		t.Error("rootCmd.Version should be set for goreleaser ldflags injection")
	}
}

func TestRootCmdSilenceUsageAndErrors(t *testing.T) {
	// Runtime errors (config load, HA connect) must not dump the full usage text.
	app := NewApp()
	if !app.rootCmd.SilenceUsage {
		t.Error("rootCmd.SilenceUsage should be true")
	}
	if !app.rootCmd.SilenceErrors {
		t.Error("rootCmd.SilenceErrors should be true")
	}
}

func TestSubcommandsRejectArgs(t *testing.T) {
	app := NewApp()
	for _, name := range []string{"config", "init"} {
		sub, _, err := app.rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("Find(%q) error = %v", name, err)
		}
		if err := sub.Args(sub, []string{"unexpected"}); err == nil {
			t.Errorf("%q should reject positional args", name)
		}
		if err := sub.Args(sub, nil); err != nil {
			t.Errorf("%q should accept no args: %v", name, err)
		}
	}
}

func TestVersionOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	app := NewApp()
	app.rootCmd.SetOut(buf)
	app.rootCmd.SetErr(buf)
	app.rootCmd.SetArgs([]string{"--version"})

	if err := app.Execute(); err != nil {
		t.Fatalf("Execute(--version) error = %v", err)
	}
	if !strings.Contains(buf.String(), version) {
		t.Errorf("--version output %q should contain version %q", buf.String(), version)
	}
}

func TestFlagOverridesShownInConfigOutput(t *testing.T) {
	// `config` display should reflect CLI flags, not just the config file.
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `homeassistant:
  url: "http://test.local:8123"
  token: "test-token-12345"
server:
  port: 8080
logging:
  level: info
`
	if err := os.WriteFile(configFile, []byte(configContent), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	app := &App{cfgFile: configFile, port: 9999}
	if err := app.runConfig(cmd, nil); err != nil {
		t.Fatalf("runConfig() error = %v", err)
	}

	if !strings.Contains(buf.String(), "Port:       9999") {
		t.Errorf("output should show flag-overridden port, got: %s", buf.String())
	}
}

func TestExecute(t *testing.T) {
	// Not parallel: uses global viper instance via NewApp
	app := NewApp()
	// Set args to show help (no actual execution)
	app.rootCmd.SetArgs([]string{"--help"})

	// Execute should not error on help
	err := app.Execute()
	if err != nil {
		t.Errorf("Execute() with --help error = %v", err)
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	// Not parallel: uses global viper instance via NewApp
	app := NewApp()
	app.rootCmd.SetArgs([]string{"unknown-command"})

	err := app.Execute()
	if err == nil {
		t.Error("Execute() with unknown command should return error")
	}
}

func TestAppFieldDefaults(t *testing.T) {
	app := &App{}

	if app.cfgFile != "" {
		t.Errorf("cfgFile default = %q, want empty", app.cfgFile)
	}
	if app.haURL != "" {
		t.Errorf("haURL default = %q, want empty", app.haURL)
	}
	if app.haToken != "" {
		t.Errorf("haToken default = %q, want empty", app.haToken)
	}
	if app.port != 0 {
		t.Errorf("port default = %d, want 0", app.port)
	}
}

func TestRootCmdHasRunE(t *testing.T) {
	// Not parallel: uses global viper instance via NewApp
	app := NewApp()

	if app.rootCmd.RunE == nil {
		t.Error("rootCmd.RunE should not be nil")
	}
}

func TestSubcommandCount(t *testing.T) {
	// Not parallel: uses global viper instance via NewApp
	app := NewApp()
	commands := app.rootCmd.Commands()

	// Should have exactly 2 subcommands: config and init
	if len(commands) != 2 {
		t.Errorf("expected 2 subcommands, got %d", len(commands))
	}
}

func TestRunConfig(t *testing.T) {
	// Save current directory and change to temp dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore directory: %v", err)
		}
	}()

	// Create a minimal config file
	configContent := `homeassistant:
  url: "http://test.local:8123"
  token: "test-token-12345"
server:
  port: 8080
logging:
  level: info
`
	if err := os.WriteFile("config.yaml", []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	app := &App{}
	err = app.runConfig(&cobra.Command{}, nil)

	if err != nil {
		t.Errorf("runConfig() error = %v", err)
	}
}

func TestRunConfig_WithRESTConfig(t *testing.T) {
	// Save current directory and change to temp dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore directory: %v", err)
		}
	}()

	// Create config file with REST rate limiting settings
	configContent := `homeassistant:
  url: "http://test.local:8123"
  token: "test-token-12345"
  rest:
    rate_limit: 20.0
    rate_burst: 10
server:
  port: 8080
logging:
  level: info
`
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	app := &App{cfgFile: configFile}
	err = app.runConfig(cmd, nil)

	output := buf.String()

	if err != nil {
		t.Errorf("runConfig() error = %v", err)
	}

	// Verify REST rate limiting values are in output
	if !strings.Contains(output, "REST API:") {
		t.Error("output should contain 'REST API:' section")
	}
	if !strings.Contains(output, "Rate Limit: 20.0 req/s") {
		t.Errorf("output should contain 'Rate Limit: 20.0 req/s', got: %s", output)
	}
	if !strings.Contains(output, "Rate Burst: 10") {
		t.Errorf("output should contain 'Rate Burst: 10', got: %s", output)
	}
}

func TestRunConfig_NoConfig(t *testing.T) {
	// Save current directory and change to temp dir with no config
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore directory: %v", err)
		}
	}()

	app := &App{}
	// This should still work but with default/empty values
	err = app.runConfig(&cobra.Command{}, nil)

	// May or may not error depending on config loading behavior
	// Just ensure it doesn't panic
	_ = err
}

func TestRunInit_PartialExisting(t *testing.T) {
	// Save current directory and change to temp dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore directory: %v", err)
		}
	}()

	// Pre-create only config.yaml
	if err := os.WriteFile("config.yaml", []byte("existing"), 0600); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	app := &App{}
	err = app.runInit(&cobra.Command{}, nil)

	if err != nil {
		t.Errorf("runInit() error = %v", err)
	}

	// Check that .env was created
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		t.Error(".env was not created")
	}

	// Verify config.yaml was not overwritten
	content, _ := os.ReadFile("config.yaml")
	if string(content) != "existing" {
		t.Error("config.yaml was overwritten")
	}
}

func TestFlagDescriptions(t *testing.T) {
	// Not parallel: uses global viper instance via NewApp
	app := NewApp()

	tests := []struct {
		flagName string
		wantDesc bool
	}{
		{"config", true},
		{"ha-url", true},
		{"ha-token", true},
		{"port", true},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			flag := app.rootCmd.PersistentFlags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("flag %q not found", tt.flagName)
			}
			if tt.wantDesc && flag.Usage == "" {
				t.Errorf("flag %q has no usage description", tt.flagName)
			}
		})
	}
}
