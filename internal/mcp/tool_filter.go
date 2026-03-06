package mcp

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// ToolFilterConfig holds tool filtering configuration.
type ToolFilterConfig struct {
	Whitelist []string
	Blacklist []string
}

// ToolFilterEngine filters tools based on whitelist/blacklist and read/write categories.
type ToolFilterEngine struct {
	accessMap      map[string]ToolClassification
	blockedTools   map[string]bool            // Completely blocked tools
	allowedActions map[string]map[string]bool // tool -> allowed actions
	enabled        bool
}

// NewToolFilterEngine creates a new tool filter engine.
// If readOnly is true, "*:write" is prepended to the blacklist.
func NewToolFilterEngine(cfg ToolFilterConfig, readOnly bool) *ToolFilterEngine {
	// Early return if filter is disabled
	if !readOnly && len(cfg.Whitelist) == 0 && len(cfg.Blacklist) == 0 {
		return &ToolFilterEngine{enabled: false}
	}

	engine := &ToolFilterEngine{
		accessMap:      buildAccessControlMap(),
		blockedTools:   make(map[string]bool),
		allowedActions: make(map[string]map[string]bool),
		enabled:        true,
	}

	// Prepend *:write to blacklist if read-only
	blacklist := cfg.Blacklist
	if readOnly {
		blacklist = append([]string{"*:write"}, blacklist...)
	}

	// Build allowed actions map
	engine.buildAllowedActionsMap(cfg.Whitelist, blacklist)

	return engine
}

// buildAllowedActionsMap computes which actions are allowed for each tool.
func (f *ToolFilterEngine) buildAllowedActionsMap(whitelist, blacklist []string) {
	// If whitelist is non-empty, only explicitly allowed items pass
	if len(whitelist) > 0 {
		f.buildFromWhitelist(whitelist)
		return
	}

	// Otherwise, start with all allowed and remove blacklisted
	f.buildFromBlacklist(blacklist)
}

// buildFromWhitelist creates allowed actions from whitelist entries.
func (f *ToolFilterEngine) buildFromWhitelist(whitelist []string) {
	for _, entry := range whitelist {
		matches := f.expandFilterEntry(entry)
		for toolName, actions := range matches {
			if _, exists := f.allowedActions[toolName]; !exists {
				f.allowedActions[toolName] = make(map[string]bool)
			}

			// If actions is nil, entire tool is allowed
			if actions == nil {
				// Mark all actions as allowed for this tool
				classification := f.accessMap[toolName]
				if classification.ParamName == "" {
					// Pure tool - entire tool allowed
					f.allowedActions[toolName] = nil
				} else {
					// Action-based tool - allow all actions
					for action := range classification.Actions {
						f.allowedActions[toolName][action] = true
					}
				}
			} else {
				// Specific actions
				for action := range actions {
					f.allowedActions[toolName][action] = true
				}
			}
		}
	}
}

// buildFromBlacklist creates allowed actions by removing blacklisted items.
func (f *ToolFilterEngine) buildFromBlacklist(blacklist []string) {
	f.initializeAllAllowed()
	f.removeBlacklistedItems(blacklist)
}

// initializeAllAllowed marks all tools and actions as initially allowed.
func (f *ToolFilterEngine) initializeAllAllowed() {
	for toolName, classification := range f.accessMap {
		f.allowedActions[toolName] = make(map[string]bool)

		if classification.ParamName == "" {
			f.allowedActions[toolName] = nil
		} else {
			f.initializeToolActions(toolName, classification)
		}
	}
}

// initializeToolActions initializes all actions and sub-actions for a tool.
func (f *ToolFilterEngine) initializeToolActions(toolName string, classification ToolClassification) {
	for action := range classification.Actions {
		f.allowedActions[toolName][action] = true
	}

	for mode, subActions := range classification.SubActions {
		for subAction := range subActions {
			subActionKey := mode + ":" + subAction
			f.allowedActions[toolName][subActionKey] = true
		}
	}
}

// removeBlacklistedItems processes blacklist entries and removes matching items.
func (f *ToolFilterEngine) removeBlacklistedItems(blacklist []string) {
	for _, entry := range blacklist {
		matches := f.expandFilterEntry(entry)
		f.applyBlacklistMatches(matches)
	}
}

// applyBlacklistMatches removes tools/actions matched by blacklist.
func (f *ToolFilterEngine) applyBlacklistMatches(matches map[string]map[string]bool) {
	for toolName, actions := range matches {
		if actions == nil {
			delete(f.allowedActions, toolName)
		} else {
			f.removeToolActions(toolName, actions)
		}
	}
}

// removeToolActions removes specific actions from a tool.
func (f *ToolFilterEngine) removeToolActions(toolName string, actionsToRemove map[string]bool) {
	toolActions, exists := f.allowedActions[toolName]
	if !exists || toolActions == nil {
		return
	}

	for action := range actionsToRemove {
		delete(toolActions, action)
	}

	if len(f.allowedActions[toolName]) == 0 {
		delete(f.allowedActions, toolName)
	}
}

// expandFilterEntry expands a filter entry into matching tools and actions.
// Returns map[toolName]map[actionName]bool, where nil actions means entire tool.
func (f *ToolFilterEngine) expandFilterEntry(entry string) map[string]map[string]bool {
	toolPattern, actionOrCategory, subAction := parseFilterEntry(entry)
	matchingTools := f.findMatchingTools(toolPattern)

	result := make(map[string]map[string]bool)
	for _, toolName := range matchingTools {
		classification := f.accessMap[toolName]
		f.processToolMatch(toolName, classification, actionOrCategory, subAction, result)
	}

	return result
}

// parseFilterEntry parses a filter entry into components.
func parseFilterEntry(entry string) (toolPattern, actionOrCategory, subAction string) {
	parts := strings.SplitN(entry, ":", 2)
	toolPattern = parts[0]

	if len(parts) > 1 {
		actionParts := strings.SplitN(parts[1], ":", 2)
		actionOrCategory = actionParts[0]
		if len(actionParts) > 1 {
			subAction = actionParts[1]
		}
	}

	return toolPattern, actionOrCategory, subAction
}

// processToolMatch processes a single tool match and adds it to result.
func (f *ToolFilterEngine) processToolMatch(
	toolName string,
	classification ToolClassification,
	actionOrCategory, subAction string,
	result map[string]map[string]bool,
) {
	if actionOrCategory == "" {
		result[toolName] = nil
		return
	}

	if isCategoryExpansion(actionOrCategory) {
		f.processCategoryMatch(toolName, classification, actionOrCategory, result)
		return
	}

	if subAction != "" {
		f.processSubActionMatch(toolName, classification, actionOrCategory, subAction, result)
		return
	}

	f.processRegularActionMatch(toolName, classification, actionOrCategory, result)
}

// isCategoryExpansion checks if the action is a category (write/read).
func isCategoryExpansion(action string) bool {
	return action == "write" || action == "read"
}

// processCategoryMatch handles category-based matching (*:write, *:read).
func (f *ToolFilterEngine) processCategoryMatch(
	toolName string,
	classification ToolClassification,
	category string,
	result map[string]map[string]bool,
) {
	actions := f.expandCategory(classification, ActionCategory(category))
	if actions == nil || len(actions) > 0 {
		result[toolName] = actions
	}
}

// processSubActionMatch handles sub-action filtering (query_entities:health:remove).
func (f *ToolFilterEngine) processSubActionMatch(
	toolName string,
	classification ToolClassification,
	mode, subAction string,
	result map[string]map[string]bool,
) {
	if classification.SubActions == nil {
		return
	}

	subActions, exists := classification.SubActions[mode]
	if !exists {
		return
	}

	if _, actionExists := subActions[subAction]; !actionExists {
		return
	}

	if result[toolName] == nil {
		result[toolName] = make(map[string]bool)
	}
	result[toolName][mode+":"+subAction] = true
}

// processRegularActionMatch handles regular action matching.
func (f *ToolFilterEngine) processRegularActionMatch(
	toolName string,
	classification ToolClassification,
	action string,
	result map[string]map[string]bool,
) {
	if classification.Actions == nil {
		return
	}

	if _, exists := classification.Actions[action]; !exists {
		return
	}

	if result[toolName] == nil {
		result[toolName] = make(map[string]bool)
	}
	result[toolName][action] = true
}

// findMatchingTools returns tool names matching the pattern (supports glob).
func (f *ToolFilterEngine) findMatchingTools(pattern string) []string {
	var matches []string

	for toolName := range f.accessMap {
		matched, err := path.Match(pattern, toolName)
		if err == nil && matched {
			matches = append(matches, toolName)
		}
	}

	return matches
}

// expandCategory returns all actions in a tool matching the category.
func (f *ToolFilterEngine) expandCategory(classification ToolClassification, category ActionCategory) map[string]bool {
	actions := make(map[string]bool)

	// Pure tool category check
	if classification.ParamName == "" && classification.PureCategory == category {
		return nil // Entire tool matches
	}

	// Action-based tool
	for action, actionCategory := range classification.Actions {
		if actionCategory == category {
			actions[action] = true
		}
	}

	return actions
}

// ApplyToRegistry removes or modifies tools in the registry based on the filter.
// Returns the number of tools completely removed.
func (f *ToolFilterEngine) ApplyToRegistry(registry *Registry) int {
	if !f.enabled {
		return 0
	}

	removed := 0

	// Determine which tools are completely blocked
	for toolName := range f.accessMap {
		if _, allowed := f.allowedActions[toolName]; !allowed {
			f.blockedTools[toolName] = true
			registry.RemoveTool(toolName)
			removed++
		} else if allowedActions := f.allowedActions[toolName]; allowedActions != nil {
			// Partially blocked - modify tool schema
			if tool, exists := registry.GetTool(toolName); exists {
				classification := f.accessMap[toolName]
				if classification.ParamName != "" {
					modifiedTool := f.modifyToolForFilter(tool, allowedActions, classification.ParamName)
					registry.UpdateTool(toolName, modifiedTool)
				}
			}
		}
	}

	return removed
}

// modifyToolForFilter updates a tool's schema to reflect filtered actions.
func (f *ToolFilterEngine) modifyToolForFilter(tool Tool, allowedActions map[string]bool, paramName string) Tool {
	// Build filtered enum
	var filteredEnum []string
	for action := range allowedActions {
		// Skip sub-action markers
		if !strings.Contains(action, ":") {
			filteredEnum = append(filteredEnum, action)
		}
	}

	// Update action property enum
	if actionProp, exists := tool.InputSchema.Properties[paramName]; exists {
		actionProp.Enum = filteredEnum
		actionProp.Description = fmt.Sprintf("Operation to perform: %s", strings.Join(filteredEnum, ", "))
		tool.InputSchema.Properties[paramName] = actionProp
	}

	// Update tool description
	tool.Description = f.updateToolDescription(tool.Description, allowedActions)

	return tool
}

// updateToolDescription filters the tool description to show only allowed actions.
func (f *ToolFilterEngine) updateToolDescription(desc string, allowedActions map[string]bool) string {
	// Split description into header and actions block
	parts := strings.SplitN(desc, "\n\nActions:\n", 2)
	if len(parts) != 2 {
		// No actions block, just update first line
		return f.updateDescriptionFirstLine(desc, allowedActions)
	}

	header := parts[0]
	actionsBlock := parts[1]

	// Filter actions block
	var filteredLines []string
	lines := strings.Split(actionsBlock, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "- ") {
			continue
		}

		// Extract action name from line like "- create: Create new automation"
		actionStart := strings.Index(line, "- ") + 2
		colonIdx := strings.Index(line[actionStart:], ":")
		if colonIdx == -1 {
			continue
		}

		actionName := line[actionStart : actionStart+colonIdx]
		if allowedActions[actionName] {
			filteredLines = append(filteredLines, line)
		}
	}

	// Rebuild description
	newHeader := f.updateDescriptionFirstLine(header, allowedActions)
	if len(filteredLines) > 0 {
		return newHeader + "\n\nActions:\n" + strings.Join(filteredLines, "\n")
	}

	return newHeader
}

// updateDescriptionFirstLine updates the first line to list only allowed actions.
func (f *ToolFilterEngine) updateDescriptionFirstLine(firstLine string, allowedActions map[string]bool) string {
	// Extract base text before dash
	dashIdx := strings.Index(firstLine, " - ")
	if dashIdx == -1 {
		return firstLine
	}

	baseText := firstLine[:dashIdx]

	// Build action list
	var actions []string
	for action := range allowedActions {
		if !strings.Contains(action, ":") {
			actions = append(actions, action)
		}
	}

	if len(actions) == 0 {
		return baseText
	}

	return fmt.Sprintf("%s - %s.", baseText, strings.Join(actions, ", "))
}

// IsActionAllowed checks if a tool action is allowed at runtime.
func (f *ToolFilterEngine) IsActionAllowed(toolName string, args map[string]any) bool {
	if !f.enabled {
		return true
	}

	// Check if tool is completely blocked
	if f.blockedTools[toolName] {
		return false
	}

	// Tool not in filter = allowed (was not filtered out)
	allowedActions, exists := f.allowedActions[toolName]
	if !exists {
		return false
	}

	// Entire tool allowed (nil map)
	if allowedActions == nil {
		return true
	}

	// Get action/mode from args
	classification := f.accessMap[toolName]
	if classification.ParamName == "" {
		// Pure tool should have been handled above
		return true
	}

	actionValue, _ := args[classification.ParamName].(string)
	if actionValue == "" {
		// No action specified - allow if tool exists in map
		return true
	}

	// Check for sub-action (query_entities mode=health action=remove)
	if classification.SubActions != nil {
		if subActionsMap, exists := classification.SubActions[actionValue]; exists {
			if subActionValue, ok := args["action"].(string); ok && subActionValue != "" {
				// Verify this is a valid sub-action
				if _, validSubAction := subActionsMap[subActionValue]; validSubAction {
					// Check if sub-action is allowed
					subActionKey := actionValue + ":" + subActionValue
					return allowedActions[subActionKey]
				}
			}
		}
	}

	// Check if action is allowed
	return allowedActions[actionValue]
}

// IsEnabled returns whether the filter is active.
func (f *ToolFilterEngine) IsEnabled() bool {
	return f.enabled
}

// ValidateFilterConfig validates all entries in the filter config against the known tool set.
// Returns a combined error listing every invalid entry, or nil if all are valid.
// Must be called before NewToolFilterEngine to catch stale or mistyped config at startup.
func ValidateFilterConfig(cfg ToolFilterConfig) error {
	if len(cfg.Whitelist) == 0 && len(cfg.Blacklist) == 0 {
		return nil
	}

	accessMap := buildAccessControlMap()

	var errs []string
	for _, entry := range cfg.Whitelist {
		if err := validateFilterEntry(entry, accessMap); err != nil {
			errs = append(errs, fmt.Sprintf("  %q: %s", entry, err.Error()))
		}
	}
	for _, entry := range cfg.Blacklist {
		if err := validateFilterEntry(entry, accessMap); err != nil {
			errs = append(errs, fmt.Sprintf("  %q: %s", entry, err.Error()))
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf(
		"tool filter configuration error: invalid entries found:\n%s\n\nfix your config and restart, or run 'ha-mcp config' to inspect the effective configuration",
		strings.Join(errs, "\n"),
	)
}

// validateFilterEntry validates a single filter entry against the access map.
func validateFilterEntry(entry string, accessMap map[string]ToolClassification) error {
	toolPattern, actionOrCategory, subAction := parseFilterEntry(entry)

	// Category expansions (*:write, *:read) are always valid internal keywords.
	if isCategoryExpansion(actionOrCategory) {
		return nil
	}

	matchedTools := filterGlobMatches(toolPattern, accessMap)
	if len(matchedTools) == 0 {
		return fmt.Errorf("no tools match pattern %q", toolPattern)
	}

	// Bare tool name — existence verified above.
	if actionOrCategory == "" {
		return nil
	}

	isGlob := strings.ContainsAny(toolPattern, "*?[")
	if subAction != "" {
		return validateFilterSubAction(toolPattern, actionOrCategory, subAction, matchedTools, accessMap, isGlob)
	}
	return validateFilterAction(toolPattern, actionOrCategory, matchedTools, accessMap, isGlob)
}

// filterGlobMatches returns all tool names in accessMap matching pattern.
func filterGlobMatches(pattern string, accessMap map[string]ToolClassification) []string {
	var matches []string
	for toolName := range accessMap {
		if matched, err := path.Match(pattern, toolName); err == nil && matched {
			matches = append(matches, toolName)
		}
	}
	return matches
}

// validateFilterSubAction checks that a tool:mode:subAction triple is valid.
func validateFilterSubAction(toolPattern, mode, subAction string, matchedTools []string, accessMap map[string]ToolClassification, isGlob bool) error {
	for _, toolName := range matchedTools {
		classification := accessMap[toolName]
		if classification.SubActions == nil {
			continue
		}
		subActionsMap, ok := classification.SubActions[mode]
		if !ok {
			continue
		}
		if _, exists := subActionsMap[subAction]; exists {
			return nil
		}
	}
	if isGlob {
		return fmt.Errorf("action %q not found on any tool matching %q", mode+":"+subAction, toolPattern)
	}
	return fmt.Errorf("tool %q has no sub-action %q for mode %q", matchedTools[0], subAction, mode)
}

// validateFilterAction checks that a tool:action pair is valid.
func validateFilterAction(toolPattern, action string, matchedTools []string, accessMap map[string]ToolClassification, isGlob bool) error {
	for _, toolName := range matchedTools {
		if _, exists := accessMap[toolName].Actions[action]; exists {
			return nil
		}
	}
	if isGlob {
		return fmt.Errorf("action %q not found on any tool matching %q", action, toolPattern)
	}
	toolName := matchedTools[0]
	classification := accessMap[toolName]
	if classification.ParamName == "" {
		return fmt.Errorf("tool %q has no action parameter; use bare tool name", toolName)
	}
	var validActions []string
	for a := range classification.Actions {
		validActions = append(validActions, a)
	}
	sort.Strings(validActions)
	return fmt.Errorf("tool %q has no action %q (valid: %s)", toolName, action, strings.Join(validActions, ", "))
}
