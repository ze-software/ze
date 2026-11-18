// Design: docs/guide/l2tp.md -- original-session selector and audit arguments

package cmd

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestSessionTeardownArgumentsKeepGuardSeparateFromAudit checks a complete
// operator request, retaining quoted reason keywords and the following cause.
func TestSessionTeardownArgumentsKeepGuardSeparateFromAudit(t *testing.T) {
	const reference = "l2tp-00000000000000000000000000000001"
	request, refusal := parseSessionTeardownRequest([]string{
		"42", "expected-id", reference, "tunnel-id", "7", "listener", "edge",
		"actor", "alice", "reason", "maintenance cause 999", "cause", "6",
	})
	require.Nil(t, refusal)
	birth, err := subscriberid.ParseL2TPBirth(reference)
	require.NoError(t, err)
	require.Equal(t, l2tp.SessionSelector{SID: 42, TID: 7, Birth: birth}, request.selector)
	require.Equal(t, "edge", request.listener)
	require.Equal(t, "alice", request.actor)
	require.Equal(t, "maintenance cause 999", request.reason)
	require.Equal(t, uint32(6), request.cause)
}

// TestSessionTeardownArgumentsPreserveFreeReasonWords retains the established
// unquoted multiword reason form and its following keyword-delimited cause.
func TestSessionTeardownArgumentsPreserveFreeReasonWords(t *testing.T) {
	request, refusal := parseSessionTeardownRequest([]string{"42", "reason", "maintenance", "window", "cause", "6"})
	require.Nil(t, refusal)
	require.Equal(t, "maintenance window", request.reason)
	require.Equal(t, uint32(6), request.cause)
	require.True(t, request.selector.Birth.IsZero(), "interactive use does not require a generated guard")
}

// TestSessionTeardownArgumentsRejectMalformedValues requires refusal before a
// selector can be handed to the subsystem; no valid zero may replace a bad ID.
func TestSessionTeardownArgumentsRejectMalformedValues(t *testing.T) {
	for _, args := range [][]string{
		nil, {"0"}, {"65536"}, {"-1"},
		{"42", "expected-id"}, {"42", "expected-id", "invalid"},
		{"42", "expected-id", "l2tp-00000000000000000000000000000000"},
		{"42", "expected-id", "L2TP-00000000000000000000000000000001"},
		{"42", "expected-id", "l2tp-00000000000000000000000000000001", "expected-id", "l2tp-00000000000000000000000000000002"},
		{"42", "tunnel-id"}, {"42", "tunnel-id", "0"}, {"42", "tunnel-id", "65536"},
		{"42", "listener"}, {"42", "listener", ""}, {"42", "listener", "-help"},
		{"42", "actor"}, {"42", "actor", ""}, {"42", "actor", "alice", "actor", "bob"},
		{"42", "reason"}, {"42", "reason", ""}, {"42", "reason", strings.Repeat("x", 257)},
		{"42", "cause"}, {"42", "cause", "invalid"}, {"42", "cause", "4294967296"},
		{"42", "unknown", "value"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, refusal := parseSessionTeardownRequest(args)
			require.NotNil(t, refusal)
		})
	}
}

// TestSessionTeardownListenerNamesRequireOneBoundEndpoint checks the original
// inventory binding, retaining unknown, ambiguous and explicit-endpoint cases.
func TestSessionTeardownListenerNamesRequireOneBoundEndpoint(t *testing.T) {
	bound := netip.MustParseAddrPort("127.0.0.1:1701")
	service := &fakeService{listeners: []l2tp.ListenerSnapshot{{Name: "edge", Addr: bound}}}
	request, refusal := parseSessionTeardownRequest([]string{"42", "listener", "edge"})
	require.Nil(t, refusal)
	selector, refusal := request.resolveSelector(service)
	require.Nil(t, refusal)
	require.Equal(t, bound, selector.Listener)
	service.listeners = append(service.listeners, l2tp.ListenerSnapshot{Name: "edge", Addr: netip.MustParseAddrPort("127.0.0.1:1702")})
	_, refusal = request.resolveSelector(service)
	require.NotNil(t, refusal)
	for _, name := range []string{"unknown", "127.0.0.1:0"} {
		request, refusal = parseSessionTeardownRequest([]string{"42", "listener", name})
		require.Nil(t, refusal)
		_, refusal = request.resolveSelector(service)
		require.NotNil(t, refusal)
	}
	request, refusal = parseSessionTeardownRequest([]string{"42", "listener", bound.String()})
	require.Nil(t, refusal)
	selector, refusal = request.resolveSelector(service)
	require.Nil(t, refusal)
	require.Equal(t, bound, selector.Listener)
}

// TestSessionTeardownCompletionOffersNamesOnlyForListenerValue checks the
// registered owner's argument position and ensures a listener is never an SID.
func TestSessionTeardownCompletionOffersNamesOnlyForListenerValue(t *testing.T) {
	publishFake(t, &fakeService{
		listeners: []l2tp.ListenerSnapshot{{Name: "edge", Addr: netip.MustParseAddrPort("127.0.0.1:1701")}},
		snapshot:  l2tp.Snapshot{Tunnels: []l2tp.TunnelSnapshot{{Sessions: []l2tp.SessionSnapshot{{LocalSID: 42}}}}},
	})
	for _, completed := range [][]string{nil, {"42"}, {"42", "reason"}, {"42", "reason", "maintenance"}} {
		for _, candidate := range completeSessionTeardown(nil, completed) {
			require.NotEqual(t, "edge", candidate.Value)
		}
	}
	candidates := completeSessionTeardown(nil, []string{"42", "listener"})
	require.Len(t, candidates, 1)
	require.Equal(t, "edge", candidates[0].Value)
}
