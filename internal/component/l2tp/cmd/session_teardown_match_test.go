// Design: docs/guide/l2tp.md -- clearing a subscriber session by user name or address

package cmd

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// matchTestSessions is a live inventory of three subscribers on two tunnels.
// alice@example.net holds 192.0.2.10; bob@example.net holds 192.0.2.11.
func matchTestSessions(t *testing.T, aliceTwice bool) (l2tp.Snapshot, []subscriberid.L2TPBirth) {
	t.Helper()
	var births []subscriberid.L2TPBirth
	for _, reference := range []string{
		"l2tp-00000000000000000000000000000001",
		"l2tp-00000000000000000000000000000002",
		"l2tp-00000000000000000000000000000003",
	} {
		birth, err := subscriberid.ParseL2TPBirth(reference)
		require.NoError(t, err)
		births = append(births, birth)
	}
	listener := netip.MustParseAddrPort("192.0.2.1:1701")
	third := "carol@example.net"
	if aliceTwice {
		third = "alice@example.net"
	}
	return l2tp.Snapshot{Tunnels: []l2tp.TunnelSnapshot{
		{LocalTID: 1, Sessions: []l2tp.SessionSnapshot{
			{LocalSID: 3, TunnelLocalTID: 1, Listener: listener, Birth: births[0], Username: "alice@example.net", AssignedAddr: netip.MustParseAddr("192.0.2.10")},
			{LocalSID: 4, TunnelLocalTID: 1, Listener: listener, Birth: births[1], Username: "bob@example.net", AssignedAddr: netip.MustParseAddr("192.0.2.11")},
		}},
		{LocalTID: 2, Sessions: []l2tp.SessionSnapshot{
			{LocalSID: 3, TunnelLocalTID: 2, Listener: listener, Birth: births[2], Username: third, AssignedAddr: netip.MustParseAddr("2001:db8::10")},
		}},
	}}, births
}

func publishMatchTestService(t *testing.T, snapshot l2tp.Snapshot) (*sessionTeardownServiceSpy, *sessionTeardownCaptureSpy) {
	t.Helper()
	capture := &sessionTeardownCaptureSpy{}
	service := &sessionTeardownServiceSpy{
		fakeService: &fakeService{snapshot: snapshot},
		captured:    capture,
	}
	publishFake(t, service)
	return service, capture
}

// TestClearSessionByUserTearsDownTheOneMatchingOriginal checks that a clear by
// user name captures the one matching row by every coordinate and its Birth,
// records the audit, tears it down, and answers which session it was.
func TestClearSessionByUserTearsDownTheOneMatchingOriginal(t *testing.T) {
	snapshot, births := matchTestSessions(t, false)
	service, capture := publishMatchTestService(t, snapshot)
	ctx := &pluginserver.CommandContext{RequestContext: t.Context()}
	response, err := handleSessionTeardownUser(ctx, commandtest.Args("alice@example.net", "reason", "maintenance", "window", "cause", "6"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	require.Equal(t, []l2tp.SessionSelector{{SID: 3, TID: 1, Listener: netip.MustParseAddrPort("192.0.2.1:1701"), Birth: births[0]}}, service.selectors)
	require.Equal(t, 1, capture.audits)
	require.Equal(t, 1, capture.clears)
	require.Zero(t, service.legacyClears, "a clear by user name must never fall back to a teardown by session ID")
	answer := responseString(t, response)
	require.Contains(t, answer, `"action":"session-teardown"`)
	require.Contains(t, answer, `"session-id":3`)
	require.Contains(t, answer, `"tunnel-id":1`)
	require.Contains(t, answer, `"username":"alice@example.net"`)
	require.Contains(t, answer, `"reason":"maintenance window"`)
	require.Contains(t, answer, `"status":"sent"`)
}

// TestClearSessionByUserRefusesSeveralMatchesAndClearsNone checks the answer
// the RADIUS Disconnect path gives several matches: nothing is captured or
// cleared, and the refusal lists one qualified clear by ID for each match.
func TestClearSessionByUserRefusesSeveralMatchesAndClearsNone(t *testing.T) {
	snapshot, births := matchTestSessions(t, true)
	service, capture := publishMatchTestService(t, snapshot)
	ctx := &pluginserver.CommandContext{RequestContext: t.Context()}
	response, err := handleSessionTeardownUser(ctx, commandtest.Args("alice@example.net"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, response.Status)
	require.Empty(t, service.selectors)
	require.Zero(t, capture.audits)
	require.Zero(t, capture.clears)
	require.Equal(t, `2 L2TP sessions match user "alice@example.net"`, strings.SplitN(response.Error, "\n", 2)[0])
	require.Contains(t, response.Error, "clear l2tp session id 3 tunnel-id 1 listener 192.0.2.1:1701 expected-id "+births[0].String())
	require.Contains(t, response.Error, "clear l2tp session id 3 tunnel-id 2 listener 192.0.2.1:1701 expected-id "+births[2].String())
	require.True(t, strings.HasSuffix(response.Error, "\nfix: show l2tp sessions"), response.Error)
}

// TestClearSessionByUserOrAddressRefusesNoMatch checks that a value no live
// session carries is refused with the fix, and selects nothing. A user name
// differing only in case is not the subscriber's name.
func TestClearSessionByUserOrAddressRefusesNoMatch(t *testing.T) {
	snapshot, _ := matchTestSessions(t, false)
	for _, test := range []struct {
		name    string
		handler func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error)
		value   string
		problem string
	}{
		{name: "user", handler: handleSessionTeardownUser, value: "Alice@example.net", problem: `No L2TP session matches user "Alice@example.net"`},
		{name: "address", handler: handleSessionTeardownAddress, value: "192.0.2.99", problem: "No L2TP session matches address 192.0.2.99"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, capture := publishMatchTestService(t, snapshot)
			ctx := &pluginserver.CommandContext{RequestContext: t.Context()}
			response, err := test.handler(ctx, commandtest.Args(test.value))
			require.NoError(t, err)
			require.Equal(t, plugin.StatusError, response.Status)
			require.Empty(t, service.selectors)
			require.Zero(t, capture.clears)
			require.Equal(t, test.problem, strings.SplitN(response.Error, "\n", 2)[0])
			require.True(t, strings.HasSuffix(response.Error, "\nfix: show l2tp sessions"), response.Error)
		})
	}
}

// TestClearSessionByAddressTearsDownTheSessionHoldingIt checks that the IPCP
// address selects its one session, and the answer echoes the address.
func TestClearSessionByAddressTearsDownTheSessionHoldingIt(t *testing.T) {
	snapshot, births := matchTestSessions(t, false)
	service, capture := publishMatchTestService(t, snapshot)
	ctx := &pluginserver.CommandContext{RequestContext: t.Context()}
	response, err := handleSessionTeardownAddress(ctx, commandtest.Args("192.0.2.11"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	require.Equal(t, []l2tp.SessionSelector{{SID: 4, TID: 1, Listener: netip.MustParseAddrPort("192.0.2.1:1701"), Birth: births[1]}}, service.selectors)
	require.Zero(t, capture.audits, "no audit keyword was typed")
	require.Equal(t, 1, capture.clears)
	answer := responseString(t, response)
	require.Contains(t, answer, `"session-id":4`)
	require.Contains(t, answer, `"address":"192.0.2.11"`)
}

// TestClearSessionByUserOrAddressRefusesMalformedInput requires refusal before
// any selection: a missing, empty, option-like or over-long name, a value that
// is not IPv4, a session-ID qualifier, and malformed audit keywords. The bound
// counts octets, so 127 two-octet characters (254 octets) are refused too.
func TestClearSessionByUserOrAddressRefusesMalformedInput(t *testing.T) {
	for _, args := range [][]string{
		nil, {""}, {"-help"}, {strings.Repeat("a", 254)}, {strings.Repeat("é", 127)},
		{"alice@example.net", "tunnel-id", "1"}, {"alice@example.net", "expected-id", "l2tp-00000000000000000000000000000001"},
		{"alice@example.net", "reason"}, {"alice@example.net", "cause", "invalid"}, {"alice@example.net", "actor", "a", "actor", "b"},
	} {
		t.Run("user "+strings.Join(args, " "), func(t *testing.T) {
			_, refusal := parseSessionTeardownUser(args)
			require.NotNil(t, refusal)
		})
	}
	for _, args := range [][]string{
		nil, {""}, {"192.0.2"}, {"2001:db8::10"}, {"::ffff:192.0.2.10"}, {"192.0.2.10", "listener", "edge"},
	} {
		t.Run("address "+strings.Join(args, " "), func(t *testing.T) {
			_, refusal := parseSessionTeardownAddress(args)
			require.NotNil(t, refusal)
		})
	}
	request, refusal := parseSessionTeardownUser([]string{strings.Repeat("a", 253)})
	require.Nil(t, refusal, "a 253-octet name is the longest RADIUS User-Name and is accepted")
	require.Equal(t, strings.Repeat("a", 253), request.value)
}

// TestClearSessionByUserOrAddressCompletesLiveValues checks that tab offers
// the live user names and IPv4 addresses, each once, then only the audit
// keywords not yet typed, and nothing at a keyword's value.
func TestClearSessionByUserOrAddressCompletesLiveValues(t *testing.T) {
	snapshot, _ := matchTestSessions(t, true)
	publishMatchTestService(t, snapshot)
	values := func(candidates []pluginserver.Completion) []string {
		var out []string
		for _, candidate := range candidates {
			out = append(out, candidate.Value)
		}
		return out
	}
	require.Equal(t, []string{"alice@example.net", "bob@example.net"}, values(completeSessionTeardownUser(nil, nil)))
	require.Equal(t, []string{"192.0.2.10", "192.0.2.11"}, values(completeSessionTeardownAddress(nil, nil)))
	require.Equal(t, []string{"actor", "reason", "cause"}, values(completeSessionTeardownUser(nil, []string{"alice@example.net"})))
	require.Equal(t, []string{"actor", "cause"}, values(completeSessionTeardownUser(nil, []string{"alice@example.net", "reason", "maintenance"})))
	require.Empty(t, completeSessionTeardownAddress(nil, []string{"192.0.2.10", "reason"}))
}
