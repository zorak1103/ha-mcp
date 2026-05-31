// Package main provides the entry point for the ha-mcp server.
// coverage-exempt: CLI orchestration, server lifecycle, and OS signal handling require a real process environment
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/zorak1103/ha-mcp/configs"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/handlers"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/logging"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// App holds the CLI application state and dependencies.
type App struct {
	cfgFile  string
	haURL    string
	haToken  string
	port     int
	readOnly bool
	rootCmd  *cobra.Command
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
	a.rootCmd.PersistentFlags().BoolVar(&a.readOnly, "read-only", false, "Enable read-only mode (blocks all write operations)")

	bindPFlag("homeassistant.url", a.rootCmd.PersistentFlags().Lookup("ha-url"))
	bindPFlag("homeassistant.token", a.rootCmd.PersistentFlags().Lookup("ha-token"))
	bindPFlag("server.port", a.rootCmd.PersistentFlags().Lookup("port"))
	bindPFlag("server.read_only", a.rootCmd.PersistentFlags().Lookup("read-only"))
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

	if err := os.WriteFile(filename, content, 0o600); err != nil {
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
	fmt.Printf("  URL:        %s\n", masked.HomeAssistant.URL)
	fmt.Printf("  Token:      %s\n", masked.HomeAssistant.Token)
	fmt.Println()
	fmt.Println("  REST API:")
	fmt.Printf("    Rate Limit: %.1f req/s\n", masked.HomeAssistant.REST.RateLimit)
	fmt.Printf("    Rate Burst: %d\n", masked.HomeAssistant.REST.RateBurst)
	fmt.Printf("    Max Retries: %d\n", masked.HomeAssistant.REST.MaxRetries)
	fmt.Printf("    Retry Initial Delay: %d ms\n", masked.HomeAssistant.REST.RetryInitialDelayMs)
	fmt.Printf("    Retry Max Delay: %d ms\n", masked.HomeAssistant.REST.RetryMaxDelayMs)
	fmt.Println()
	fmt.Println("  WebSocket:")
	fmt.Printf("    Max Retries: %d\n", masked.HomeAssistant.WebSocket.MaxRetries)
	fmt.Printf("    Retry Initial Delay: %d ms\n", masked.HomeAssistant.WebSocket.RetryInitialDelayMs)
	fmt.Printf("    Retry Max Delay: %d ms\n", masked.HomeAssistant.WebSocket.RetryMaxDelayMs)
	fmt.Println()
	fmt.Println("Server:")
	fmt.Printf("  Port:       %d\n", masked.Server.Port)
	fmt.Println()
	fmt.Println("Logging:")
	fmt.Printf("  Level:      %s\n", masked.Logging.Level)

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
	cfg, err := config.Load(a.cfgFile)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	logger := a.setupLogger(cfg)
	ctx, cancel := a.setupGracefulShutdown(logger)
	defer cancel()

	// Create client pool (always required for per-request auth)
	restConfig := &homeassistant.RESTClientConfig{
		RateLimit: cfg.HomeAssistant.REST.RateLimit,
		RateBurst: cfg.HomeAssistant.REST.RateBurst,
	}
	clientPool := homeassistant.NewClientPoolWithFullConfig(cfg.HomeAssistant.URL, 30*time.Minute, 100, restConfig, nil, logger)
	defer func() {
		logger.Info("Closing client pool...")
		if closeErr := clientPool.Close(); closeErr != nil {
			logger.Error("Error closing client pool", "error", closeErr)
		}
	}()

	// Create default client only if token is configured (optional, for backwards compatibility)
	var defaultClient homeassistant.Client
	if cfg.HomeAssistant.Token != "" {
		logger.Info("Creating default HA client from configured token...")
		defaultClient, err = a.initHomeAssistantClient(ctx, cfg, logger)
		if err != nil {
			logger.Warn("Could not create default HA client, per-request auth only", "error", err)
		} else {
			defer a.closeHomeAssistantClient(defaultClient, logger)
		}
	} else {
		logger.Info("No default token configured - clients must provide Authorization header")
	}

	mcpServer, err := a.initMCPServer(clientPool, defaultClient, cfg, logger)
	if err != nil {
		return fmt.Errorf("initializing MCP server: %w", err)
	}
	a.startMCPServer(mcpServer, logger, cancel)

	<-ctx.Done()
	logger.Info("Shutdown complete")

	return nil
}

// setupLogger configures and returns a logger based on the configuration.
func (a *App) setupLogger(cfg *config.Config) *logging.Logger {
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

	if logLevel <= logging.LevelTrace {
		logger.Warn("TRACE log level active: payload summaries only (method, keys, size); not intended for production use")
	}

	return logger
}

// setupGracefulShutdown configures signal handling for graceful shutdown.
func (a *App) setupGracefulShutdown(logger *logging.Logger) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is called in goroutine on signal and returned to caller

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down...", "signal", sig)
		cancel()
	}()

	return ctx, cancel
}

// initHomeAssistantClient creates and connects the Home Assistant WebSocket client.
func (a *App) initHomeAssistantClient(
	ctx context.Context,
	cfg *config.Config,
	logger *logging.Logger,
) (homeassistant.Client, error) {
	logger.Info("Connecting to Home Assistant WebSocket API...")

	restConfig := &homeassistant.RESTClientConfig{
		RateLimit: cfg.HomeAssistant.REST.RateLimit,
		RateBurst: cfg.HomeAssistant.REST.RateBurst,
	}
	haClient, err := homeassistant.NewConnectedClient(ctx, cfg.HomeAssistant.URL, cfg.HomeAssistant.Token, nil, restConfig)
	if err != nil {
		return nil, fmt.Errorf("connecting to Home Assistant: %w", err)
	}

	logger.Info("Connected to Home Assistant WebSocket API")

	return haClient, nil
}

// closeHomeAssistantClient gracefully closes the Home Assistant WebSocket connection.
func (a *App) closeHomeAssistantClient(client homeassistant.Client, logger *logging.Logger) {
	logger.Info("Closing Home Assistant WebSocket connection...")

	if err := homeassistant.CloseClient(client); err != nil {
		logger.Error("Error closing Home Assistant client", "error", err)
	}
}

// initMCPServer creates and configures the MCP server with all registered tools.
// Returns an error if the tool filter config references non-existent tools or actions.
func (a *App) initMCPServer(
	clientPool *homeassistant.ClientPool,
	defaultClient homeassistant.Client,
	cfg *config.Config,
	logger *logging.Logger,
) (*mcp.Server, error) {
	registry := mcp.NewRegistry()
	handlers.RegisterAllTools(registry)
	handlers.RegisterAllResources(registry)

	logger.Info("Registered MCP tools", "count", registry.ToolCount())
	logger.Info("Registered MCP resources", "count", registry.ResourceCount())
	registry.LogRegisteredTools(logger)

	filterCfg := mcp.ToolFilterConfig{
		Whitelist: cfg.Server.ToolFilter.Whitelist,
		Blacklist: cfg.Server.ToolFilter.Blacklist,
	}
	if err := mcp.ValidateFilterConfig(filterCfg); err != nil {
		return nil, err
	}

	filter := mcp.NewToolFilterEngine(filterCfg, cfg.Server.ReadOnly)

	if filter.IsEnabled() {
		removed := filter.ApplyToRegistry(registry)
		logger.Info("Tool filter applied",
			"removed", removed,
			"remaining", registry.ToolCount(),
			"read_only", cfg.Server.ReadOnly)
	}

	server := mcp.NewServer(clientPool, defaultClient, registry, cfg.Server.Port, logger)
	server.SetToolFilter(filter)
	server.SetWaitConfig(mcp.WaitConfig{
		Timeout:      time.Duration(cfg.HomeAssistant.Wait.WaitTimeoutMs) * time.Millisecond,
		PollInterval: time.Duration(cfg.HomeAssistant.Wait.WaitPollIntervalMs) * time.Millisecond,
	})

	return server, nil
}

// startMCPServer starts the MCP server in a goroutine.
func (a *App) startMCPServer(server *mcp.Server, logger *logging.Logger, cancel context.CancelFunc) {
	go func() {
		if err := server.Start(); err != nil {
			logger.Error("MCP server error", "error", err)
			cancel()
		}
	}()
}
