package mcp

import (
	"context"

	"github.com/zorak1103/ha-mcp/internal/jsonorder"
)

// argumentOrderKey is the unexported context key for the tool arguments' key order.
type argumentOrderKey struct{}

// WithArgumentOrder returns a context carrying the key order of a tool call's
// "arguments" object. Handlers decode arguments into maps, which lose the
// order the caller sent; this shadow tree lets them restore it on write.
func WithArgumentOrder(ctx context.Context, o *jsonorder.Order) context.Context {
	return context.WithValue(ctx, argumentOrderKey{}, o)
}

// ArgumentOrderFromContext returns the key order of the tool call's arguments,
// or nil when none was recorded (no arguments, capture failed, or a plain context).
func ArgumentOrderFromContext(ctx context.Context) *jsonorder.Order {
	o, _ := ctx.Value(argumentOrderKey{}).(*jsonorder.Order)
	return o
}

// captureArgumentOrder records the key order of the "arguments" member of raw
// tools/call params. A capture failure is logged at debug level and yields nil,
// which makes consumers fall back to sorted keys (the pre-existing behavior).
func (s *Server) captureArgumentOrder(raw []byte) *jsonorder.Order {
	if len(raw) == 0 {
		return nil
	}
	o, err := jsonorder.Capture(raw)
	if err != nil {
		s.logger.Debug("Could not capture argument key order", "error", err)
		return nil
	}
	return o.Field("arguments")
}
