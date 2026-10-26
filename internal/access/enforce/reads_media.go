package enforce

import (
	"context"
	"net/url"
	"strings"
	"unicode"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

const (
	signCameraPrefix  = "/api/camera_proxy/"
	signMediaPrefix   = "/media/"
	cameraMediaPrefix = "media-source://camera/"
)

// Cameras.

func (c *client) GetCameraSnapshot(ctx context.Context, entityID string) ([]byte, string, error) {
	hidden, err := c.entityHidden(ctx, entityID)
	if err != nil {
		return nil, "", c.undecidable("GetCameraSnapshot", err)
	}
	if hidden {
		return nil, "", errCameraSnapshotMissing()
	}
	return c.inner.GetCameraSnapshot(ctx, entityID)
}

func (c *client) GetCameraStream(ctx context.Context, entityID string) (*homeassistant.StreamInfo, error) {
	return guardEntity(ctx, c, "GetCameraStream", entityID, errCameraStreamMissing(), func() (*homeassistant.StreamInfo, error) {
		return c.inner.GetCameraStream(ctx, entityID)
	})
}

// Templates.

// RenderTemplate can read any state, so once the policy can hide an object a
// template cannot be told apart from a leak: it is refused as a whole. Without
// hiding rules it passes through.
func (c *client) RenderTemplate(ctx context.Context, template string) (string, error) {
	if c.p.HidesAnything() {
		return "", c.refuse("render_template")
	}
	return c.inner.RenderTemplate(ctx, template)
}

// Signed paths.

// signKind classifies a path handed to SignPath.
type signKind int

const (
	signOther  signKind = iota // anything the policy cannot vouch for
	signMedia                  // /media/...
	signCamera                 // /api/camera_proxy/<entity id>
)

// SignPath signs paths under /media/ and camera snapshots. Once the policy can
// hide an object, a camera path is checked like a state lookup of that camera
// (so a hidden camera and a missing one fail with the same error) and every
// other path is refused: a signed URL would otherwise be a way around the
// filters. Without hiding rules it passes through.
func (c *client) SignPath(ctx context.Context, path string, expires int) (string, error) {
	if !c.p.HidesAnything() {
		return c.inner.SignPath(ctx, path, expires)
	}
	kind, cameraID := classifySignPath(path)
	switch kind {
	case signMedia:
		return c.inner.SignPath(ctx, path, expires)
	case signCamera:
		if _, err := c.GetState(ctx, cameraID); err != nil {
			return "", err
		}
		return c.inner.SignPath(ctx, path, expires)
	default:
		return "", c.refuse("sign_path")
	}
}

// classifySignPath recognizes the two signable shapes strictly: no control
// characters, backslashes or fragments, no dot segments (also percent-encoded)
// and exactly one segment after the camera prefix. For a camera path it
// returns the decoded entity id.
func classifySignPath(p string) (signKind, string) {
	if strings.ContainsAny(p, "\\#") || strings.ContainsFunc(p, unicode.IsControl) {
		return signOther, ""
	}
	switch {
	case strings.HasPrefix(p, signMediaPrefix):
		if u, err := url.Parse(p); err == nil && !hasDotSegment(u.Path) {
			return signMedia, ""
		}
	case strings.HasPrefix(p, signCameraPrefix):
		rest, _, _ := strings.Cut(strings.TrimPrefix(p, signCameraPrefix), "?")
		id, err := url.PathUnescape(rest)
		if err == nil && id != "" && !strings.Contains(id, "/") && id != "." && id != ".." {
			return signCamera, id
		}
	}
	return signOther, ""
}

func hasDotSegment(p string) bool {
	for seg := range strings.SplitSeq(p, "/") {
		if seg == "." || seg == ".." {
			return true
		}
	}
	return false
}

// Media browsing.

// BrowseMedia drops camera items of hidden cameras (media-source://camera/<id>)
// from the returned tree, at any depth. Browsing a hidden camera item directly
// answers like an unknown item. The shared tree is copied, never modified.
func (c *client) BrowseMedia(ctx context.Context, mediaContentID string) (*homeassistant.MediaBrowseResult, error) {
	if !c.p.HidesAnything() {
		return c.inner.BrowseMedia(ctx, mediaContentID)
	}
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("BrowseMedia", err)
	}
	if hiddenCameraItem(mediaContentID, hidden) {
		return nil, errMediaItemMissing()
	}
	tree, err := c.inner.BrowseMedia(ctx, mediaContentID)
	if err != nil || tree == nil {
		return tree, err
	}
	return filterMediaTree(tree, func(id string) bool { return hiddenCameraItem(id, hidden) }), nil
}

// hiddenCameraItem reports whether a media content id addresses a camera that
// is hidden (or is not a valid entity id, which is treated as hidden). The
// camera folder itself (no id after the prefix) and other sources are kept.
func hiddenCameraItem(mediaContentID string, hidden func(string) bool) bool {
	rest, ok := strings.CutPrefix(strings.ToLower(strings.TrimSpace(mediaContentID)), cameraMediaPrefix)
	if !ok {
		return false
	}
	rest, _, _ = strings.Cut(rest, "?")
	return rest != "" && hidden(rest)
}

// filterMediaTree returns a copy of n without the descendants for which drop is
// true.
func filterMediaTree(n *homeassistant.MediaBrowseResult, drop func(contentID string) bool) *homeassistant.MediaBrowseResult {
	cp := *n
	cp.Children = nil
	for _, child := range n.Children {
		if child == nil || drop(child.MediaContentID) {
			continue
		}
		cp.Children = append(cp.Children, filterMediaTree(child, drop))
	}
	return &cp
}
