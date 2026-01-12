// Package config provides configuration loading for the ha-mcp server.
package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/viper"
)

// resetLoadEnvOnce resets the sync.Once for testing purposes.
// This is necessary because loadDotEnv uses sync.Once which persists across tests.
func resetLoadEnvOnce() {
	loadEnvOnce = sync.Once{}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
		envVars    map[string]string
		wantErr    bool
		errContain string
	}{
		{
			name:       "valid config from env vars",
			configFile: "",
			envVars: map[string]string{
				"HA_URL":   "http://test.local:8123",
				"HA_TOKEN": "test-token-12345678",
			},
			wantErr: false,
		},
		{
			name:       "missing token",
			configFile: "",
			envVars: map[string]string{
				"HA_URL": "http://test.local:8123",
			},
			wantErr:    true,
			errContain: "homeassistant.token is required",
		},
		{
			name:       "missing URL uses default",
			configFile: "",
			envVars: map[string]string{
				"HA_TOKEN": "test-token-12345678",
			},
			wantErr: false,
		},
		{
			name:       "invalid port from env",
			configFile: "",
			envVars: map[string]string{
				"HA_URL":      "http://test.local:8123",
				"HA_TOKEN":    "test-token-12345678",
				"HA_MCP_PORT": "99999",
			},
			wantErr:    true,
			errContain: "server.port must be between 1 and 65535",
		},
		{
			name:       "negative port from env",
			configFile: "",
			envVars: map[string]string{
				"HA_URL":      "http://test.local:8123",
				"HA_TOKEN":    "test-token-12345678",
				"HA_MCP_PORT": "-1",
			},
			wantErr:    true,
			errContain: "server.port must be between 1 and 65535",
		},
		{
			name:       "non-existent config file",
			configFile: "/non/existent/config.yaml",
			envVars:    map[string]string{},
			wantErr:    true,
			errContain: "reading config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset sync.Once for each test
			resetLoadEnvOnce()

			// Clear and set environment variables
			clearEnvVars()
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg, err := Load(tt.configFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Load() error = nil, wantErr = true")
					return
				}
				if tt.errContain != "" && !containsString(err.Error(), tt.errContain) {
					t.Errorf("Load() error = %v, want error containing %q", err, tt.errContain)
				}
				return
			}

			if err != nil {
				t.Errorf("Load() unexpected error = %v", err)
				return
			}

			if cfg == nil {
				t.Errorf("Load() returned nil config without error")
			}
		})
	}
}

func TestLoadWithConfigFile(t *testing.T) {
	resetLoadEnvOnce()
	clearEnvVars()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
homeassistant:
  url: "http://yaml-test.local:8123"
  token: "yaml-token-12345678"
server:
  port: 9090
logging:
  level: "debug"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HomeAssistant.URL != "http://yaml-test.local:8123" {
		t.Errorf("URL = %q, want %q", cfg.HomeAssistant.URL, "http://yaml-test.local:8123")
	}
	if cfg.HomeAssistant.Token != "yaml-token-12345678" {
		t.Errorf("Token = %q, want %q", cfg.HomeAssistant.Token, "yaml-token-12345678")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Level = %q, want %q", cfg.Logging.Level, "debug")
	}
}

func TestLoadForDisplay(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
		envVars    map[string]string
		wantErr    bool
	}{
		{
			name:       "loads without validation - missing token allowed",
			configFile: "",
			envVars: map[string]string{
				"HA_URL": "http://test.local:8123",
				// No token - should still work for display
			},
			wantErr: false,
		},
		{
			name:       "loads with all values",
			configFile: "",
			envVars: map[string]string{
				"HA_URL":   "http://test.local:8123",
				"HA_TOKEN": "display-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetLoadEnvOnce()
			clearEnvVars()
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg, err := LoadForDisplay(tt.configFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadForDisplay() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("LoadForDisplay() unexpected error = %v", err)
				return
			}

			if cfg == nil {
				t.Errorf("LoadForDisplay() returned nil config without error")
			}
		})
	}
}

func TestLoadWithViper(t *testing.T) {
	tests := []struct {
		name       string
		setupViper func(*viper.Viper)
		configFile string
		wantErr    bool
		errContain string
	}{
		{
			name: "valid pre-configured viper",
			setupViper: func(v *viper.Viper) {
				v.Set("homeassistant.url", "http://viper-test.local:8123")
				v.Set("homeassistant.token", "viper-token-12345678")
				v.Set("server.port", 8888)
			},
			configFile: "",
			wantErr:    false,
		},
		{
			name: "missing token in viper",
			setupViper: func(v *viper.Viper) {
				v.Set("homeassistant.url", "http://viper-test.local:8123")
			},
			configFile: "",
			wantErr:    true,
			errContain: "homeassistant.token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetLoadEnvOnce()
			clearEnvVars()

			v := viper.New()
			if tt.setupViper != nil {
				tt.setupViper(v)
			}

			cfg, err := LoadWithViper(v, tt.configFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadWithViper() error = nil, wantErr = true")
					return
				}
				if tt.errContain != "" && !containsString(err.Error(), tt.errContain) {
					t.Errorf("LoadWithViper() error = %v, want error containing %q", err, tt.errContain)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadWithViper() unexpected error = %v", err)
				return
			}

			if cfg == nil {
				t.Errorf("LoadWithViper() returned nil config without error")
			}
		})
	}
}

func TestLoadWithViperConfigFile(t *testing.T) {
	resetLoadEnvOnce()
	clearEnvVars()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
homeassistant:
  url: "http://viper-yaml.local:8123"
  token: "viper-yaml-token"
server:
  port: 7777
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	v := viper.New()
	cfg, err := LoadWithViper(v, configPath)
	if err != nil {
		t.Fatalf("LoadWithViper() error = %v", err)
	}

	if cfg.Server.Port != 7777 {
		t.Errorf("Port = %d, want %d", cfg.Server.Port, 7777)
	}
}

func TestBindFlags(t *testing.T) {
	tests := []struct {
		name    string
		haURL   string
		haToken string
		port    int
		wantURL string
		wantTok string
		wantPrt int
	}{
		{
			name:    "all flags set",
			haURL:   "http://flag.local:8123",
			haToken: "flag-token",
			port:    9999,
			wantURL: "http://flag.local:8123",
			wantTok: "flag-token",
			wantPrt: 9999,
		},
		{
			name:    "only URL set",
			haURL:   "http://only-url.local:8123",
			haToken: "",
			port:    0,
			wantURL: "http://only-url.local:8123",
			wantTok: "",
			wantPrt: 0,
		},
		{
			name:    "only token set",
			haURL:   "",
			haToken: "only-token",
			port:    0,
			wantURL: "",
			wantTok: "only-token",
			wantPrt: 0,
		},
		{
			name:    "only port set",
			haURL:   "",
			haToken: "",
			port:    1234,
			wantURL: "",
			wantTok: "",
			wantPrt: 1234,
		},
		{
			name:    "nothing set",
			haURL:   "",
			haToken: "",
			port:    0,
			wantURL: "",
			wantTok: "",
			wantPrt: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()

			BindFlags(v, tt.haURL, tt.haToken, tt.port)

			if tt.wantURL != "" {
				got := v.GetString("homeassistant.url")
				if got != tt.wantURL {
					t.Errorf("URL = %q, want %q", got, tt.wantURL)
				}
			}
			if tt.wantTok != "" {
				got := v.GetString("homeassistant.token")
				if got != tt.wantTok {
					t.Errorf("Token = %q, want %q", got, tt.wantTok)
				}
			}
			if tt.wantPrt != 0 {
				got := v.GetInt("server.port")
				if got != tt.wantPrt {
					t.Errorf("Port = %d, want %d", got, tt.wantPrt)
				}
			}
		})
	}
}

func TestMaskedConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantToken string
	}{
		{
			name: "long token is masked",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "abcdefghijklmnopqrstuvwxyz",
				},
				Server:  ServerConfig{Port: 8080},
				Logging: LoggingConfig{Level: "info"},
			},
			wantToken: "abcd****wxyz",
		},
		{
			name: "short token becomes ****",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "short",
				},
				Server:  ServerConfig{Port: 8080},
				Logging: LoggingConfig{Level: "info"},
			},
			wantToken: "****",
		},
		{
			name: "exactly 8 char token becomes ****",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "12345678",
				},
				Server:  ServerConfig{Port: 8080},
				Logging: LoggingConfig{Level: "info"},
			},
			wantToken: "****",
		},
		{
			name: "9 char token is masked",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "123456789",
				},
				Server:  ServerConfig{Port: 8080},
				Logging: LoggingConfig{Level: "info"},
			},
			wantToken: "1234****6789",
		},
		{
			name: "empty token stays empty",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "",
				},
				Server:  ServerConfig{Port: 8080},
				Logging: LoggingConfig{Level: "info"},
			},
			wantToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := tt.config.MaskedConfig()

			if masked.HomeAssistant.Token != tt.wantToken {
				t.Errorf("MaskedConfig().Token = %q, want %q", masked.HomeAssistant.Token, tt.wantToken)
			}

			// Verify other fields are unchanged
			if masked.HomeAssistant.URL != tt.config.HomeAssistant.URL {
				t.Errorf("MaskedConfig().URL = %q, want %q", masked.HomeAssistant.URL, tt.config.HomeAssistant.URL)
			}
			if masked.Server.Port != tt.config.Server.Port {
				t.Errorf("MaskedConfig().Port = %d, want %d", masked.Server.Port, tt.config.Server.Port)
			}
			if masked.Logging.Level != tt.config.Logging.Level {
				t.Errorf("MaskedConfig().Level = %q, want %q", masked.Logging.Level, tt.config.Logging.Level)
			}
		})
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "empty token",
			token: "",
			want:  "****",
		},
		{
			name:  "1 char",
			token: "a",
			want:  "****",
		},
		{
			name:  "8 chars exactly",
			token: "abcdefgh",
			want:  "****",
		},
		{
			name:  "9 chars - first masking",
			token: "abcdefghi",
			want:  "abcd****fghi",
		},
		{
			name:  "16 chars",
			token: "abcdefghijklmnop",
			want:  "abcd****mnop",
		},
		{
			name:  "long token",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJob21lYXNzaXN0YW50",
			want:  "eyJh****YW50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskToken(tt.token)
			if got != tt.want {
				t.Errorf("maskToken(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		wantErr    bool
		errContain string
	}{
		{
			name: "valid config",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "valid-token",
				},
				Server:  ServerConfig{Port: 8080},
				Logging: LoggingConfig{Level: "info"},
			},
			wantErr: false,
		},
		{
			name: "empty URL",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "",
					Token: "valid-token",
				},
				Server:  ServerConfig{Port: 8080},
				Logging: LoggingConfig{Level: "info"},
			},
			wantErr:    true,
			errContain: "homeassistant.url is required",
		},
		{
			name: "empty token",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "",
				},
				Server:  ServerConfig{Port: 8080},
				Logging: LoggingConfig{Level: "info"},
			},
			wantErr:    true,
			errContain: "homeassistant.token is required",
		},
		{
			name: "port 0",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "valid-token",
				},
				Server:  ServerConfig{Port: 0},
				Logging: LoggingConfig{Level: "info"},
			},
			wantErr:    true,
			errContain: "server.port must be between 1 and 65535",
		},
		{
			name: "negative port",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "valid-token",
				},
				Server:  ServerConfig{Port: -1},
				Logging: LoggingConfig{Level: "info"},
			},
			wantErr:    true,
			errContain: "server.port must be between 1 and 65535",
		},
		{
			name: "port too high",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "valid-token",
				},
				Server:  ServerConfig{Port: 65536},
				Logging: LoggingConfig{Level: "info"},
			},
			wantErr:    true,
			errContain: "server.port must be between 1 and 65535",
		},
		{
			name: "port at lower boundary (1)",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "valid-token",
				},
				Server:  ServerConfig{Port: 1},
				Logging: LoggingConfig{Level: "info"},
			},
			wantErr: false,
		},
		{
			name: "port at upper boundary (65535)",
			config: Config{
				HomeAssistant: HomeAssistantConfig{
					URL:   "http://test.local:8123",
					Token: "valid-token",
				},
				Server:  ServerConfig{Port: 65535},
				Logging: LoggingConfig{Level: "info"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("validate() error = nil, wantErr = true")
					return
				}
				if tt.errContain != "" && !containsString(err.Error(), tt.errContain) {
					t.Errorf("validate() error = %v, want error containing %q", err, tt.errContain)
				}
				return
			}

			if err != nil {
				t.Errorf("validate() unexpected error = %v", err)
			}
		})
	}
}

func TestSetupViper(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
		wantErr    bool
	}{
		{
			name:       "no config file",
			configFile: "",
			wantErr:    false,
		},
		{
			name:       "non-existent config file",
			configFile: "/path/to/nonexistent.yaml",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetLoadEnvOnce()
			clearEnvVars()

			v, err := setupViper(tt.configFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("setupViper() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("setupViper() unexpected error = %v", err)
				return
			}

			if v == nil {
				t.Errorf("setupViper() returned nil viper without error")
				return
			}

			// Verify defaults are set
			if v.GetString("homeassistant.url") != "http://homeassistant.local:8123" {
				t.Errorf("default URL not set correctly")
			}
			if v.GetInt("server.port") != 8080 {
				t.Errorf("default port not set correctly")
			}
			if v.GetString("logging.level") != "INFO" {
				t.Errorf("default logging level not set correctly")
			}
		})
	}
}

func TestSetupViperWithValidConfigFile(t *testing.T) {
	resetLoadEnvOnce()
	clearEnvVars()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
homeassistant:
  url: "http://setup-test.local:8123"
  token: "setup-token"
server:
  port: 5555
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	v, err := setupViper(configPath)
	if err != nil {
		t.Fatalf("setupViper() error = %v", err)
	}

	if v.GetString("homeassistant.url") != "http://setup-test.local:8123" {
		t.Errorf("URL from config file not loaded correctly")
	}
	if v.GetInt("server.port") != 5555 {
		t.Errorf("Port from config file not loaded correctly")
	}
}

func TestConfigDefaults(t *testing.T) {
	resetLoadEnvOnce()
	clearEnvVars()
	t.Setenv("HA_TOKEN", "test-token-for-defaults")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check default values
	if cfg.HomeAssistant.URL != "http://homeassistant.local:8123" {
		t.Errorf("Default URL = %q, want %q", cfg.HomeAssistant.URL, "http://homeassistant.local:8123")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Default Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Logging.Level != "INFO" {
		t.Errorf("Default Level = %q, want %q", cfg.Logging.Level, "INFO")
	}
}

func TestEnvVarOverrides(t *testing.T) {
	resetLoadEnvOnce()
	clearEnvVars()

	t.Setenv("HA_URL", "http://env-override.local:8123")
	t.Setenv("HA_TOKEN", "env-override-token")
	t.Setenv("HA_MCP_PORT", "3333")
	t.Setenv("HA_MCP_LOG_LEVEL", "debug")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HomeAssistant.URL != "http://env-override.local:8123" {
		t.Errorf("URL = %q, want %q", cfg.HomeAssistant.URL, "http://env-override.local:8123")
	}
	if cfg.HomeAssistant.Token != "env-override-token" {
		t.Errorf("Token = %q, want %q", cfg.HomeAssistant.Token, "env-override-token")
	}
	if cfg.Server.Port != 3333 {
		t.Errorf("Port = %d, want %d", cfg.Server.Port, 3333)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Level = %q, want %q", cfg.Logging.Level, "debug")
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		HomeAssistant: HomeAssistantConfig{
			URL:   "http://test.local:8123",
			Token: "test-token",
		},
		Server: ServerConfig{
			Port: 9000,
		},
		Logging: LoggingConfig{
			Level: "warn",
		},
	}

	want := Config{
		HomeAssistant: HomeAssistantConfig{
			URL:   "http://test.local:8123",
			Token: "test-token",
		},
		Server: ServerConfig{
			Port: 9000,
		},
		Logging: LoggingConfig{
			Level: "warn",
		},
	}

	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("Config mismatch (-want +got):\n%s", diff)
	}
}

// Helper functions

func clearEnvVars() {
	envVars := []string{"HA_URL", "HA_TOKEN", "HA_MCP_PORT", "HA_MCP_LOG_LEVEL"}
	for _, v := range envVars {
		_ = os.Unsetenv(v)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
