package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeAccessFile writes content to a temp config file and returns its path.
func writeAccessFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	return p
}

func TestAccess_MissingFileOrSection_Defaults(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		got, err := loadAccess("")
		if err != nil {
			t.Fatalf("loadAccess(\"\") error = %v", err)
		}
		if got.Default != "read" || len(got.Rules) != 0 || got.System != nil {
			t.Errorf("loadAccess(\"\") = %+v, want default read, no rules, no system", got)
		}
	})

	t.Run("file without access section", func(t *testing.T) {
		p := writeAccessFile(t, "homeassistant:\n  url: http://x:8123\nserver:\n  host: localhost\n")
		got, err := loadAccess(p)
		if err != nil {
			t.Fatalf("loadAccess() error = %v", err)
		}
		if got.Default != "read" || len(got.Rules) != 0 || got.System != nil {
			t.Errorf("loadAccess() = %+v, want default read, no rules, no system", got)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		got, err := loadAccess(writeAccessFile(t, ""))
		if err != nil {
			t.Fatalf("loadAccess() error = %v", err)
		}
		if got.Default != "read" {
			t.Errorf("Default = %q, want read", got.Default)
		}
	})

	t.Run("access section without default", func(t *testing.T) {
		p := writeAccessFile(t, "access:\n  rules:\n    - match: {entities: [\"light.*\"]}\n      allow: write\n")
		got, err := loadAccess(p)
		if err != nil {
			t.Fatalf("loadAccess() error = %v", err)
		}
		if got.Default != "read" || len(got.Rules) != 1 {
			t.Errorf("loadAccess() = %+v, want default read and one rule", got)
		}
	})
}

func TestAccess_UnknownRuleKey_ErrorsWithLine(t *testing.T) {
	p := writeAccessFile(t, `access:
  default: read
  rules:
    - match:
        entites: ["light.*"]
      allow: write
`)
	_, err := loadAccess(p)
	if err == nil {
		t.Fatal("loadAccess() error = nil, want unknown key error")
	}
	if !strings.Contains(err.Error(), "entites") {
		t.Errorf("error %q does not name the unknown key", err)
	}
	if !strings.Contains(err.Error(), "line 5") {
		t.Errorf("error %q does not name line 5", err)
	}
}

func TestAccess_UnknownSectionKey_Errors(t *testing.T) {
	_, err := loadAccess(writeAccessFile(t, "access:\n  dfault: write\n"))
	if err == nil || !strings.Contains(err.Error(), "dfault") {
		t.Errorf("loadAccess() error = %v, want unknown key error naming dfault", err)
	}
}

func TestAccess_EnvIgnored(t *testing.T) {
	t.Setenv("ACCESS_DEFAULT", "write")
	t.Setenv("ACCESS_RULES", "x")

	got, err := loadAccess(writeAccessFile(t, "access:\n  default: read\n"))
	if err != nil {
		t.Fatalf("loadAccess() error = %v", err)
	}
	if got.Default != "read" {
		t.Errorf("Default = %q, want read (env must be ignored)", got.Default)
	}

	got, err = loadAccess("")
	if err != nil {
		t.Fatalf("loadAccess(\"\") error = %v", err)
	}
	if got.Default != "read" {
		t.Errorf("Default = %q, want read (env must be ignored)", got.Default)
	}
}

func TestAccess_ExampleConfigParses(t *testing.T) {
	got, err := loadAccess(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatalf("loadAccess(example) error = %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("Validate() on example config = %v", err)
	}
}

func TestAccess_AllowScalarOrList(t *testing.T) {
	p := writeAccessFile(t, `access:
  default: none
  rules:
    - match: {entities: ["light.*"]}
      allow: write
    - match: {areas: [kitchen]}
      allow: [read, control, configure]
      deny: configure
      create: false
      delete: false
  system: [reload, backup]
`)
	got, err := loadAccess(p)
	if err != nil {
		t.Fatalf("loadAccess() error = %v", err)
	}
	if got.Default != "none" || len(got.Rules) != 2 {
		t.Fatalf("loadAccess() = %+v", got)
	}
	if want := (CapList{"write"}); !reflect.DeepEqual(got.Rules[0].Allow, want) {
		t.Errorf("rule 0 allow = %v, want %v", got.Rules[0].Allow, want)
	}
	if want := (CapList{"read", "control", "configure"}); !reflect.DeepEqual(got.Rules[1].Allow, want) {
		t.Errorf("rule 1 allow = %v, want %v", got.Rules[1].Allow, want)
	}
	if want := (CapList{"configure"}); !reflect.DeepEqual(got.Rules[1].Deny, want) {
		t.Errorf("rule 1 deny = %v, want %v", got.Rules[1].Deny, want)
	}
	if got.Rules[1].Create == nil || *got.Rules[1].Create || got.Rules[1].Delete == nil || *got.Rules[1].Delete {
		t.Errorf("rule 1 create/delete = %v/%v, want false/false", got.Rules[1].Create, got.Rules[1].Delete)
	}
	if got.Rules[0].Create != nil || got.Rules[0].Delete != nil {
		t.Errorf("rule 0 create/delete must stay nil when absent")
	}
	if got.System == nil || !reflect.DeepEqual(*got.System, CapList{"reload", "backup"}) {
		t.Errorf("System = %v, want [reload backup]", got.System)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
}

func TestAccess_SystemEmptyListIsPresent(t *testing.T) {
	got, err := loadAccess(writeAccessFile(t, "access:\n  default: write\n  system: []\n"))
	if err != nil {
		t.Fatalf("loadAccess() error = %v", err)
	}
	if got.System == nil || len(*got.System) != 0 {
		t.Errorf("System = %v, want present and empty", got.System)
	}
}

func TestAccess_WrongTypes_Error(t *testing.T) {
	tests := map[string]string{
		"allow mapping":    "access:\n  rules:\n    - match: {entities: [a.b]}\n      allow: {x: y}\n",
		"allow nested seq": "access:\n  rules:\n    - match: {entities: [a.b]}\n      allow: [[read]]\n",
		"rules not list":   "access:\n  rules: nope\n",
		"create not bool":  "access:\n  rules:\n    - match: {entities: [a.b]}\n      allow: write\n      create: maybe\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := loadAccess(writeAccessFile(t, content)); err == nil {
				t.Error("loadAccess() error = nil, want error")
			}
		})
	}
}

func TestAccess_MissingConfigFile_Errors(t *testing.T) {
	if _, err := loadAccess(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Error("loadAccess() error = nil, want error for unreadable file")
	}
}

func TestValidate_Errors(t *testing.T) {
	falseVal := false
	rule := func(r AccessRule) AccessConfig {
		return AccessConfig{Default: "read", Rules: []AccessRule{r}}
	}
	ent := AccessMatch{Entities: []string{"light.*"}}
	sys := func(v ...string) AccessConfig {
		c := CapList(v)
		return AccessConfig{Default: "read", System: &c}
	}

	tests := []struct {
		name string
		cfg  AccessConfig
		want string
	}{
		{"bad default", AccessConfig{Default: "admin"}, "default"},
		{"deny read", rule(AccessRule{Match: ent, Deny: CapList{"read"}}), "allow: none"},
		{"create false without configure", rule(AccessRule{Match: ent, Allow: CapList{"control"}, Create: &falseVal}), "create"},
		{"delete false without allow", rule(AccessRule{Match: ent, Deny: CapList{"control"}, Delete: &falseVal}), "delete"},
		{"no matcher", rule(AccessRule{Allow: CapList{"write"}}), "matcher"},
		{"neither allow nor deny", rule(AccessRule{Match: ent}), "allow"},
		{"unknown capability", rule(AccessRule{Match: ent, Allow: CapList{"admin"}}), "admin"},
		{"unknown deny capability", rule(AccessRule{Match: ent, Deny: CapList{"nuke"}}), "nuke"},
		{"none combined", rule(AccessRule{Match: ent, Allow: CapList{"none", "read"}}), "none"},
		{"unknown system value", sys("reload", "teleport"), "teleport"},
		{"invalid glob", rule(AccessRule{Match: AccessMatch{Entities: []string{"light.[a"}}, Allow: CapList{"read"}}), "light.[a"},
		{"uppercase entity glob", rule(AccessRule{Match: AccessMatch{Entities: []string{"Light.*"}}, Allow: CapList{"read"}}), "Light.*"},
		{"empty matcher value", rule(AccessRule{Match: AccessMatch{Areas: []string{""}}, Allow: CapList{"read"}}), "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate() error = %q, want it to contain %q", err, tt.want)
			}
			if !strings.HasPrefix(err.Error(), "access") {
				t.Errorf("Validate() error = %q, want prefix \"access\"", err)
			}
		})
	}
}

func TestValidate_Valid(t *testing.T) {
	falseVal := false
	all := CapList{"all"}
	valid := []AccessConfig{
		{},
		{Default: "read"},
		{Default: "write", System: &all},
		{Default: "none", Rules: []AccessRule{
			{Match: AccessMatch{Entities: []string{"light.*", "sensor.temp_?"}}, Allow: CapList{"write"}, Create: &falseVal, Delete: &falseVal},
			{Match: AccessMatch{Dashboards: []string{"lovelace-kitchen"}, Labels: []string{"public"}}, Allow: CapList{"none"}},
			{Match: AccessMatch{Devices: []string{"abc123"}}, Allow: CapList{"read", "control"}, Deny: CapList{"configure"}},
		}},
	}
	for i, c := range valid {
		if err := c.Validate(); err != nil {
			t.Errorf("config %d: Validate() = %v", i, err)
		}
	}
}

func TestLoad_InvalidAccessFailsLoad(t *testing.T) {
	resetLoadEnvOnce()
	t.Setenv("HA_URL", "http://ha.local:8123")

	t.Run("semantic error", func(t *testing.T) {
		p := writeAccessFile(t, "access:\n  default: admin\n")
		_, err := Load(p)
		if err == nil || !strings.Contains(err.Error(), "access") {
			t.Errorf("Load() error = %v, want wrapped access error", err)
		}
	})

	t.Run("unknown key", func(t *testing.T) {
		p := writeAccessFile(t, "access:\n  rules:\n    - match: {entites: [a.b]}\n      allow: read\n")
		_, err := Load(p)
		if err == nil || !strings.Contains(err.Error(), "access") {
			t.Errorf("Load() error = %v, want wrapped access error", err)
		}
	})

	t.Run("display load tolerates semantic error", func(t *testing.T) {
		p := writeAccessFile(t, "access:\n  default: admin\n")
		if _, err := LoadForDisplay(p); err != nil {
			t.Errorf("LoadForDisplay() error = %v, want nil (display skips validation)", err)
		}
	})

	t.Run("valid section is loaded", func(t *testing.T) {
		p := writeAccessFile(t, "access:\n  default: none\n")
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Access.Default != "none" {
			t.Errorf("Access.Default = %q, want none", cfg.Access.Default)
		}
	})

	t.Run("no config file", func(t *testing.T) {
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Access.Default != "read" {
			t.Errorf("Access.Default = %q, want read", cfg.Access.Default)
		}
	})
}

func TestAccess_SystemNullIsPresentAndEmpty(t *testing.T) {
	tests := map[string]string{
		"bare key":              "access:\n  default: write\n  system:\n",
		"explicit null":         "access:\n  default: write\n  system: null\n",
		"tilde":                 "access:\n  default: write\n  system: ~\n",
		"all items commented":   "access:\n  default: write\n  system:\n    # - reload\n    # - backup\n",
		"bare key before rules": "access:\n  system:\n  default: write\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := loadAccess(writeAccessFile(t, content))
			if err != nil {
				t.Fatalf("loadAccess() error = %v", err)
			}
			if got.System == nil {
				t.Fatal("System = nil (absent), want present and empty: a null system section must not follow default")
			}
			if len(*got.System) != 0 {
				t.Errorf("System = %v, want empty", *got.System)
			}
		})
	}
}

func TestAccess_SystemAbsentStaysNil(t *testing.T) {
	got, err := loadAccess(writeAccessFile(t, "access:\n  default: write\n"))
	if err != nil {
		t.Fatalf("loadAccess() error = %v", err)
	}
	if got.System != nil {
		t.Errorf("System = %v, want nil when the key is absent", got.System)
	}
}
