// Design: docs/architecture/diagnostics/debug-filtering.md -- slog handler that filters on flag and scope attributes
// Related: slogutil.go -- ConfigureFilter, ClearFilter, filterRegistry

package slogutil

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"sync"
)

// filterHandler wraps a base slog.Handler and filters log records based on
// flag and scope attributes. When no filters are configured, it delegates
// directly to the base handler (zero-cost path).
//
// Direction filtering uses the scope mechanism: a scope entry with
// kind="direction" and value="receive" filters on the direction attribute.
type filterHandler struct {
	base slog.Handler

	mu     sync.RWMutex
	flags  map[string]bool
	scopes map[string]string // scope-kind -> scope-value

	preAttrs []slog.Attr
}

func newFilterHandler(base slog.Handler) *filterHandler {
	return &filterHandler{base: base}
}

func (h *filterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h *filterHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface requires value receiver
	h.mu.RLock()
	hasFlags := len(h.flags) > 0
	hasScopes := len(h.scopes) > 0
	h.mu.RUnlock()

	if !hasFlags && !hasScopes {
		return h.base.Handle(ctx, r)
	}

	allAttrs := make([]slog.Attr, 0, len(h.preAttrs)+r.NumAttrs())
	allAttrs = append(allAttrs, h.preAttrs...)
	r.Attrs(func(a slog.Attr) bool {
		allAttrs = append(allAttrs, a)
		return true
	})

	if hasFlags && !h.matchFlag(allAttrs) {
		return nil
	}
	if hasScopes && !h.matchScope(allAttrs) {
		return nil
	}

	return h.base.Handle(ctx, r)
}

func (h *filterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, len(h.preAttrs), len(h.preAttrs)+len(attrs))
	copy(combined, h.preAttrs)
	combined = append(combined, attrs...)
	return &filterHandler{
		base:     h.base.WithAttrs(attrs),
		flags:    h.flags,
		scopes:   h.scopes,
		preAttrs: combined,
	}
}

func (h *filterHandler) WithGroup(name string) slog.Handler {
	return &filterHandler{
		base:     h.base.WithGroup(name),
		flags:    h.flags,
		scopes:   h.scopes,
		preAttrs: h.preAttrs,
	}
}

func (h *filterHandler) setFlags(flags []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.flags = make(map[string]bool, len(flags))
	for _, f := range flags {
		h.flags[f] = true
	}
}

func (h *filterHandler) setScopes(scopes map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.scopes = make(map[string]string, len(scopes))
	maps.Copy(h.scopes, scopes)
}

func (h *filterHandler) clearFilters() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.flags = nil
	h.scopes = nil
}

func (h *filterHandler) activeFlags() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.flags) == 0 {
		return nil
	}
	out := make([]string, 0, len(h.flags))
	for f := range h.flags {
		out = append(out, f)
	}
	slices.Sort(out)
	return out
}

func (h *filterHandler) activeScopes() map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.scopes) == 0 {
		return nil
	}
	out := make(map[string]string, len(h.scopes))
	maps.Copy(out, h.scopes)
	return out
}

func (h *filterHandler) matchFlag(attrs []slog.Attr) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, a := range attrs {
		if a.Key == "flag" {
			return h.flags[a.Value.String()]
		}
	}
	return false
}

func (h *filterHandler) matchScope(attrs []slog.Attr) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	matched := 0
	for _, a := range attrs {
		if expected, ok := h.scopes[a.Key]; ok {
			if a.Value.String() != expected {
				return false
			}
			matched++
		}
	}
	return matched == len(h.scopes)
}
