package handlers

import (
	"strings"
	"testing"
)

func TestParsePaginationParams(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		filters     map[string]any
		wantLimit   int
		wantOffset  int
		wantErr     bool
		errContains string
	}{
		{
			name:       "empty args returns defaults",
			args:       map[string]any{},
			filters:    nil,
			wantLimit:  0,
			wantOffset: 0,
		},
		{
			name:       "limit from float64",
			args:       map[string]any{"limit": float64(50)},
			filters:    nil,
			wantLimit:  50,
			wantOffset: 0,
		},
		{
			name:       "limit from int",
			args:       map[string]any{"limit": 50},
			filters:    nil,
			wantLimit:  50,
			wantOffset: 0,
		},
		{
			name:       "limit capped at max",
			args:       map[string]any{"limit": float64(2000)},
			filters:    nil,
			wantLimit:  DefaultMaxLimit,
			wantOffset: 0,
		},
		{
			name:       "offset from float64",
			args:       map[string]any{"offset": float64(100)},
			filters:    nil,
			wantLimit:  0,
			wantOffset: 100,
		},
		{
			name:       "negative offset becomes zero",
			args:       map[string]any{"offset": float64(-10)},
			filters:    nil,
			wantLimit:  0,
			wantOffset: 0,
		},
		{
			name:       "negative limit becomes zero",
			args:       map[string]any{"limit": float64(-10)},
			filters:    nil,
			wantLimit:  0,
			wantOffset: 0,
		},
		{
			name:       "both limit and offset",
			args:       map[string]any{"limit": float64(25), "offset": float64(50)},
			filters:    nil,
			wantLimit:  25,
			wantOffset: 50,
		},
		{
			name:        "invalid cursor",
			args:        map[string]any{"cursor": "invalid-base64!@#"},
			filters:     nil,
			wantErr:     true,
			errContains: "invalid cursor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := ParsePaginationParams(tt.args, tt.filters)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if params.Limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", params.Limit, tt.wantLimit)
			}
			if params.Offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", params.Offset, tt.wantOffset)
			}
		})
	}
}

func TestCursorEncodeDecode(t *testing.T) {
	tests := []struct {
		name        string
		offset      int
		limit       int
		filtersHash string
	}{
		{
			name:        "basic cursor",
			offset:      100,
			limit:       50,
			filtersHash: "abc123",
		},
		{
			name:        "zero values",
			offset:      0,
			limit:       0,
			filtersHash: "",
		},
		{
			name:        "large values",
			offset:      999999,
			limit:       1000,
			filtersHash: "1234567890abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor := EncodeCursor(tt.offset, tt.limit, tt.filtersHash)
			decoded, err := DecodeCursor(cursor)
			if err != nil {
				t.Fatalf("failed to decode cursor: %v", err)
			}

			if decoded.Offset != tt.offset {
				t.Errorf("offset = %d, want %d", decoded.Offset, tt.offset)
			}
			if decoded.Limit != tt.limit {
				t.Errorf("limit = %d, want %d", decoded.Limit, tt.limit)
			}
			if decoded.FiltersHash != tt.filtersHash {
				t.Errorf("filtersHash = %q, want %q", decoded.FiltersHash, tt.filtersHash)
			}
		})
	}
}

func TestCursorValidation(t *testing.T) {
	// Create a cursor with specific filters
	filters := map[string]any{"domain": "light", "area": "living_room"}
	hash := ComputeFiltersHash(filters)
	cursor := EncodeCursor(100, 50, hash)

	// Should succeed with same filters
	args := map[string]any{"cursor": cursor}
	params, err := ParsePaginationParams(args, filters)
	if err != nil {
		t.Errorf("expected success with same filters, got error: %v", err)
	}
	if params.Offset != 100 {
		t.Errorf("offset = %d, want 100", params.Offset)
	}
	if params.Limit != 50 {
		t.Errorf("limit = %d, want 50", params.Limit)
	}

	// Should fail with different filters
	differentFilters := map[string]any{"domain": "switch"}
	_, err = ParsePaginationParams(args, differentFilters)
	if err == nil {
		t.Error("expected error with different filters, got nil")
	}
	if !strings.Contains(err.Error(), "filters have changed") {
		t.Errorf("expected error about changed filters, got: %v", err)
	}
}

func TestComputeFiltersHash(t *testing.T) {
	tests := []struct {
		name     string
		filters1 map[string]any
		filters2 map[string]any
		wantSame bool
	}{
		{
			name:     "same filters same hash",
			filters1: map[string]any{"domain": "light", "area": "living"},
			filters2: map[string]any{"domain": "light", "area": "living"},
			wantSame: true,
		},
		{
			name:     "different order same hash",
			filters1: map[string]any{"area": "living", "domain": "light"},
			filters2: map[string]any{"domain": "light", "area": "living"},
			wantSame: true,
		},
		{
			name:     "different values different hash",
			filters1: map[string]any{"domain": "light"},
			filters2: map[string]any{"domain": "switch"},
			wantSame: false,
		},
		{
			name:     "different keys different hash",
			filters1: map[string]any{"domain": "light"},
			filters2: map[string]any{"area": "light"},
			wantSame: false,
		},
		{
			name:     "nil filters empty hash",
			filters1: nil,
			filters2: nil,
			wantSame: true,
		},
		{
			name:     "empty filters empty hash",
			filters1: map[string]any{},
			filters2: map[string]any{},
			wantSame: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := ComputeFiltersHash(tt.filters1)
			hash2 := ComputeFiltersHash(tt.filters2)

			if tt.wantSame && hash1 != hash2 {
				t.Errorf("expected same hash, got %q and %q", hash1, hash2)
			}
			if !tt.wantSame && hash1 == hash2 {
				t.Errorf("expected different hashes, both got %q", hash1)
			}
		})
	}
}

func TestApplyPagination(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}

	tests := []struct {
		name        string
		params      PaginationParams
		wantCount   int
		wantTotal   int
		wantOffset  int
		wantHasMore bool
		wantItems   []string
	}{
		{
			name:        "no limit returns all",
			params:      PaginationParams{Limit: NoLimit},
			wantCount:   10,
			wantTotal:   10,
			wantOffset:  0,
			wantHasMore: false,
			wantItems:   items,
		},
		{
			name:        "first page",
			params:      PaginationParams{Limit: 3, Offset: 0},
			wantCount:   3,
			wantTotal:   10,
			wantOffset:  0,
			wantHasMore: true,
			wantItems:   []string{"a", "b", "c"},
		},
		{
			name:        "middle page",
			params:      PaginationParams{Limit: 3, Offset: 3},
			wantCount:   3,
			wantTotal:   10,
			wantOffset:  3,
			wantHasMore: true,
			wantItems:   []string{"d", "e", "f"},
		},
		{
			name:        "last page partial",
			params:      PaginationParams{Limit: 3, Offset: 9},
			wantCount:   1,
			wantTotal:   10,
			wantOffset:  9,
			wantHasMore: false,
			wantItems:   []string{"j"},
		},
		{
			name:        "exact last page",
			params:      PaginationParams{Limit: 2, Offset: 8},
			wantCount:   2,
			wantTotal:   10,
			wantOffset:  8,
			wantHasMore: false,
			wantItems:   []string{"i", "j"},
		},
		{
			name:        "offset beyond items",
			params:      PaginationParams{Limit: 3, Offset: 15},
			wantCount:   0,
			wantTotal:   10,
			wantOffset:  10,
			wantHasMore: false,
			wantItems:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyPagination(items, tt.params)

			if result.Pagination.Count != tt.wantCount {
				t.Errorf("count = %d, want %d", result.Pagination.Count, tt.wantCount)
			}
			if result.Pagination.Total != tt.wantTotal {
				t.Errorf("total = %d, want %d", result.Pagination.Total, tt.wantTotal)
			}
			if result.Pagination.Offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", result.Pagination.Offset, tt.wantOffset)
			}
			if result.Pagination.HasMore != tt.wantHasMore {
				t.Errorf("hasMore = %v, want %v", result.Pagination.HasMore, tt.wantHasMore)
			}
			if len(result.Items) != len(tt.wantItems) {
				t.Errorf("items length = %d, want %d", len(result.Items), len(tt.wantItems))
			}
			for i, item := range result.Items {
				if item != tt.wantItems[i] {
					t.Errorf("items[%d] = %q, want %q", i, item, tt.wantItems[i])
				}
			}

			// Verify next cursor when hasMore
			if tt.wantHasMore {
				if result.Pagination.NextCursor == nil {
					t.Error("expected next_cursor when has_more is true")
				}
			} else {
				if result.Pagination.NextCursor != nil {
					t.Error("expected no next_cursor when has_more is false")
				}
			}
		})
	}
}

func TestApplyPaginationEmpty(t *testing.T) {
	var items []string

	result := ApplyPagination(items, PaginationParams{Limit: 10})

	if result.Pagination.Total != 0 {
		t.Errorf("total = %d, want 0", result.Pagination.Total)
	}
	if result.Pagination.Count != 0 {
		t.Errorf("count = %d, want 0", result.Pagination.Count)
	}
	if result.Pagination.HasMore {
		t.Error("expected has_more to be false for empty items")
	}
	if len(result.Items) != 0 {
		t.Error("expected empty items")
	}
}

func TestApplyPaginationSingleItem(t *testing.T) {
	items := []string{"only"}

	result := ApplyPagination(items, PaginationParams{Limit: 10})

	if result.Pagination.Total != 1 {
		t.Errorf("total = %d, want 1", result.Pagination.Total)
	}
	if result.Pagination.Count != 1 {
		t.Errorf("count = %d, want 1", result.Pagination.Count)
	}
	if result.Pagination.HasMore {
		t.Error("expected has_more to be false for single item")
	}
}

func TestBuildPaginationSummary(t *testing.T) {
	tests := []struct {
		name     string
		meta     PaginationMetadata
		itemType string
		want     string
	}{
		{
			name: "no limit",
			meta: PaginationMetadata{
				Total: 100,
				Count: 100,
				Limit: 0,
			},
			itemType: "entities",
			want:     "Found 100 entities",
		},
		{
			name: "with pagination no more",
			meta: PaginationMetadata{
				Total:   50,
				Count:   50,
				Offset:  0,
				Limit:   100,
				HasMore: false,
			},
			itemType: "entities",
			want:     "Showing 50 of 50 entities",
		},
		{
			name: "with pagination has more",
			meta: PaginationMetadata{
				Total:   150,
				Count:   50,
				Offset:  0,
				Limit:   50,
				HasMore: true,
			},
			itemType: "items",
			want:     "Showing 50 of 150 items (offset 0, limit 50, more available)",
		},
		{
			name: "middle page",
			meta: PaginationMetadata{
				Total:   150,
				Count:   50,
				Offset:  50,
				Limit:   50,
				HasMore: true,
			},
			itemType: "results",
			want:     "Showing 50 of 150 results (offset 50, limit 50, more available)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPaginationSummary(tt.meta, tt.itemType)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNextCursorRoundTrip(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	filters := map[string]any{"domain": "light"}

	// First page
	params1, _ := ParsePaginationParams(map[string]any{"limit": float64(3)}, filters)
	result1 := ApplyPagination(items, params1)

	if result1.Pagination.NextCursor == nil {
		t.Fatal("expected next cursor for first page")
	}

	// Second page using cursor
	params2, err := ParsePaginationParams(map[string]any{"cursor": *result1.Pagination.NextCursor}, filters)
	if err != nil {
		t.Fatalf("failed to parse cursor: %v", err)
	}

	result2 := ApplyPagination(items, params2)

	if result2.Pagination.Offset != 3 {
		t.Errorf("second page offset = %d, want 3", result2.Pagination.Offset)
	}
	if len(result2.Items) != 3 {
		t.Errorf("second page items = %d, want 3", len(result2.Items))
	}
	if result2.Items[0] != "d" {
		t.Errorf("second page first item = %q, want 'd'", result2.Items[0])
	}
}
