package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Access defaults and capability names. The access section is read from the
// YAML file only (never from env vars or flags), so these are plain strings
// that Validate checks.
const (
	accessDefaultRead  = "read"
	accessDefaultWrite = "write"
	accessDefaultNone  = "none"

	accessCapRead      = "read"
	accessCapControl   = "control"
	accessCapConfigure = "configure"
	accessCapWrite     = "write"
	accessCapNone      = "none"

	accessSystemAll = "all"
)

// accessSystemValues are the valid entries of access.system.
var accessSystemValues = []string{
	"reload", "restart", "backup", "updates", "config_entries", "hacs",
	"addons", "blueprints", "statistics", "logs", "homeassistant", accessSystemAll,
}

// CapList is a list of capability (or system) names. In YAML it may be written
// as a single scalar (`allow: write`) or as a sequence (`allow: [read, control]`).
type CapList []string

// UnmarshalYAML accepts a scalar or a sequence of scalars.
func (c *CapList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*c = CapList{s}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		*c = CapList(list)
		return nil
	default:
		return fmt.Errorf("line %d: expected a single value or a list of values", node.Line)
	}
}

// AccessMatch selects the objects a rule applies to. Within one rule the
// matcher kinds are evaluated by the policy engine; here they are only parsed.
type AccessMatch struct {
	Entities   []string `yaml:"entities"`
	Dashboards []string `yaml:"dashboards"`
	Areas      []string `yaml:"areas"`
	Labels     []string `yaml:"labels"`
	Devices    []string `yaml:"devices"`
}

func (m AccessMatch) isEmpty() bool {
	return len(m.Entities)+len(m.Dashboards)+len(m.Areas)+len(m.Labels)+len(m.Devices) == 0
}

// AccessRule grants or removes capabilities for the objects it matches.
type AccessRule struct {
	Match  AccessMatch `yaml:"match"`
	Allow  CapList     `yaml:"allow"`
	Deny   CapList     `yaml:"deny"`
	Create *bool       `yaml:"create"`
	Delete *bool       `yaml:"delete"`
}

// AccessConfig is the object-level access control section of the config file.
type AccessConfig struct {
	// Default is the capability level of objects no rule matches: read, write or none.
	Default string       `yaml:"default"`
	Rules   []AccessRule `yaml:"rules"`
	// System lists the permitted system operations. nil means the section is
	// absent, which follows Default.
	System *CapList `yaml:"system"`
}

// loadAccess reads the access section from the YAML file at configFile.
// An empty path, a file without the section or an empty file yield the
// default policy (default: read, no rules). The section is decoded strictly
// (unknown keys are errors carrying a line number); unknown keys elsewhere in
// the file are tolerated. Viper is deliberately not used: it lower-cases keys,
// decodes weakly typed and lets environment variables override leaves.
func loadAccess(configFile string) (AccessConfig, error) {
	def := AccessConfig{Default: accessDefaultRead}
	if configFile == "" {
		return def, nil
	}

	data, err := os.ReadFile(configFile) //nolint:gosec // path is the operator-supplied --config file
	if err != nil {
		return AccessConfig{}, fmt.Errorf("reading access config: %w", err)
	}

	var doc struct {
		Access *AccessConfig  `yaml:"access"`
		Rest   map[string]any `yaml:",inline"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return def, nil
		}
		return AccessConfig{}, fmt.Errorf("parsing access section of %s: %w", configFile, err)
	}
	if doc.Access == nil {
		return def, nil
	}
	if doc.Access.Default == "" {
		doc.Access.Default = accessDefaultRead
	}
	if doc.Access.System == nil && systemKeyIsNull(data) {
		empty := CapList{}
		doc.Access.System = &empty
	}
	return *doc.Access, nil
}

// systemKeyIsNull reports whether access.system is present with a null value
// (`system:` with nothing under it, `system: ~`, or all items commented out).
// yaml.v3 skips unmarshalers and leaves a pointer nil for null, which would be
// indistinguishable from an absent key; absent follows access.default, so a
// null section must be treated as present and empty (no system operation
// allowed) instead, or it would fail open under `default: write`. A yaml.Node
// field receives the raw node, null included, and stays zero when the key is
// absent. The data already decoded cleanly in the strict pass.
func systemKeyIsNull(data []byte) bool {
	var raw struct {
		Access *struct {
			System yaml.Node `yaml:"system"`
		} `yaml:"access"`
		Rest map[string]any `yaml:",inline"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil || raw.Access == nil {
		return false
	}
	return raw.Access.System.Kind == yaml.ScalarNode && raw.Access.System.ShortTag() == "!!null"
}

// Validate checks the access section. Invalid rules fail startup; they are
// never skipped. An empty Default is treated as read.
func (a AccessConfig) Validate() error {
	switch a.Default {
	case "", accessDefaultRead, accessDefaultWrite, accessDefaultNone:
	default:
		return fmt.Errorf("access.default %q is invalid: must be one of read, write, none", a.Default)
	}
	for i, r := range a.Rules {
		if err := r.validate(); err != nil {
			return fmt.Errorf("access.rules[%d]: %w", i, err)
		}
	}
	if a.System != nil {
		for _, v := range *a.System {
			if !slices.Contains(accessSystemValues, v) {
				return fmt.Errorf("access.system: unknown value %q (valid: %s)", v, strings.Join(accessSystemValues, ", "))
			}
		}
	}
	return nil
}

func (r AccessRule) validate() error {
	if r.Match.isEmpty() {
		return errors.New("rule has no matcher: set at least one of match.entities, dashboards, areas, labels, devices")
	}
	if len(r.Allow) == 0 && len(r.Deny) == 0 {
		return errors.New("rule has neither allow nor deny")
	}
	if err := r.Match.validate(); err != nil {
		return err
	}
	if err := r.Allow.validateAllow(); err != nil {
		return err
	}
	if err := r.Deny.validateDeny(); err != nil {
		return err
	}
	return r.validateRefinements()
}

// validateRefinements checks that create/delete only appear together with
// configure or write in the same rule's allow.
func (r AccessRule) validateRefinements() error {
	grantsConfigure := slices.Contains(r.Allow, accessCapConfigure) || slices.Contains(r.Allow, accessCapWrite)
	if r.Create != nil && !grantsConfigure {
		return errors.New("create is only valid together with allow: configure or write in the same rule")
	}
	if r.Delete != nil && !grantsConfigure {
		return errors.New("delete is only valid together with allow: configure or write in the same rule")
	}
	return nil
}

func (c CapList) validateAllow() error {
	for _, v := range c {
		switch v {
		case accessCapRead, accessCapControl, accessCapConfigure, accessCapWrite:
		case accessCapNone:
			if len(c) > 1 {
				return errors.New("allow: none cannot be combined with other values")
			}
		default:
			return fmt.Errorf("unknown capability %q in allow (valid: read, control, configure, write, none)", v)
		}
	}
	return nil
}

func (c CapList) validateDeny() error {
	for _, v := range c {
		switch v {
		case accessCapControl, accessCapConfigure, accessCapWrite:
		case accessCapRead, accessCapNone:
			return fmt.Errorf("deny: %s is not supported; use allow: none to hide an object", v)
		default:
			return fmt.Errorf("unknown capability %q in deny (valid: control, configure, write)", v)
		}
	}
	return nil
}

func (m AccessMatch) validate() error {
	for _, g := range m.Entities {
		if g == "" {
			return errors.New("match.entities contains an empty value")
		}
		if g != strings.ToLower(g) {
			return fmt.Errorf("match.entities glob %q must be lowercase (entity ids are lowercase)", g)
		}
		if _, err := path.Match(g, ""); err != nil {
			return fmt.Errorf("match.entities glob %q is invalid: %w", g, err)
		}
	}
	named := []struct {
		name string
		vals []string
	}{{"dashboards", m.Dashboards}, {"areas", m.Areas}, {"labels", m.Labels}, {"devices", m.Devices}}
	for _, n := range named {
		if slices.Contains(n.vals, "") {
			return fmt.Errorf("match.%s contains an empty value", n.name)
		}
	}
	return nil
}
