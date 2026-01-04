// Package main provides the entry point for the ha-mcp server.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"gitlab.com/zorak1103/ha-mcp/configs"
	"gitlab.com/zorak1103/ha-mcp/internal/config"
	"gitlab.com/zorak1103/ha-mcp/internal/handlers"
	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/logging"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// App holds the CLI application state and dependencies.
type App struct {
	cfgFile string
	haURL   string
	haToken string
	port    int
	rootCmd *cobra.Command
}

// NewApp creates a new CLI application instance with all dependencies.
func NewApp() *App {
	app := &App{}
	app.rootCmd = app.buildRootCmd()
	app.setupFlags()
	app.addCommands()
	return app
}

// buildRootCmd creates the root cobra command.
func (a *App) buildRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ha-mcp",
		Short: "MCP Server for Home Assistant",
		Long: `ha-mcp is a Model Context Protocol (MCP) server that provides
AI agents like Cline and opencode with access to Home Assistant.

It exposes Home Assistant entities, automations, scripts, scenes,
and helpers through the MCP protocol over HTTP.`,
		RunE: a.run,
	}
}

// setupFlags configures CLI flags and binds them to viper.
func (a *App) setupFlags() {
	a.rootCmd.PersistentFlags().StringVar(&a.cfgFile, "config", "", "config file (default: ./config.yaml)")
	a.rootCmd.PersistentFlags().StringVar(&a.haURL, "ha-url", "", "Home Assistant URL")
	a.rootCmd.PersistentFlags().StringVar(&a.haToken, "ha-token", "", "Home Assistant long-lived access token")
	a.rootCmd.PersistentFlags().IntVar(&a.port, "port", 0, "MCP server port")

	bindPFlag("homeassistant.url", a.rootCmd.PersistentFlags().Lookup("ha-url"))
	bindPFlag("homeassistant.token", a.rootCmd.PersistentFlags().Lookup("ha-token"))
	bindPFlag("server.port", a.rootCmd.PersistentFlags().Lookup("port"))
}

// addCommands adds subcommands to the root command.
func (a *App) addCommands() {
	a.rootCmd.AddCommand(a.buildConfigCmd())
	a.rootCmd.AddCommand(a.buildInitCmd())
}

// buildConfigCmd creates the config subcommand that displays the effective configuration.
func (a *App) buildConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Display the effective configuration",
		Long: `Display the effective configuration with sensitive data masked.

This command shows the configuration that would be used if the server were started,
including values from the config file, environment variables, and CLI flags.
Sensitive data like tokens are masked for security.`,
		RunE: a.runConfig,
	}
}

// buildInitCmd creates the init subcommand that creates configuration files.
func (a *App) buildInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration files",
		Long: `Create configuration files in the current directory.

This command creates:
  - config.yaml: YAML configuration file
  - .env: Environment variables file

If files already exist, they will not be overwritten unless --force is specified.`,
		RunE: a.runInit,
	}
}

// runInit creates configuration files from embedded templates.
func (a *App) runInit(_ *cobra.Command, _ []string) error {
	created := 0

	// Create config.yaml
	wasCreated, err := a.writeConfigFile("config.yaml", configs.ConfigYAML)
	if err != nil {
		return err
	}
	if wasCreated {
		created++
	}

	// Create .env
	wasCreated, err = a.writeConfigFile(".env", configs.EnvExample)
	if err != nil {
		return err
	}
	if wasCreated {
		created++
	}

	if created == 0 {
		fmt.Println("All configuration files already exist. Nothing to do.")
		return nil
	}

	fmt.Printf("Created %d configuration file(s) in current directory.\n", created)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit config.yaml or .env with your Home Assistant settings")
	fmt.Println("  2. Run 'ha-mcp config' to verify your configuration")
	fmt.Println("  3. Run 'ha-mcp' to start the server")

	return nil
}

// writeConfigFile writes content to a file if it doesn't already exist.
// Returns true if the file was created, false if it was skipped.
func (a *App) writeConfigFile(filename string, content []byte) (bool, error) {
	if _, err := os.Stat(filename); err == nil {
		fmt.Printf("Skipping %s (already exists)\n", filename)
		return false, nil
	}

	if err := os.WriteFile(filename, content, 0600); err != nil {
		return false, fmt.Errorf("writing %s: %w", filename, err)
	}

	fmt.Printf("Created %s\n", filename)
	return true, nil
}

// runConfig loads and displays the effective configuration with masked sensitive data.
func (a *App) runConfig(_ *cobra.Command, _ []string) error {
	// Load configuration without validation (allow missing token for display)
	cfg, err := config.LoadForDisplay(a.cfgFile)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Get masked version for output
	masked := cfg.MaskedConfig()

	// Output in human-readable format
	fmt.Println("Effective Configuration")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Println("Home Assistant:")
	fmt.Printf("  URL:   %s\n", masked.HomeAssistant.URL)
	fmt.Printf("  Token: %s\n", masked.HomeAssistant.Token)
	fmt.Println()
	fmt.Println("Server:")
	fmt.Printf("  Port:  %d\n", masked.Server.Port)
	fmt.Println()
	fmt.Println("Logging:")
	fmt.Printf("  Level: %s\n", masked.Logging.Level)

	return nil
}

// Execute runs the CLI application.
func (a *App) Execute() error {
	return a.rootCmd.Execute()
}

// bindPFlag binds a flag to viper and logs an error if binding fails.
func bindPFlag(key string, flag *pflag.Flag) {
	if err := viper.BindPFlag(key, flag); err != nil {
		log.Printf("warning: failed to bind flag %s: %v", key, err)
	}
}

func main() {
	app := NewApp()
	if err := app.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run executes the main server logic.
func (a *App) run(_ *cobra.Command, _ []string) error {
	// Load configuration
	cfg, err := config.Load(a.cfgFile)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Setup logger with configured level
	logLevel, err := logging.ParseLevel(cfg.Logging.Level)
	if err != nil {
		log.Printf("Warning: invalid log level %q, using INFO", cfg.Logging.Level)
		logLevel = logging.LevelInfo
	}
	logger := logging.New(logLevel)
	logging.SetDefault(logger)

	logger.Info("Starting ha-mcp server", "port", cfg.Server.Port)
	logger.Info("Home Assistant URL", "url", cfg.HomeAssistant.URL)
	logger.Info("Log level", "level", logging.LevelString(logLevel))

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down...", "signal", sig)
		cancel()
	}()

	// Initialize Home Assistant WebSocket client
	logger.Info("Connecting to Home Assistant WebSocket API...")
	haClient, err := homeassistant.NewDefaultWSClient(ctx, cfg.HomeAssistant.URL, cfg.HomeAssistant.Token)
	if err != nil {
		return fmt.Errorf("connecting to Home Assistant: %w", err)
	}
	logger.Info("Connected to Home Assistant WebSocket API")

	// Ensure graceful shutdown of WebSocket connection
	defer func() {
		logger.Info("Closing Home Assistant WebSocket connection...")
		if closeErr := homeassistant.CloseClient(haClient); closeErr != nil {
			logger.Error("Error closing Home Assistant client", "error", closeErr)
		}
	}()

	// Initialize MCP registry and register all tools
	registry := mcp.NewRegistry()

	// Register all tool handlers (entity, automation, helpers, media, etc.)
	handlers.RegisterAllTools(registry)

	logger.Info("Registered MCP tools", "count", registry.ToolCount())

	// Log all registered tools at debug level
	registry.LogRegisteredTools(logger)

	// Initialize MCP server with logger
	mcpServer := mcp.NewServer(haClient, registry, cfg.Server.Port, logger)

	// Start MCP server in goroutine
	go func() {
		if err := mcpServer.Start(); err != nil {
			logger.Error("MCP server error", "error", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutdown complete")

	return nil
}
