package access

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// Denial describes a refused write: either an object lacking a capability or,
// when Object is empty, a system operation that is not granted. A Denial with
// neither is an operation the enforcing layer could not classify.
type Denial struct {
	Tool       string
	Object     string
	Capability string
	System     SystemValue
}

// Message is the text returned to the MCP client. It deliberately says nothing
// about how the policy is configured.
func (d Denial) Message() string {
	var b strings.Builder
	b.WriteString("denied by access policy: ")
	switch {
	case d.Object != "":
		fmt.Fprintf(&b, "`%s` is read-only, `%s` not granted", d.Object, d.Capability)
	case d.System != "":
		fmt.Fprintf(&b, "system operation `%s` is not granted", d.System)
	default:
		b.WriteString("operation not permitted")
	}
	if d.Tool != "" {
		fmt.Fprintf(&b, " (tool: %s)", d.Tool)
	}
	return b.String()
}

// Tracker records the first write denial of one MCP tool call, so the caller
// can replace a handler's result even when the handler swallowed the error. It
// is safe for concurrent use and every method accepts a nil receiver.
type Tracker struct {
	tool string

	mu         sync.Mutex
	first      *Denial
	configured map[string]bool
}

type trackerKey struct{}

// WithTracker returns a context carrying a new Tracker for the given tool.
func WithTracker(ctx context.Context, tool string) (context.Context, *Tracker) {
	t := &Tracker{tool: tool, configured: map[string]bool{}}
	return context.WithValue(ctx, trackerKey{}, t), t
}

// TrackerFrom returns the Tracker stored in ctx, or nil if there is none.
func TrackerFrom(ctx context.Context) *Tracker {
	t, _ := ctx.Value(trackerKey{}).(*Tracker)
	return t
}

// Deny logs d at WARN and keeps it if it is the first denial. A missing d.Tool
// defaults to the tracker's tool.
func (t *Tracker) Deny(d Denial) {
	if t == nil {
		return
	}
	if d.Tool == "" {
		d.Tool = t.tool
	}
	object := d.Object
	if object == "" {
		object = string(d.System)
	}
	slog.Warn("access denied", "tool", d.Tool, "object", object, "capability", d.Capability)

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.first == nil {
		t.first = &d
	}
}

// First returns a copy of the first recorded denial, or nil.
func (t *Tracker) First() *Denial {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.first == nil {
		return nil
	}
	d := *t.first
	return &d
}

// MarkConfigured records that a configure operation on domain succeeded in
// this request.
func (t *Tracker) MarkConfigured(domain string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.configured[strings.ToLower(domain)] = true
}

// ReloadAllowed reports whether a reload of domain follows a successful
// configure operation in this request.
func (t *Tracker) ReloadAllowed(domain string) bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.configured[strings.ToLower(domain)]
}

// Tool returns the name of the tool call this tracker belongs to.
func (t *Tracker) Tool() string {
	if t == nil {
		return ""
	}
	return t.tool
}
