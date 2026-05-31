// internal/handlers/skills/catalog_test.go
package skills_test

import (
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/handlers/skills"
)

func TestCatalog_Length(t *testing.T) {
	t.Parallel()
	const want = 7
	if len(skills.Catalog) != want {
		t.Errorf("Catalog has %d entries, want %d", len(skills.Catalog), want)
	}
}

func TestCatalog_Slugs_UniqueAndKebab(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, m := range skills.Catalog {
		if m.Slug == "" {
			t.Error("found empty slug in Catalog")
		}
		if m.Slug != strings.ToLower(m.Slug) {
			t.Errorf("slug %q is not lowercase", m.Slug)
		}
		if strings.Contains(m.Slug, " ") || strings.Contains(m.Slug, "_") {
			t.Errorf("slug %q must be kebab-case (no spaces or underscores)", m.Slug)
		}
		if seen[m.Slug] {
			t.Errorf("duplicate slug %q in Catalog", m.Slug)
		}
		seen[m.Slug] = true
	}
}

func TestCatalog_AllHaveNameAndDescription(t *testing.T) {
	t.Parallel()
	for _, m := range skills.Catalog {
		if m.Name == "" {
			t.Errorf("skill %q has empty Name", m.Slug)
		}
		if m.Description == "" {
			t.Errorf("skill %q has empty Description", m.Slug)
		}
	}
}

func TestCatalog_URIs(t *testing.T) {
	t.Parallel()
	for _, m := range skills.Catalog {
		uri := m.URI()
		wantURI := skills.URIPrefix + m.Slug
		if uri != wantURI {
			t.Errorf("skill %q URI = %q, want %q", m.Slug, uri, wantURI)
		}
		if !strings.HasPrefix(uri, "skill://ha-mcp/") {
			t.Errorf("skill %q URI %q does not start with skill://ha-mcp/", m.Slug, uri)
		}
	}
}

func TestRead_ValidSlugs(t *testing.T) {
	t.Parallel()
	for _, m := range skills.Catalog {
		content, err := skills.Read(m.Slug)
		if err != nil {
			t.Errorf("Read(%q) error = %v", m.Slug, err)
			continue
		}
		if content == "" {
			t.Errorf("Read(%q) returned empty content", m.Slug)
		}
		// Every skill must have at least a heading
		if !strings.Contains(content, "#") {
			t.Errorf("Read(%q) content missing markdown heading", m.Slug)
		}
	}
}

func TestRead_InvalidSlug(t *testing.T) {
	t.Parallel()
	_, err := skills.Read("bogus-nonexistent-slug-xyz")
	if err == nil {
		t.Error("Read(bogus) should return an error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "bogus-nonexistent-slug-xyz") {
		t.Errorf("error should mention the slug, got: %v", err)
	}
}
