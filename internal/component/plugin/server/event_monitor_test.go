package server

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

// VALIDATES: AC-1 -- no filters means all events, all peers, both directions.
// PREVENTS: Default subscription missing event types.
func TestParseEventMonitorArgsNoFilters(t *testing.T) {
	opts, err := ParseEventMonitorArgs(nil)
	require.NoError(t, err)
	assert.Empty(t, opts.IncludeTypes, "no include filter")
	assert.Empty(t, opts.ExcludeTypes, "no exclude filter")
	assert.Empty(t, opts.Peer, "no peer filter")
	assert.Empty(t, opts.Direction, "no direction filter")
}

// VALIDATES: AC-2 -- include filter parsed correctly.
// PREVENTS: Include keyword not recognized.
func TestParseEventMonitorArgsInclude(t *testing.T) {
	opts, err := ParseEventMonitorArgs([]string{"include", "update,state"})
	require.NoError(t, err)
	assert.Equal(t, []string{"update", "state"}, opts.IncludeTypes)
	assert.Empty(t, opts.ExcludeTypes)
}

// VALIDATES: AC-3 -- exclude filter parsed correctly.
// PREVENTS: Exclude keyword not recognized.
func TestParseEventMonitorArgsExclude(t *testing.T) {
	opts, err := ParseEventMonitorArgs([]string{"exclude", "keepalive"})
	require.NoError(t, err)
	assert.Equal(t, []string{"keepalive"}, opts.ExcludeTypes)
	assert.Empty(t, opts.IncludeTypes)
}

// VALIDATES: AC-4 -- include and exclude are mutually exclusive.
// PREVENTS: Ambiguous filter allowing both include and exclude.
func TestParseEventMonitorIncludeExcludeMutuallyExclusive(t *testing.T) {
	_, err := ParseEventMonitorArgs([]string{"include", "update", "exclude", "state"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// VALIDATES: AC-5 -- combined peer and include filters.
// PREVENTS: Multiple keyword combinations failing to parse.
func TestParseEventMonitorArgsCombined(t *testing.T) {
	opts, err := ParseEventMonitorArgs([]string{"include", "update", "peer", "10.0.0.1", "direction", "received"})
	require.NoError(t, err)
	assert.Equal(t, []string{"update"}, opts.IncludeTypes)
	assert.Equal(t, "10.0.0.1", opts.Peer)
	assert.Equal(t, "received", opts.Direction)
}

// VALIDATES: AC-7 -- invalid event type rejected with helpful error.
// PREVENTS: Unknown event types silently accepted.
func TestParseEventMonitorInvalidType(t *testing.T) {
	_, err := ParseEventMonitorArgs([]string{"include", "invalid-type"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event type")
}

// VALIDATES: Duplicate keywords rejected.
// PREVENTS: Ambiguous args from repeated keywords.
func TestParseEventMonitorDuplicateKeyword(t *testing.T) {
	_, err := ParseEventMonitorArgs([]string{"peer", "10.0.0.1", "peer", "10.0.0.2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate keyword")
}

// VALIDATES: validateEventTypeAnyNamespace accepts types valid in any namespace.
// PREVENTS: Event types from non-BGP namespaces rejected.
func TestValidateEventTypeAnyNamespace(t *testing.T) {
	tests := []struct {
		name    string
		et      string
		wantErr bool
	}{
		{"bgp update", "update", false},
		{"bgp state", "state", false},
		{"bgp rib cache", "cache", false},
		{"bgp rib route", "route", false},
		{"invalid", "nonexistent", true},
		{"sent is direction not event", "sent", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEventTypeAnyNamespace(tt.et)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// VALIDATES: AC-2 -- BuildEventMonitorSubscriptions with include filter subscribes to listed types only.
// PREVENTS: Include filter not limiting event delivery.
func TestBuildEventMonitorSubscriptionsInclude(t *testing.T) {
	opts := &EventMonitorOpts{
		IncludeTypes: []string{"update", "state"},
	}
	subs := BuildEventMonitorSubscriptions(opts)

	var eventTypes []string
	for _, s := range subs {
		eventTypes = append(eventTypes, fmt.Sprintf("%s/%s", s.Namespace, s.EventType))
	}
	assert.Contains(t, eventTypes, "bgp/update")
	assert.Contains(t, eventTypes, "bgp/state")
	assert.NotContains(t, eventTypes, "bgp/keepalive")
}

// VALIDATES: AC-3 -- BuildEventMonitorSubscriptions with exclude filter subscribes to all except excluded.
// PREVENTS: Exclude filter not actually excluding event types.
func TestBuildEventMonitorSubscriptionsExclude(t *testing.T) {
	opts := &EventMonitorOpts{
		ExcludeTypes: []string{"keepalive"},
	}
	subs := BuildEventMonitorSubscriptions(opts)

	var eventTypes []string
	for _, s := range subs {
		eventTypes = append(eventTypes, s.EventType.String())
	}
	assert.NotContains(t, eventTypes, "keepalive")
	assert.Contains(t, eventTypes, "update")
	assert.Contains(t, eventTypes, "state")
}

// VALIDATES: AC-6 -- direction filter applied to all subscriptions.
// PREVENTS: Direction filter ignored during subscription building.
func TestBuildEventMonitorSubscriptionsDirection(t *testing.T) {
	opts := &EventMonitorOpts{
		Direction: events.DirectionReceived,
	}
	subs := BuildEventMonitorSubscriptions(opts)
	require.NotEmpty(t, subs)
	for _, s := range subs {
		assert.Equal(t, events.DirReceived, s.Direction,
			"all subscriptions should have direction=received, got %s for %s/%s", s.Direction, s.Namespace, s.EventType)
	}
}

// VALIDATES: Keywords may appear in any order.
// PREVENTS: Parser requiring fixed keyword order.
func TestParseEventMonitorArgsAnyOrder(t *testing.T) {
	opts, err := ParseEventMonitorArgs([]string{"direction", "sent", "peer", "10.0.0.1", "include", "update"})
	require.NoError(t, err)
	assert.Equal(t, "sent", opts.Direction)
	assert.Equal(t, "10.0.0.1", opts.Peer)
	assert.Equal(t, []string{"update"}, opts.IncludeTypes)
}

// VALIDATES: Default subscriptions include all event types dynamically.
// PREVENTS: Hardcoded event type list missing newer types (congested, resumed, rpki).
func TestBuildEventMonitorSubscriptionsDefault(t *testing.T) {
	opts := &EventMonitorOpts{}
	subs := BuildEventMonitorSubscriptions(opts)

	var types []string
	for _, s := range subs {
		types = append(types, s.EventType.String())
	}
	// Must include types that the old hardcoded allBGPEventTypes missed.
	assert.Contains(t, types, "congested", "should include congested (was missing from old list)")
	assert.Contains(t, types, "resumed", "should include resumed (was missing from old list)")
	assert.Contains(t, types, "rpki", "should include rpki (was missing from old list)")
	// Must include RIB types too.
	assert.Contains(t, types, "cache", "should include rib cache")
	assert.Contains(t, types, "route", "should include rib route")
	// Must NOT include "sent" (it's a direction flag in the BGP namespace, not an event type).
	assert.NotContains(t, types, "sent", "sent is a direction, not an event type")
}

// VALIDATES: Unknown keyword rejected.
// PREVENTS: Typos silently ignored.
func TestParseEventMonitorUnknownKeyword(t *testing.T) {
	_, err := ParseEventMonitorArgs([]string{"filter", "update"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown keyword")
}

// VALIDATES: Peer exclusion syntax (!) accepted.
// PREVENTS: Negated peer selectors rejected by parser.
func TestParseEventMonitorPeerExclusion(t *testing.T) {
	opts, err := ParseEventMonitorArgs([]string{"peer", "!10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "!10.0.0.1", opts.Peer)
}

// VALIDATES: Missing value after keyword returns error.
// PREVENTS: Index-out-of-range panic when keyword is last arg.
func TestParseEventMonitorMissingValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"include missing", []string{"include"}, "requires"},
		{"exclude missing", []string{"exclude"}, "requires"},
		{"peer missing", []string{"peer"}, "requires"},
		{"direction missing", []string{"direction"}, "requires"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEventMonitorArgs(tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// VALIDATES: Invalid direction value rejected.
// PREVENTS: Bogus direction silently accepted.
func TestParseEventMonitorInvalidDirection(t *testing.T) {
	_, err := ParseEventMonitorArgs([]string{"direction", "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid direction")
}

// VALIDATES: Duplicate include/exclude/direction keywords rejected.
// PREVENTS: Ambiguous args from repeated keywords.
func TestParseEventMonitorDuplicateAllKeywords(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"duplicate include", []string{"include", "update", "include", "state"}},
		{"duplicate exclude", []string{"exclude", "update", "exclude", "state"}},
		{"duplicate direction", []string{"direction", "sent", "direction", "received"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEventMonitorArgs(tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "duplicate keyword")
		})
	}
}

// VALIDATES: Trailing comma in event type list rejected.
// PREVENTS: Empty string silently passing validation.
func TestParseEventMonitorTrailingComma(t *testing.T) {
	_, err := ParseEventMonitorArgs([]string{"include", "update,"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty event type")
}

// VALIDATES: Invalid peer selectors rejected.
// PREVENTS: Empty or malformed peer selectors creating broken filters.
func TestParseEventMonitorInvalidPeer(t *testing.T) {
	tests := []struct {
		name string
		peer string
	}{
		{"bare exclamation", "!"},
		{"double exclamation", "!!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEventMonitorArgs([]string{"peer", tt.peer})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid peer selector")
		})
	}
}

// VALIDATES: Peer filter set on subscriptions when peer is specified.
// PREVENTS: Peer filter silently dropped during subscription building.
func TestBuildEventMonitorSubscriptionsPeerFilter(t *testing.T) {
	opts := &EventMonitorOpts{Peer: "10.0.0.1"}
	subs := BuildEventMonitorSubscriptions(opts)
	require.NotEmpty(t, subs)
	for _, s := range subs {
		require.NotNil(t, s.PeerFilter, "all subs should have PeerFilter")
		assert.Equal(t, "10.0.0.1", s.PeerFilter.Selector)
	}
}

// VALIDATES: No peer filter when peer is empty.
// PREVENTS: Non-nil PeerFilter with empty selector causing match failures.
func TestBuildEventMonitorSubscriptionsNoPeerFilter(t *testing.T) {
	opts := &EventMonitorOpts{}
	subs := BuildEventMonitorSubscriptions(opts)
	require.NotEmpty(t, subs)
	for _, s := range subs {
		assert.Nil(t, s.PeerFilter, "all subs should have nil PeerFilter when no peer specified")
	}
}

// VALIDATES: Default direction is "both" when not specified.
// PREVENTS: Missing direction causing empty or wrong filter.
func TestBuildEventMonitorSubscriptionsDefaultDirection(t *testing.T) {
	opts := &EventMonitorOpts{}
	subs := BuildEventMonitorSubscriptions(opts)
	require.NotEmpty(t, subs)
	for _, s := range subs {
		assert.Equal(t, events.DirBoth, s.Direction)
	}
}

// VALIDATES: Combined header with multiple filters.
// PREVENTS: Header missing parts when multiple filters active.
func TestFormatEventMonitorHeaderCombined(t *testing.T) {
	opts := &EventMonitorOpts{
		IncludeTypes: []string{"update"},
		Peer:         "10.0.0.1",
		Direction:    "received",
	}
	got := formatEventMonitorHeader(opts)
	assert.Contains(t, got, "include=update")
	assert.Contains(t, got, "peer=10.0.0.1")
	assert.Contains(t, got, "direction=received")
}

// VALIDATES: Case-sensitive peer names preserved through arg extraction.
// PREVENTS: Peer names lowercased by GetStreamingHandlerForCommand.
func TestGetStreamingHandlerPreservesArgCase(t *testing.T) {
	streamingHandlersMu.Lock()
	saved := streamingHandlers
	streamingHandlers = make(map[string]StreamingHandler)
	streamingHandlersMu.Unlock()
	defer func() {
		streamingHandlersMu.Lock()
		streamingHandlers = saved
		streamingHandlersMu.Unlock()
	}()

	handler := func(_ context.Context, _ *Server, _ io.Writer, _ string, _ []string) error { return nil }
	RegisterStreamingHandler("monitor event", handler)

	_, args := GetStreamingHandlerForCommand("monitor event peer MyRouter-1")
	require.Equal(t, []string{"peer", "MyRouter-1"}, args, "args should preserve original case")
}

// VALIDATES: Event monitor header line describes active filters.
// PREVENTS: User sees no indication of what's being filtered.
func TestFormatEventMonitorHeader(t *testing.T) {
	tests := []struct {
		name string
		opts *EventMonitorOpts
		want string
	}{
		{"no filters", &EventMonitorOpts{}, "monitoring: all events, all peers"},
		{"peer only", &EventMonitorOpts{Peer: "10.0.0.1"}, "peer=10.0.0.1"},
		{"include", &EventMonitorOpts{IncludeTypes: []string{"update", "state"}}, "include=update,state"},
		{"exclude", &EventMonitorOpts{ExcludeTypes: []string{"keepalive"}}, "exclude=keepalive"},
		{"direction", &EventMonitorOpts{Direction: "received"}, "direction=received"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatEventMonitorHeader(tt.opts)
			assert.True(t, strings.Contains(got, tt.want),
				"header %q should contain %q", got, tt.want)
		})
	}
}
