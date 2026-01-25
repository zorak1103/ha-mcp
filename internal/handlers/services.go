// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// ServiceHandlers provides handlers for Home Assistant service discovery operations.
type ServiceHandlers struct{}

// NewServiceHandlers creates a new ServiceHandlers instance.
func NewServiceHandlers() *ServiceHandlers {
	return &ServiceHandlers{}
}

// RegisterTools registers all service-related tools with the registry.
func (h *ServiceHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.listServicesTool(), h.handleListServices)
}

// listServicesTool returns the tool definition for listing services.
func (h *ServiceHandlers) listServicesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_services",
		Description: "List all available Home Assistant services. Services define actions that can be performed on entities. Use 'domain' filter to get services for a specific domain (e.g., 'light', 'switch', 'climate').",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Filter options for services list",
			Properties: map[string]mcp.JSONSchema{
				"domain": {
					Type:        "string",
					Description: "Filter by domain (e.g., 'light', 'switch', 'climate'). If not specified, returns all domains.",
				},
			},
		},
	}
}

// compactServiceEntry represents a minimal service entry for compact output.
type compactServiceEntry struct {
	Domain       string   `json:"domain"`
	ServiceCount int      `json:"service_count"`
	Services     []string `json:"services"`
}

// serviceDetail represents detailed service information.
type serviceDetail struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Fields      []string `json:"fields,omitempty"`
	HasTarget   bool     `json:"has_target,omitempty"`
}

// handleListServices handles requests to list available services.
func (h *ServiceHandlers) handleListServices(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	services, err := client.GetServices(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting services: %v", err)),
			},
			IsError: true,
		}, nil
	}

	domainFilter := getString(args, "domain")

	if domainFilter != "" {
		return h.handleFilteredDomain(services, domainFilter)
	}

	return h.handleCompactServices(services)
}

// handleFilteredDomain returns detailed services for a specific domain.
func (h *ServiceHandlers) handleFilteredDomain(services []homeassistant.Service, domain string) (*mcp.ToolsCallResult, error) {
	var found *homeassistant.Service
	for i := range services {
		if strings.EqualFold(services[i].Domain, domain) {
			found = &services[i]
			break
		}
	}

	if found == nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("No services found for domain '%s'", domain)),
			},
		}, nil
	}

	details := make(map[string]serviceDetail)
	for name, svc := range found.Services {
		detail := serviceDetail{
			Name:        svc.Name,
			Description: svc.Description,
			HasTarget:   svc.Target != nil,
		}

		if len(svc.Fields) > 0 {
			fields := make([]string, 0, len(svc.Fields))
			for fieldName := range svc.Fields {
				fields = append(fields, fieldName)
			}
			sort.Strings(fields)
			detail.Fields = fields
		}

		details[name] = detail
	}

	output, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	summary := fmt.Sprintf("Domain '%s' has %d service(s)", found.Domain, len(found.Services))
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(summary + "\n\n" + string(output)),
		},
	}, nil
}

// handleCompactServices returns a compact overview of all service domains.
func (h *ServiceHandlers) handleCompactServices(services []homeassistant.Service) (*mcp.ToolsCallResult, error) {
	compact := make([]compactServiceEntry, 0, len(services))

	for _, svc := range services {
		serviceNames := make([]string, 0, len(svc.Services))
		for name := range svc.Services {
			serviceNames = append(serviceNames, name)
		}
		sort.Strings(serviceNames)

		compact = append(compact, compactServiceEntry{
			Domain:       svc.Domain,
			ServiceCount: len(svc.Services),
			Services:     serviceNames,
		})
	}

	sort.Slice(compact, func(i, j int) bool {
		return compact[i].Domain < compact[j].Domain
	})

	output, err := json.MarshalIndent(compact, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	summary := fmt.Sprintf("Found %d service domain(s). Use 'domain' parameter to get detailed service info.", len(compact))
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(summary + "\n\n" + string(output)),
		},
	}, nil
}
