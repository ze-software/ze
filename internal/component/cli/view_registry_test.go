package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestViewRegistryResolvesPing verifies a monitor-ping command resolves to the
// registered ping view through the registry, not a Model isXCommand chain (AC-1).
//
// VALIDATES: resolveView maps "monitor ping <t>" (plain and piped) to the ping
// viewSpec, and leaves a bare "monitor ping" (no target) unresolved.
// PREVENTS: dispatch regressing to a hardcoded per-feature chain, or the bare
// no-target form wrongly claiming the view instead of falling through.
func TestViewRegistryResolvesPing(t *testing.T) {
	spec, ok := resolveView("monitor ping 127.0.0.1")
	require.True(t, ok, "monitor ping must resolve to a registered view")
	assert.Equal(t, ViewKeyPing, spec.key)

	// Piped ping resolves to the same view (one spec covers plain and piped).
	spec, ok = resolveView("monitor ping 127.0.0.1 | json")
	require.True(t, ok)
	assert.Equal(t, ViewKeyPing, spec.key)

	// A bare "monitor ping" with no target must NOT resolve (preserves the old
	// isPingMonitorCommand target-required semantics: it falls through instead).
	_, ok = resolveView("monitor ping")
	assert.False(t, ok, "bare monitor ping (no target) must not resolve")
}

// TestViewRegistryLongestPrefixMatch verifies sibling "monitor *" prefixes
// resolve to the most specific match and honor the handler.go word boundary:
// "monitor bgpx" must NOT match "monitor bgp" (AC-1 / A-4).
//
// VALIDATES: longest-prefix selection among matching specs, plus the word-
// boundary rule copied from handler.go matchesPrefix, plus empty-input no-match.
// PREVENTS: a sibling "monitor *" command resolving to the wrong (shorter) view,
// or "monitor bgpx" being mis-resolved to the "monitor bgp" dashboard.
func TestViewRegistryLongestPrefixMatch(t *testing.T) {
	mk := func(key, prefix string) viewSpec {
		p := prefix
		return viewSpec{key: key, prefix: p, matches: func(in string) bool {
			return prefixMatch(strings.TrimSpace(in), p)
		}}
	}
	specs := []viewSpec{mk("generic", "monitor"), mk("bgp", "monitor bgp")}

	got, ok := matchViewSpecs(specs, "monitor bgp")
	require.True(t, ok)
	assert.Equal(t, "bgp", got.key, "exact longest prefix wins")

	got, ok = matchViewSpecs(specs, "monitor bgp summary")
	require.True(t, ok)
	assert.Equal(t, "bgp", got.key, "longest prefix wins over shorter sibling")

	got, ok = matchViewSpecs(specs, "monitor bgpx")
	require.True(t, ok)
	assert.Equal(t, "generic", got.key, "word boundary: bgpx must not match bgp")

	got, ok = matchViewSpecs(specs, "monitor event")
	require.True(t, ok)
	assert.Equal(t, "generic", got.key)

	// Boundary: empty input matches nothing.
	_, ok = matchViewSpecs(specs, "")
	assert.False(t, ok, "empty input must not match")

	// Non-monitor input matches nothing.
	_, ok = matchViewSpecs(specs, "show bgp")
	assert.False(t, ok)
}

// TestViewRegistryDiscoversRegisteredView verifies a newly registered view is
// reachable through the registry and its factory store without editing Model
// (AC-2). The synthetic view is unregistered after the test.
//
// VALIDATES: a view registered via RegisterView is exposed by RegisteredViews,
// resolvable by resolveView, and startable with its factory read from the keyed
// store -- all without editing Model.
// PREVENTS: a new view requiring a Model/consumer edit to become reachable.
func TestViewRegistryDiscoversRegisteredView(t *testing.T) {
	const key = "unit-test-view"
	started := false
	var gotFactory any
	RegisterView(viewSpec{
		key:    key,
		prefix: "unittestview",
		start: func(m *Model, _ string) tea.Cmd {
			started = true
			gotFactory, _ = m.viewFactoryRaw(key)
			return nil
		},
	})
	defer unregisterView(key)

	// Discoverable through the exported iteration used by consumers.
	found := false
	for _, v := range RegisteredViews() {
		if v.Key == key {
			found = true
			assert.Equal(t, "unittestview", v.Prefix)
		}
	}
	assert.True(t, found, "RegisteredViews must expose the new view")

	// Resolvable and startable with no Model edit; factory injected generically.
	spec, ok := resolveView("unittestview now")
	require.True(t, ok)
	assert.Equal(t, key, spec.key)

	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetViewFactory(key, "sentinel-factory")
	spec.start(&m, "unittestview now")
	assert.True(t, started, "registered view start must be reachable via the registry")
	assert.Equal(t, "sentinel-factory", gotFactory, "injected factory reaches the view via the keyed store")
}

// runOperationalCommand drives one operational command through the real dispatch
// (Model.handleEnter), updating *m in place. It exercises a view-switch end to
// end, the same path the SSH/web CLIs use. The Model is taken by pointer (it is
// heavy) even though handleEnter is a value receiver.
func runOperationalCommand(t *testing.T, m *Model, input string) {
	t.Helper()
	m.textInput.SetValue(input)
	updated, _ := m.handleEnter()
	out, ok := updated.(Model)
	require.True(t, ok, "handleEnter must return a cli.Model")
	*m = out
}

// pingFactoryRecordingCancels returns a PingFactory whose every call appends a
// live *bool to *canceled; that bool flips true when the session's CancelFunc
// runs. It lets a test observe whether switching views releases the outgoing
// ping poller's context. The channel is left open (never closed) so the session
// stays "live" until explicitly canceled.
func pingFactoryRecordingCancels(canceled *[]*bool) PingFactory {
	return func(_ context.Context, _ string, _, _ time.Duration, _, _ int) (<-chan map[string]any, context.CancelFunc, error) {
		flag := new(bool)
		*canceled = append(*canceled, flag)
		ch := make(chan map[string]any)
		return ch, func() { *flag = true }, nil
	}
}

// TestDispatchSwitchReleasesPreviousView proves the spec's Security Review
// requirement: switching from a live view to another cancels the outgoing view's
// context so its poller does not leak. This is the regression guard for the
// review-found view-switch leak (release() wired into handleEnter).
//
// VALIDATES: starting a second view while one is active cancels the first view's
// context (via the activeView.release path in Model.handleEnter).
// PREVENTS: a re-introduction of the orphaned-context/goroutine leak on switch.
func TestDispatchSwitchReleasesPreviousView(t *testing.T) {
	var canceled []*bool
	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetPingFactory(pingFactoryRecordingCancels(&canceled))

	runOperationalCommand(t, &m, "monitor ping 192.0.2.1")
	require.Len(t, canceled, 1, "first ping session must have started")
	require.NotNil(t, m.activeView, "a ping view must be active")
	assert.False(t, *canceled[0], "the first ping context must still be live")

	// Switch to a second ping view; the first session must be released.
	runOperationalCommand(t, &m, "monitor ping 192.0.2.2")
	require.Len(t, canceled, 2, "the second ping session must have started")
	assert.True(t, *canceled[0], "switching views must cancel the previous view's context (no leak)")
	assert.False(t, *canceled[1], "the newly active view must still be live")
}

// TestDispatchFailedStartPreservesActiveView proves the error-path guard: a
// command that resolves to a view but fails to start (invalid args) must NOT tear
// down the currently-active view. release() runs only when start installs a new
// view, so a failed switch leaves the old view running.
//
// VALIDATES: a rejected `monitor ping ... interval 5ms` leaves the prior ping
// view active and un-released, with the arg error surfaced.
// PREVENTS: a typo in a switch command silently killing a running monitor.
func TestDispatchFailedStartPreservesActiveView(t *testing.T) {
	var canceled []*bool
	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetPingFactory(pingFactoryRecordingCancels(&canceled))

	runOperationalCommand(t, &m, "monitor ping 192.0.2.1")
	require.NotNil(t, m.activeView, "a ping view must be active")
	require.Len(t, canceled, 1)

	// Invalid interval: parse fails before the factory is called, so no new view
	// is installed and the active one must survive untouched.
	runOperationalCommand(t, &m, "monitor ping 192.0.2.2 interval 5ms")
	assert.Len(t, canceled, 1, "a failed start must not open a new session")
	assert.False(t, *canceled[0], "a failed start must not release the still-active view")
	assert.NotNil(t, m.activeView, "the previous view must remain active after a failed switch")
	assert.Contains(t, m.statusMessage, "interval must be", "the arg error must be surfaced")
}
