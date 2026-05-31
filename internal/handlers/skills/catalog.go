// Package skills provides embedded ha-mcp skill guidance for MCP resource delivery.
// Each skill is a standalone markdown file embedded into the binary at compile time.
package skills

import (
	"embed"
	"fmt"
)

//go:embed *.md
var content embed.FS

// URIPrefix is the base URI scheme for all ha-mcp skill resources.
const URIPrefix = "skill://ha-mcp/"

// SkillMeta describes a single skill exposed as an MCP resource.
type SkillMeta struct {
	Slug        string
	Name        string
	Description string
}

// URI returns the MCP resource URI for this skill.
func (m SkillMeta) URI() string {
	return URIPrefix + m.Slug
}

// Catalog is the ordered list of all ha-mcp skills.
// Slug must be lowercase-kebab and match the corresponding *.md filename exactly.
var Catalog = []SkillMeta{
	{
		Slug:        "format-selection",
		Name:        "Format Selection",
		Description: "When to use format=natural (default) vs. format=json — avoid the common token-waste pattern.",
	},
	{
		Slug:        "automation-patterns",
		Name:        "Automation Patterns",
		Description: "Mode selection (single/restart/queued/parallel), trigger IDs, motion+timer pattern, conditions vs. templates.",
	},
	{
		Slug:        "template-resilience",
		Name:        "Template Resilience",
		Description: "has_value() guards, unavailable/unknown handling, validating templates with render_template before deploy.",
	},
	{
		Slug:        "helper-selection",
		Name:        "Helper Selection",
		Description: "Decision matrix for 26 helper types; id vs. name parameter rules; built-in vs. template sensor trade-offs.",
	},
	{
		Slug:        "dashboard-safety",
		Name:        "Dashboard Safety",
		Description: "Backup-first pattern, large-config truncation risk, when to use the HA UI Raw Config Editor vs. the API.",
	},
	{
		Slug:        "entity-renaming",
		Name:        "Entity Renaming",
		Description: "Safe rename workflow, area/label patterns, manage_entity update quirks and slugify traps.",
	},
	{
		Slug:        "debugging-workflow",
		Name:        "Debugging Workflow",
		Description: "get_logbook(mode=correlation) for timing analysis, batch get_state, trace inspection, system log triage.",
	},
}

// Read returns the markdown content for the given skill slug.
// Returns an error if the slug is not in the embedded filesystem.
func Read(slug string) (string, error) {
	b, err := content.ReadFile(slug + ".md")
	if err != nil {
		return "", fmt.Errorf("unknown skill %q", slug)
	}
	return string(b), nil
}
