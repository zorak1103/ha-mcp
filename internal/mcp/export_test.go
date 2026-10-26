package mcp

// BuildAccessControlMap exposes the unexported access map to the external mcp_test package,
// which can import internal/handlers (the in-package tests cannot: handlers imports mcp).
var BuildAccessControlMap = buildAccessControlMap
