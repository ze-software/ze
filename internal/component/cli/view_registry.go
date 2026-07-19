// Design: ai/rules/plugin-self-containment.md -- "Registration over hardcoding (the CLI client too)"
// Pattern: ai/patterns/registration.md -- init() + registry + longest-prefix match
// Mirror:  internal/component/plugin/server/handler.go -- RegisterMonitorProvider / matchesPrefix
//
// The client-side live-view registry. Each rich live view (dashboard, ping,
// traceroute) registers a viewSpec from its own model_*.go init(); the Model
// discovers views through this registry instead of carrying a per-feature field
// and switch arm for each. A single Model.activeView handle plus a generic
// keyed factory store (Model.viewFactories) replace the per-view fields.

package cli

import (
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// activeView is the lifecycle interface a live view exposes to the Model while
// it is the single active full-screen view. The concrete instances live in the
// view's own file (model_ping.go / model_traceroute.go / model_dashboard.go),
// so render/update/state stay in the cli package (Design 1) and cli imports no
// engine package.
type activeView interface {
	// update processes a routed viewMsg tick/data message. It mutates *m and
	// returns the updated model and next command (bubbletea Update semantics).
	update(m *Model, msg tea.Msg) (tea.Model, tea.Cmd)
	// render returns the full-screen content for this view, or "" to fall
	// through to the normal viewport render (used by the piped | log variants,
	// which append to the scrollback rather than taking the alt screen).
	render(m *Model) string
	// key handles a key press. It returns true when the key is absorbed by the
	// view (Model returns without further handling), false to fall through.
	key(m *Model, keyStr string) bool
	// release cancels the view's live resources (its context / probe round) when
	// the view is being REPLACED by another view. It performs no scrollback or
	// viewport teardown (that is the concrete stop* path, driven by Esc/q): a
	// view-switch must cancel the outgoing poller so it does not leak (spec
	// Security Review), but a failed start never installs a new view, so the old
	// view is left running untouched.
	release()
}

// viewMsg marks a bubbletea message that must be routed to the active view.
// Collapsing the per-feature Update cases into one `case viewMsg` arm is the
// point of the registry: a new view's tick messages route without a Model edit.
type viewMsg interface {
	isViewMsg()
}

// viewSpec is a live view's registration descriptor. One spec per view (3
// total); the plain vs piped ("| ...") split is handled inside start, not by a
// second spec.
type viewSpec struct {
	// key indexes the view's factory in Model.viewFactories and is the stable
	// identifier consumers use to inject the concrete factory.
	key string
	// prefix is the command prefix used for longest-prefix resolution (mirrors
	// handler.go matchesPrefix word-boundary semantics).
	prefix string
	// matches reports whether input selects this view. Defaults to a
	// prefixMatch on prefix; ping/traceroute supply pipe/target-aware matchers.
	matches func(input string) bool
	// start builds the active view from input, storing it on Model.activeView,
	// and returns the initial tea.Cmd (or nil after setting a status message).
	start func(m *Model, input string) tea.Cmd
}

// ViewInfo is the exported, read-only view of a registered view. Consumers
// (hub session factory, cli client) iterate RegisteredViews() and inject each
// view's concrete factory by Key instead of calling per-view typed setters.
type ViewInfo struct {
	Key    string
	Prefix string
}

var (
	viewRegistryMu sync.RWMutex
	viewRegistry   []viewSpec
)

// RegisterView registers a live view. Called from a view's own file init().
// The spec is unexported, so only in-package views (Design 1) can register.
func RegisterView(spec viewSpec) {
	if spec.matches == nil {
		prefix := spec.prefix
		spec.matches = func(input string) bool {
			return prefixMatch(strings.TrimSpace(input), prefix)
		}
	}
	viewRegistryMu.Lock()
	viewRegistry = append(viewRegistry, spec)
	viewRegistryMu.Unlock()
}

// unregisterView removes a registered view by key. Used by tests to keep the
// global registry clean after registering a synthetic view.
func unregisterView(key string) {
	viewRegistryMu.Lock()
	defer viewRegistryMu.Unlock()
	out := viewRegistry[:0]
	for _, spec := range viewRegistry {
		if spec.key != key {
			out = append(out, spec)
		}
	}
	viewRegistry = out
}

// RegisteredViews returns the registered views as exported descriptors, sorted
// by nothing in particular (consumers switch on Key). Iterating this is how a
// consumer discovers which factories to inject without a per-view setter chain.
func RegisteredViews() []ViewInfo {
	viewRegistryMu.RLock()
	defer viewRegistryMu.RUnlock()
	out := make([]ViewInfo, 0, len(viewRegistry))
	for _, spec := range viewRegistry {
		out = append(out, ViewInfo{Key: spec.key, Prefix: spec.prefix})
	}
	return out
}

// resolveView resolves input to a registered view by longest-prefix match.
func resolveView(input string) (viewSpec, bool) {
	viewRegistryMu.RLock()
	defer viewRegistryMu.RUnlock()
	return matchViewSpecs(viewRegistry, input)
}

// matchViewSpecs selects, among specs whose matches(input) is true, the one
// with the longest prefix. This mirrors the daemon-side monitor-provider
// resolution (handler.go GetMonitorProvider): sibling "monitor *" prefixes
// resolve unambiguously to the most specific match. Pure and lock-free so it
// is unit-testable with a synthetic spec set.
func matchViewSpecs(specs []viewSpec, input string) (viewSpec, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return viewSpec{}, false
	}
	var best viewSpec
	bestLen := -1
	found := false
	for _, spec := range specs {
		if spec.matches != nil && spec.matches(trimmed) && len(spec.prefix) > bestLen {
			best = spec
			bestLen = len(spec.prefix)
			found = true
		}
	}
	return best, found
}

// prefixMatch reports whether input begins with prefix on a word boundary.
// Copied from internal/component/plugin/server/handler.go matchesPrefix so the
// client registry and the daemon monitor-provider registry resolve prefixes
// identically: "monitor bgpx" must NOT match "monitor bgp".
func prefixMatch(input, prefix string) bool {
	return input == prefix ||
		(strings.HasPrefix(input, prefix) && len(input) > len(prefix) && input[len(prefix)] == ' ')
}

// SetViewFactory stores a view's concrete factory under key. The value is kept
// as any and asserted to the view's factory type once at start (fail-closed:
// a wrong type yields a clear status message, never a nil-driven no-op).
func (m *Model) SetViewFactory(key string, factory any) {
	if m.viewFactories == nil {
		m.viewFactories = make(map[string]any)
	}
	m.viewFactories[key] = factory
}

// viewFactoryRaw returns the raw factory stored under key and whether one is
// present. Callers assert it to the view's concrete factory type.
func (m *Model) viewFactoryRaw(key string) (any, bool) {
	if m.viewFactories == nil {
		return nil, false
	}
	f, ok := m.viewFactories[key]
	return f, ok
}
