package handlers

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

const (
	// DefaultMaxLimit is the maximum allowed limit for pagination.
	DefaultMaxLimit = 1000
	// NoLimit indicates no limit is applied (backwards compatible).
	NoLimit = 0
)

// PaginationParams holds pagination parameters extracted from tool args.
type PaginationParams struct {
	Limit       int
	Offset      int
	Cursor      string
	FiltersHash string
}

// CursorData holds the cursor structure for pagination.
type CursorData struct {
	Offset      int    `json:"offset"`
	Limit       int    `json:"limit"`
	FiltersHash string `json:"hash"`
}

// PaginationMetadata contains pagination response metadata.
type PaginationMetadata struct {
	Total      int     `json:"total"`
	Count      int     `json:"count"`
	Offset     int     `json:"offset"`
	Limit      int     `json:"limit"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor,omitempty"`
}

// PaginatedResponse wraps items with pagination metadata.
type PaginatedResponse[T any] struct {
	Items      []T                `json:"items"`
	Pagination PaginationMetadata `json:"pagination"`
}

// ParsePaginationParams extracts pagination parameters from tool arguments.
// If cursor is provided, it decodes the cursor and validates the filters hash.
// filtersForHash should be a map of the current filter values to detect filter changes.
func ParsePaginationParams(args, filtersForHash map[string]any) (PaginationParams, error) {
	params := PaginationParams{}

	// Extract cursor if present
	if cursor, ok := args["cursor"].(string); ok && cursor != "" {
		params.Cursor = cursor
		decoded, err := DecodeCursor(cursor)
		if err != nil {
			return params, fmt.Errorf("invalid cursor: %w", err)
		}
		params.Offset = decoded.Offset
		params.Limit = decoded.Limit
		params.FiltersHash = decoded.FiltersHash

		// Validate filters hash if filters were provided
		if filtersForHash != nil {
			currentHash := ComputeFiltersHash(filtersForHash)
			if currentHash != params.FiltersHash {
				return params, fmt.Errorf("cursor invalid: filters have changed since cursor was created")
			}
		}
		return params, nil
	}

	// Extract limit using type switch
	switch v := args["limit"].(type) {
	case float64:
		params.Limit = int(v)
	case int:
		params.Limit = v
	}

	// Enforce max limit
	if params.Limit > DefaultMaxLimit {
		params.Limit = DefaultMaxLimit
	}

	// Extract offset using type switch
	switch v := args["offset"].(type) {
	case float64:
		params.Offset = int(v)
	case int:
		params.Offset = v
	}

	// Ensure non-negative values
	if params.Offset < 0 {
		params.Offset = 0
	}
	if params.Limit < 0 {
		params.Limit = NoLimit
	}

	// Compute filters hash for cursor creation
	if filtersForHash != nil {
		params.FiltersHash = ComputeFiltersHash(filtersForHash)
	}

	return params, nil
}

// DecodeCursor decodes a base64-encoded cursor string.
func DecodeCursor(cursor string) (*CursorData, error) {
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}

	var data CursorData
	if err := json.Unmarshal(decoded, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor: %w", err)
	}

	return &data, nil
}

// EncodeCursor creates a base64-encoded cursor string.
func EncodeCursor(offset, limit int, filtersHash string) string {
	data := CursorData{
		Offset:      offset,
		Limit:       limit,
		FiltersHash: filtersHash,
	}

	jsonBytes, _ := json.Marshal(data)
	return base64.URLEncoding.EncodeToString(jsonBytes)
}

// ComputeFiltersHash computes a hash of the filter values for consistency checking.
func ComputeFiltersHash(filters map[string]any) string {
	if len(filters) == 0 {
		return ""
	}

	// Sort keys for deterministic hashing
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build ordered representation
	ordered := make([]any, 0, len(filters)*2)
	for _, k := range keys {
		ordered = append(ordered, k, filters[k])
	}

	jsonBytes, _ := json.Marshal(ordered)
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter hash
}

// ApplyPagination applies pagination to a slice of items.
// Returns a PaginatedResponse with the paginated items and metadata.
// If limit is 0 (NoLimit), returns all items.
func ApplyPagination[T any](items []T, params PaginationParams) PaginatedResponse[T] {
	total := len(items)

	// No limit means return all items
	if params.Limit == NoLimit {
		return PaginatedResponse[T]{
			Items: items,
			Pagination: PaginationMetadata{
				Total:   total,
				Count:   total,
				Offset:  0,
				Limit:   0,
				HasMore: false,
			},
		}
	}

	// Apply offset
	start := params.Offset
	if start > total {
		start = total
	}

	// Apply limit
	end := start + params.Limit
	if end > total {
		end = total
	}

	paginatedItems := items[start:end]
	hasMore := end < total

	var nextCursor *string
	if hasMore {
		cursor := EncodeCursor(end, params.Limit, params.FiltersHash)
		nextCursor = &cursor
	}

	return PaginatedResponse[T]{
		Items: paginatedItems,
		Pagination: PaginationMetadata{
			Total:      total,
			Count:      len(paginatedItems),
			Offset:     start,
			Limit:      params.Limit,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		},
	}
}

// BuildPaginationSummary creates a human-readable summary of pagination state.
func BuildPaginationSummary(meta PaginationMetadata, itemType string) string {
	if meta.Limit == 0 {
		return fmt.Sprintf("Found %d %s", meta.Total, itemType)
	}

	summary := fmt.Sprintf("Showing %d of %d %s", meta.Count, meta.Total, itemType)
	if meta.HasMore {
		summary += fmt.Sprintf(" (offset %d, limit %d, more available)", meta.Offset, meta.Limit)
	}
	return summary
}

// PaginationSchemaProperties returns the standard pagination InputSchema properties.
func PaginationSchemaProperties() map[string]any {
	return map[string]any{
		"limit": map[string]any{
			"type":        "integer",
			"description": fmt.Sprintf("Maximum number of items to return (max %d, default: no limit)", DefaultMaxLimit),
		},
		"cursor": map[string]any{
			"type":        "string",
			"description": "Pagination cursor from previous response to get next page",
		},
	}
}
