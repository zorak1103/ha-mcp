package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/skills"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const (
	skillActionList = "list"
	skillActionRead = "read"
	skillMimeType   = "text/markdown"
)

// RegisterAllResources registers all ha-mcp skill:// resources with the registry.
// Each skill is served as an MCP resource with URI skill://ha-mcp/<slug>.
// Resource content is embedded markdown; no HA client call is made.
func RegisterAllResources(registry *mcp.Registry) {
	for _, meta := range skills.Catalog {
		registry.RegisterResource(
			mcp.Resource{
				URI:         meta.URI(),
				Name:        meta.Name,
				Description: meta.Description,
				MimeType:    skillMimeType,
			},
			makeSkillResourceHandler(meta.Slug),
		)
	}
}

// makeSkillResourceHandler returns a ResourceHandler for the given skill slug.
// The handler reads the embedded markdown and returns it as a resource content block.
func makeSkillResourceHandler(slug string) mcp.ResourceHandler {
	return func(_ context.Context, _ homeassistant.Client, uri string) (*mcp.ResourcesReadResult, error) {
		body, err := skills.Read(slug)
		if err != nil {
			return nil, fmt.Errorf("failed to read skill resource %q: %w", uri, err)
		}
		return &mcp.ResourcesReadResult{
			Contents: []mcp.ResourceContent{
				{
					URI:      uri,
					MimeType: skillMimeType,
					Text:     body,
				},
			},
		}, nil
	}
}

// SkillHandlers provides the get_skill tool — a fallback for tool-only MCP clients
// that do not support the resources/list and resources/read protocol methods.
type SkillHandlers struct{}

// NewSkillHandlers creates a new SkillHandlers instance.
func NewSkillHandlers() *SkillHandlers {
	return &SkillHandlers{}
}

// RegisterTools registers the get_skill tool with the registry.
func (h *SkillHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getSkillTool(), h.handleGetSkill)
}

// getSkillTool returns the tool definition for get_skill.
func (h *SkillHandlers) getSkillTool() mcp.Tool {
	return mcp.Tool{
		Name: "get_skill",
		Description: "Retrieve ha-mcp skill guidance. Use action=list to see available topics, " +
			"action=read with skill=<slug> to fetch full guidance for a topic. " +
			"For resource-aware MCP clients, the same content is available as skill://ha-mcp/<slug> resources.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Enum:        []string{skillActionList, skillActionRead},
					Description: "Action: 'list' to see all available skills, 'read' to fetch a specific skill by slug",
				},
				"skill": {
					Type:        "string",
					Description: "Skill slug to read (required for action=read). Use action=list to discover valid slugs.",
				},
				"format": {
					Type:        "string",
					Enum:        []string{formatNatural, formatJSON},
					Description: "Output format: 'natural' for readable text (default), 'json' for structured data",
				},
			},
			Required: []string{"action"},
		},
	}
}

// handleGetSkill dispatches list and read actions.
func (h *SkillHandlers) handleGetSkill(
	_ context.Context,
	_ homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	action := getString(args, "action")
	switch action {
	case skillActionList:
		return h.handleSkillList(args)
	case skillActionRead:
		return h.handleSkillRead(args)
	default:
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("invalid action %q: must be one of [list, read]", action)),
			},
			IsError: true,
		}, nil
	}
}

// handleSkillList returns the skill catalog.
func (h *SkillHandlers) handleSkillList(args map[string]any) (*mcp.ToolsCallResult, error) {
	if getString(args, "format") == formatJSON {
		return h.handleSkillListJSON()
	}
	return h.handleSkillListNatural()
}

func (h *SkillHandlers) handleSkillListNatural() (*mcp.ToolsCallResult, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Available ha-mcp skills (%d total). Use action=read with skill=<slug> for full guidance.\n\n", len(skills.Catalog))
	for _, m := range skills.Catalog {
		fmt.Fprintf(&sb, "- **%s** (`%s`): %s\n", m.Name, m.Slug, m.Description)
	}
	return &mcp.ToolsCallResult{Content: []mcp.ContentBlock{mcp.NewTextContent(sb.String())}}, nil
}

func (h *SkillHandlers) handleSkillListJSON() (*mcp.ToolsCallResult, error) {
	type entry struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URI         string `json:"uri"`
	}
	entries := make([]entry, 0, len(skills.Catalog))
	for _, m := range skills.Catalog {
		entries = append(entries, entry{
			Slug:        m.Slug,
			Name:        m.Name,
			Description: m.Description,
			URI:         m.URI(),
		})
	}
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("error formatting catalog: %v", err))},
			IsError: true,
		}, nil
	}
	return &mcp.ToolsCallResult{Content: []mcp.ContentBlock{mcp.NewTextContent(string(b))}}, nil
}

// handleSkillRead fetches and returns a single skill's markdown.
func (h *SkillHandlers) handleSkillRead(args map[string]any) (*mcp.ToolsCallResult, error) {
	slug := getString(args, "skill")
	if slug == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("skill is required for action=read")},
			IsError: true,
		}, nil
	}
	body, err := skills.Read(slug)
	if err != nil {
		return h.unknownSkillError(slug)
	}
	if getString(args, "format") == formatJSON {
		return h.handleSkillReadJSON(slug, body)
	}
	return &mcp.ToolsCallResult{Content: []mcp.ContentBlock{mcp.NewTextContent(body)}}, nil
}

func (h *SkillHandlers) handleSkillReadJSON(slug, body string) (*mcp.ToolsCallResult, error) {
	var meta skills.SkillMeta
	for _, m := range skills.Catalog {
		if m.Slug == slug {
			meta = m
			break
		}
	}
	if meta.Slug == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("internal: no catalog entry for slug %q", slug))},
			IsError: true,
		}, nil
	}
	type result struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URI         string `json:"uri"`
		Content     string `json:"content"`
	}
	r := result{
		Slug:        meta.Slug,
		Name:        meta.Name,
		Description: meta.Description,
		URI:         meta.URI(),
		Content:     body,
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("error formatting result: %v", err))},
			IsError: true,
		}, nil
	}
	return &mcp.ToolsCallResult{Content: []mcp.ContentBlock{mcp.NewTextContent(string(b))}}, nil
}

func (h *SkillHandlers) unknownSkillError(slug string) (*mcp.ToolsCallResult, error) {
	validSlugs := make([]string, 0, len(skills.Catalog))
	for _, m := range skills.Catalog {
		validSlugs = append(validSlugs, m.Slug)
	}
	msg := fmt.Sprintf("unknown skill %q; valid slugs: %s", slug, strings.Join(validSlugs, ", "))
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(msg)},
		IsError: true,
	}, nil
}
