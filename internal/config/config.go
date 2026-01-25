// Package config provides configuration loading for the ha-mcp server.
// Configuration is loaded in order: YAML file → .env file → ENV vars → CLI flags.
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var loadEnvOnce sync.Once

// loadDotEnv loads .env file if it exists (does not override existing env vars).
// It is called once before loading configuration.
func loadDotEnv() {
	loadEnvOnce.Do(func() {
		dotEnvSearchPaths := []string{".env", "configs/.env"}
		for _, f := range dotEnvSearchPaths {
			if _, err := os.Stat(f); err == nil {
				// Load .env but don't override existing environment variables
				_ = godotenv.Load(f)
				return
			}
		}
	})
}

// mustBindEnv binds an environment variable to a config key, panicking on error.
// This is safe because viper.BindEnv only fails if the key is empty, which is a programming error.
func mustBindEnv(v *viper.Viper, key string, envVars ...string) {
	if err := v.BindEnv(append([]string{key}, envVars...)...); err != nil {
		panic(fmt.Sprintf("failed to bind env var for key %s: %v", key, err))
	}
}

// Config holds all configuration for the ha-mcp server.
type Config struct {
	HomeAssistant HomeAssistantConfig `mapstructure:"homeassistant"`
	Server        ServerConfig        `mapstructure:"server"`
	Logging       LoggingConfig       `mapstructure:"logging"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

// HomeAssistantConfig holds Home Assistant connection settings.
type HomeAssistantConfig struct {
	URL       string          `mapstructure:"url"`
	Token     string          `mapstructure:"token"`
	REST      RESTConfig      `mapstructure:"rest"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
}

// RESTConfig holds REST API client settings.
type RESTConfig struct {
	// RateLimit is the number of requests per second (0 = unlimited, default: 10)
	RateLimit float64 `mapstructure:"rate_limit"`
	// RateBurst is the maximum burst size for rate limiting (default: 5)
	RateBurst int `mapstructure:"rate_burst"`
	// MaxRetries is the maximum number of retry attempts (default: 3)
	MaxRetries int `mapstructure:"max_retries"`
	// RetryInitialDelayMs is the initial delay between retries in milliseconds (default: 100)
	RetryInitialDelayMs int `mapstructure:"retry_initial_delay_ms"`
	// RetryMaxDelayMs is the maximum delay between retries in milliseconds (default: 5000)
	RetryMaxDelayMs int `mapstructure:"retry_max_delay_ms"`
}

// WebSocketConfig holds WebSocket client settings.
type WebSocketConfig struct {
	// MaxRetries is the maximum number of retry attempts (default: 3)
	MaxRetries int `mapstructure:"max_retries"`
	// RetryInitialDelayMs is the initial delay between retries in milliseconds (default: 100)
	RetryInitialDelayMs int `mapstructure:"retry_initial_delay_ms"`
	// RetryMaxDelayMs is the maximum delay between retries in milliseconds (default: 5000)
	RetryMaxDelayMs int `mapstructure:"retry_max_delay_ms"`
}

// ServerConfig holds MCP server settings.
type ServerConfig struct {
	Port int `mapstructure:"port"`
}

// setupViper creates and configures a new viper instance with defaults and environment bindings.
// This is the common setup used by all config loading functions.
func setupViper(configFile string) (*viper.Viper, error) {
	loadDotEnv()

	v := viper.New()

	// Set defaults
	v.SetDefault("homeassistant.url", "http://homeassistant.local:8123")
	v.SetDefault("homeassistant.token", "")
	v.SetDefault("homeassistant.rest.rate_limit", 10.0)
	v.SetDefault("homeassistant.rest.rate_burst", 5)
	v.SetDefault("homeassistant.rest.max_retries", 3)
	v.SetDefault("homeassistant.rest.retry_initial_delay_ms", 100)
	v.SetDefault("homeassistant.rest.retry_max_delay_ms", 5000)
	v.SetDefault("homeassistant.websocket.max_retries", 3)
	v.SetDefault("homeassistant.websocket.retry_initial_delay_ms", 100)
	v.SetDefault("homeassistant.websocket.retry_max_delay_ms", 5000)
	v.SetDefault("server.port", 8080)
	v.SetDefault("logging.level", "INFO")

	// Load from config file if specified
	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	// Enable environment variable overrides
	v.SetEnvPrefix("")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind specific env vars to config keys
	mustBindEnv(v, "homeassistant.url", "HA_URL")
	mustBindEnv(v, "homeassistant.token", "HA_TOKEN")
	mustBindEnv(v, "homeassistant.rest.rate_limit", "HA_REST_RATE_LIMIT")
	mustBindEnv(v, "homeassistant.rest.rate_burst", "HA_REST_RATE_BURST")
	mustBindEnv(v, "homeassistant.rest.max_retries", "HA_REST_MAX_RETRIES")
	mustBindEnv(v, "homeassistant.rest.retry_initial_delay_ms", "HA_REST_RETRY_INITIAL_DELAY_MS")
	mustBindEnv(v, "homeassistant.rest.retry_max_delay_ms", "HA_REST_RETRY_MAX_DELAY_MS")
	mustBindEnv(v, "homeassistant.websocket.max_retries", "HA_WS_MAX_RETRIES")
	mustBindEnv(v, "homeassistant.websocket.retry_initial_delay_ms", "HA_WS_RETRY_INITIAL_DELAY_MS")
	mustBindEnv(v, "homeassistant.websocket.retry_max_delay_ms", "HA_WS_RETRY_MAX_DELAY_MS")
	mustBindEnv(v, "server.port", "HA_MCP_PORT")
	mustBindEnv(v, "logging.level", "HA_MCP_LOG_LEVEL")

	return v, nil
}

// Load loads configuration from YAML file, environment variables, and CLI flags.
// Priority: CLI flags > ENV vars > .env file > YAML file > defaults.
// The configFile parameter is the path to the YAML config file (can be empty).
func Load(configFile string) (*Config, error) {
	v, err := setupViper(configFile)
	if err != nil {
		return nil, err
	}

	// Unmarshal into struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Validate required fields
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// BindFlags binds cobra flags to viper configuration.
// Call this after parsing flags but before Load().
func BindFlags(v *viper.Viper, haURL, haToken string, port int) {
	if haURL != "" {
		v.Set("homeassistant.url", haURL)
	}
	if haToken != "" {
		v.Set("homeassistant.token", haToken)
	}
	if port != 0 {
		v.Set("server.port", port)
	}
}

// LoadWithViper loads configuration using a pre-configured viper instance.
// This allows CLI flags to be bound before loading.
func LoadWithViper(v *viper.Viper, configFile string) (*Config, error) {
	loadDotEnv()

	// Set defaults
	v.SetDefault("homeassistant.url", "http://homeassistant.local:8123")
	v.SetDefault("homeassistant.token", "")
	v.SetDefault("homeassistant.rest.rate_limit", 10.0)
	v.SetDefault("homeassistant.rest.rate_burst", 5)
	v.SetDefault("homeassistant.rest.max_retries", 3)
	v.SetDefault("homeassistant.rest.retry_initial_delay_ms", 100)
	v.SetDefault("homeassistant.rest.retry_max_delay_ms", 5000)
	v.SetDefault("homeassistant.websocket.max_retries", 3)
	v.SetDefault("homeassistant.websocket.retry_initial_delay_ms", 100)
	v.SetDefault("homeassistant.websocket.retry_max_delay_ms", 5000)
	v.SetDefault("server.port", 8080)
	v.SetDefault("logging.level", "INFO")

	// Load from config file if specified
	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	// Enable environment variable overrides
	v.SetEnvPrefix("")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind specific env vars to config keys
	mustBindEnv(v, "homeassistant.url", "HA_URL")
	mustBindEnv(v, "homeassistant.token", "HA_TOKEN")
	mustBindEnv(v, "homeassistant.rest.rate_limit", "HA_REST_RATE_LIMIT")
	mustBindEnv(v, "homeassistant.rest.rate_burst", "HA_REST_RATE_BURST")
	mustBindEnv(v, "homeassistant.rest.max_retries", "HA_REST_MAX_RETRIES")
	mustBindEnv(v, "homeassistant.rest.retry_initial_delay_ms", "HA_REST_RETRY_INITIAL_DELAY_MS")
	mustBindEnv(v, "homeassistant.rest.retry_max_delay_ms", "HA_REST_RETRY_MAX_DELAY_MS")
	mustBindEnv(v, "homeassistant.websocket.max_retries", "HA_WS_MAX_RETRIES")
	mustBindEnv(v, "homeassistant.websocket.retry_initial_delay_ms", "HA_WS_RETRY_INITIAL_DELAY_MS")
	mustBindEnv(v, "homeassistant.websocket.retry_max_delay_ms", "HA_WS_RETRY_MAX_DELAY_MS")
	mustBindEnv(v, "server.port", "HA_MCP_PORT")
	mustBindEnv(v, "logging.level", "HA_MCP_LOG_LEVEL")

	// Unmarshal into struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Validate required fields
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadForDisplay loads configuration without validation, for display purposes.
// This allows showing the effective configuration even if required fields are missing.
func LoadForDisplay(configFile string) (*Config, error) {
	v, err := setupViper(configFile)
	if err != nil {
		return nil, err
	}

	// Unmarshal into struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Skip validation for display purposes
	return cfg, nil
}

// MaskedConfig returns a copy of the config with sensitive data masked.
func (c *Config) MaskedConfig() Config {
	masked := *c
	if masked.HomeAssistant.Token != "" {
		masked.HomeAssistant.Token = maskToken(masked.HomeAssistant.Token)
	}
	return masked
}

// maskToken masks a token, showing only the first 4 and last 4 characters.
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// validate checks that all required configuration is present.
func (c *Config) validate() error {
	if c.HomeAssistant.URL == "" {
		return fmt.Errorf("homeassistant.url is required")
	}
	// Token is optional - can come from request header via Authorization: Bearer <token>
	// When not configured, clients must provide token in each request
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	return nil
}
