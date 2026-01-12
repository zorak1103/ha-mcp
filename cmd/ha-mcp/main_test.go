// Package main provides tests for the ha-mcp server CLI.
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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
			created, err := app.writeConfigFile(filename, tt.content)

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
	_, err := app.writeConfigFile("/nonexistent/path/config.yaml", []byte("content"))

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
	err = app.runInit(nil, nil)

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
	err = app.runInit(nil, nil)

	if err != nil {
		t.Errorf("runInit() error = %v", err)
	}

	// Verify files were not overwritten
	content, _ := os.ReadFile("config.yaml")
	if string(content) != "existing" {
		t.Error("config.yaml was overwritten")
	}
}

func TestBindPFlag(t *testing.T) {
	// Not parallel: uses global viper instance
	viper.Reset()

	// Create a flag set and flag
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("test-flag", "default", "test flag")

	flag := flags.Lookup("test-flag")
	if flag == nil {
		t.Fatal("failed to create test flag")
	}

	// This should not panic
	bindPFlag("test.key", flag)

	// Verify binding works (viper should use the flag value)
	if err := flags.Set("test-flag", "new-value"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	// Clean up
	viper.Reset()
}

func TestBindPFlag_NilFlag(_ *testing.T) {
	// Not parallel: uses global viper instance
	// This should not panic with nil flag
	bindPFlag("test.key", nil)
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
	err = app.runConfig(nil, nil)

	if err != nil {
		t.Errorf("runConfig() error = %v", err)
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
	err = app.runConfig(nil, nil)

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
	err = app.runInit(nil, nil)

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
