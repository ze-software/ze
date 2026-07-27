package role

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// openWithRole builds a ValidateOpenInput carrying one RFC 9234 Role
// capability (code 9, length 1) with the given wire value.
func openWithRole(peer string, value uint8) *sdk.ValidateOpenInput {
	return &sdk.ValidateOpenInput{
		Peer: peer,
		Remote: rpc.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{
				{Code: roleCapCode, Hex: hexByte(value)},
			},
		},
	}
}

// openWithoutRole builds a ValidateOpenInput carrying no Role capability at
// all, which RFC 9234 Section 4.2 explicitly permits ("SHOULD ignore the
// absence ... and proceed with session establishment").
func openWithoutRole(peer string) *sdk.ValidateOpenInput {
	return &sdk.ValidateOpenInput{Peer: peer, Remote: rpc.ValidateOpenMessage{}}
}

// The capability payload encoder is hexByte, defined in validate_test.go; it is
// reused here rather than duplicated.

// TestReconnectWithoutRoleCapabilityClearsStaleRole is the hole-3 regression
// guard. A peer that once advertised a Role capability and then reconnects
// WITHOUT one used to keep the stale learned role forever: the only writer of
// filterRemoteRoles was guarded by len(remoteRoles) > 0, and the only clear was
// a wholesale wipe on reconfigure. resolvePeerRole PREFERS the learned value
// over the config complement, so the peer stayed enforced against a
// relationship it no longer claimed.
//
// VALIDATES: a session whose OPEN carries no Role capability clears the role
// learned from a previous session, so resolvePeerRole falls back to the
// configured complement instead of a stale capability value.
// PREVENTS: a peer being gated by RFC 9234 Section 5 rules for a role it has
// stopped advertising.
func TestReconnectWithoutRoleCapabilityClearsStaleRole(t *testing.T) {
	clearFilterState(t)

	// We are this peer's Provider, so the peer IS our Customer by config.
	configs := map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}
	nameToIP := map[string]string{"upstream": "10.0.0.1"}
	setFilterState(configs, nameToIP)

	// Session 1: the peer advertises Provider (value 0).
	out := applyValidateOpen(configs, nameToIP, openWithRole("upstream", 0))
	require.NotNil(t, out)
	_, learned := getFilterConfig("10.0.0.1")
	require.Equal(t, roleProvider, learned, "session 1 must record the advertised role")

	// Session 2: the peer reconnects with no Role capability.
	out = applyValidateOpen(configs, nameToIP, openWithoutRole("upstream"))
	require.NotNil(t, out)
	assert.True(t, out.Accept,
		"RFC 9234 Section 4.2: absence of the capability is accepted when strict is unset")

	cfg, learned := getFilterConfig("10.0.0.1")
	assert.Empty(t, learned,
		"the stale capability role must not survive a session that does not advertise it")

	// The gates must now resolve from config, not from the stale capability.
	assert.Equal(t, roleCustomer, resolvePeerRole(learned, cfg),
		"with the stale role cleared, the peer resolves to the configured complement (we are Provider, so it is our Customer)")
}

// TestReconnectWithNewRoleCapabilityWinsOverPrevious is the ordering half of
// the fix. Clearing must never cost a freshly advertised role: the clear
// happens on the same OPEN that would set it, for the same session, so the last
// OPEN always decides.
//
// VALIDATES: a reconnect advertising a DIFFERENT role ends with the new value,
// not cleared and not the previous one.
// PREVENTS: an over-eager clear that drops a role the current session did
// advertise (the failure mode a session-down clear would introduce, since the
// state event carries no session identity to order it against an OPEN).
func TestReconnectWithNewRoleCapabilityWinsOverPrevious(t *testing.T) {
	clearFilterState(t)

	configs := map[string]*peerRoleConfig{"10.0.0.1": {role: rolePeer}}
	nameToIP := map[string]string{"lateral": "10.0.0.1"}
	setFilterState(configs, nameToIP)

	// Session 1: peer advertises Provider (0).
	applyValidateOpen(configs, nameToIP, openWithRole("lateral", 0))
	_, learned := getFilterConfig("10.0.0.1")
	require.Equal(t, roleProvider, learned)

	// Session 2: peer now advertises Peer (4).
	applyValidateOpen(configs, nameToIP, openWithRole("lateral", 4))
	_, learned = getFilterConfig("10.0.0.1")
	assert.Equal(t, rolePeer, learned, "the newest OPEN must win")

	// Session 3: back to Provider. Repeated flaps must keep converging on the
	// value the current session advertised.
	applyValidateOpen(configs, nameToIP, openWithRole("lateral", 0))
	_, learned = getFilterConfig("10.0.0.1")
	assert.Equal(t, roleProvider, learned, "each OPEN re-establishes the learned role")
}

// TestReconnectWithUnassignedRoleValueClearsStaleRole covers the third branch:
// the peer sends a Role capability whose value is outside RFC 9234 Table 1
// (5-255 unassigned), so roleValueToName cannot map it. Keeping the previous
// session's role there would be the same stale-value bug wearing a different
// hat.
//
// VALIDATES: an unmappable role value clears the learned role rather than
// leaving the previous session's value in place.
// PREVENTS: a fail-open path where an unassigned role value silently preserves
// a stale relationship.
func TestReconnectWithUnassignedRoleValueClearsStaleRole(t *testing.T) {
	clearFilterState(t)

	configs := map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}
	nameToIP := map[string]string{"upstream": "10.0.0.1"}
	setFilterState(configs, nameToIP)

	applyValidateOpen(configs, nameToIP, openWithRole("upstream", 0))
	_, learned := getFilterConfig("10.0.0.1")
	require.Equal(t, roleProvider, learned)

	// Value 7 is unassigned in RFC 9234 Table 1.
	applyValidateOpen(configs, nameToIP, openWithRole("upstream", 7))
	_, learned = getFilterConfig("10.0.0.1")
	assert.Empty(t, learned, "an unassigned role value must not preserve the previous role")
}

// TestClearedRoleIsKeyedLikeTheSetter pins the clear to the SAME key
// resolution the setter uses. OnValidateOpen identifies a peer by NAME while
// the filters look up by ADDRESS, so a clear that skipped the name->IP
// translation would silently leave the stale entry under the address key that
// the filters actually read.
//
// VALIDATES: clearing resolves the peer name through filterNameToIP exactly as
// setFilterRemoteRole does.
// PREVENTS: a half-fix that deletes an unreachable key and leaves the live one.
func TestClearedRoleIsKeyedLikeTheSetter(t *testing.T) {
	clearFilterState(t)

	configs := map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}
	nameToIP := map[string]string{"named-peer": "10.0.0.1"}
	setFilterState(configs, nameToIP)

	// Set by NAME (as OnValidateOpen does); the entry lands under the IP.
	setFilterRemoteRole("named-peer", roleCustomer)
	_, learned := getFilterConfig("10.0.0.1")
	require.Equal(t, roleCustomer, learned, "setter resolves name -> IP")

	// Clear by NAME; the IP-keyed entry the filters read must be the one removed.
	clearFilterRemoteRole("named-peer")
	_, learned = getFilterConfig("10.0.0.1")
	assert.Empty(t, learned, "clear must remove the same key the setter wrote")
}

// TestStaleRoleClearedEvenWhenOpenIsRejected covers a peer whose reconnect is
// rejected for a role mismatch. The pre-existing code deliberately records the
// remote role "even if validation rejects", so the clear must be symmetric:
// otherwise a rejected OPEN with no capability would leave the stale role
// behind precisely when the relationship is most in doubt.
//
// VALIDATES: the learned role is (re)established from the OPEN regardless of
// whether validateOpenRolePair accepts the session.
// PREVENTS: an asymmetry where accept and reject paths disagree about the
// learned role.
func TestStaleRoleClearedEvenWhenOpenIsRejected(t *testing.T) {
	clearFilterState(t)

	// strict: a peer that sends no Role capability is rejected.
	configs := map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider, strict: true}}
	nameToIP := map[string]string{"upstream": "10.0.0.1"}
	setFilterState(configs, nameToIP)

	applyValidateOpen(configs, nameToIP, openWithRole("upstream", 3)) // Customer: valid pair
	_, learned := getFilterConfig("10.0.0.1")
	require.Equal(t, roleCustomer, learned)

	out := applyValidateOpen(configs, nameToIP, openWithoutRole("upstream"))
	require.NotNil(t, out)
	require.False(t, out.Accept, "strict mode rejects a peer that sends no Role capability")

	_, learned = getFilterConfig("10.0.0.1")
	assert.Empty(t, learned, "a rejected OPEN must not leave the previous session's role behind")
}

// TestReconnectWithoutRoleIsObservable checks the operator signal for the
// clear. A role silently disappearing is as confusing as a role silently
// persisting, so the transition is logged.
//
// VALIDATES: dropping a previously learned role emits a log line naming the peer.
// PREVENTS: a silent capability regression on a peer (e.g. after a peer-side
// downgrade) being invisible to the operator.
func TestReconnectWithoutRoleIsObservable(t *testing.T) {
	clearFilterState(t)

	var buf bytes.Buffer
	prev := logger()
	ConfigureLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { ConfigureLogger(prev); loggerPtr.Store(slogutil.DiscardLogger()) })

	configs := map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}
	nameToIP := map[string]string{"upstream": "10.0.0.1"}
	setFilterState(configs, nameToIP)

	applyValidateOpen(configs, nameToIP, openWithRole("upstream", 0))
	buf.Reset()

	applyValidateOpen(configs, nameToIP, openWithoutRole("upstream"))
	assert.Contains(t, buf.String(), "role capability withdrawn",
		"losing a previously learned role must be visible")
	assert.Contains(t, buf.String(), "upstream", "the log must name the peer")
}
