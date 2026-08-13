package role

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// captureRoleLog installs a capturing WARN-level logger for one test.
func captureRoleLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := logger()
	ConfigureLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { ConfigureLogger(prev); loggerPtr.Store(slogutil.DiscardLogger()) })
	return &buf
}

// TestRoleConfigWithoutUsableRemoteIPIsRejected is the hole-2 regression guard.
//
// The delivered config keys peers by NAME with the address nested at
// connection > remote > ip (internal/component/bgp/configjson/traverse.go:69-72).
// extractPeerRoleConfigs used to fall back to the peer NAME as the map key when
// no remote IP resolved, while all three runtime readers look up by ADDRESS
// (otc.go: OTCIngressFilter and both OTCEgressFilter lookups). The config was
// therefore stored under a key nothing could ever read, and a nil cfg makes
// every RFC 9234 Section 5 gate go permissive -- the zero-value trap of
// ai/rules/evidence.md, where a miss is indistinguishable from
// "this peer has no role configured".
//
// The fallback is only wrong when the peer NAME is not itself an address.
// Operators very commonly name a peer by its own address, and then the name IS
// the key every reader uses, so that shape is kept (see
// TestRoleConfigNamedByAddressWithoutConnectionBlockIsKept). Only a name that
// can never equal PeerFilterInfo.Address.String() is rejected.
//
// Reject-and-say-so is right rather than store-anyway, because such a peer
// never becomes a runtime peer either:
// internal/component/bgp/reactor/config.go:76-78 fails an empty remote IP with
// ErrIncompleteConfig and :516-521 skips it. The role config is inert either
// way; the defect is that it was inert SILENTLY. This matches the sibling
// precedent in bgp-rpki (plugins/rpki/rpki_config.go:283-287).
//
// VALIDATES: a role-configured peer whose key can never be reached produces NO
// config entry, and the operator is told why.
// PREVENTS: role config that silently does nothing -- neither applied nor
// reported.
func TestRoleConfigWithoutUsableRemoteIPIsRejected(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string // substring the warning must carry
	}{
		{
			name: "no_connection_block",
			json: `{"bgp":{"peer":{"my-upstream":{"role":{"import":"provider"}}}}}`,
			want: "my-upstream",
		},
		{
			name: "connection_without_remote_ip",
			json: `{"bgp":{"peer":{"my-upstream":{"connection":{"local":{"ip":"10.0.0.9"}},"role":{"import":"provider"}}}}}`,
			want: "my-upstream",
		},
		{
			// A local IP is not a peer key: only the REMOTE address is ever
			// looked up, so a name that is not an address stays unreachable.
			name: "hostname_style_name",
			json: `{"bgp":{"peer":{"rr1.example.net":{"role":{"import":"rs-client"}}}}}`,
			want: "rr1.example.net",
		},
		{
			// A peer nested in a dynamic group inherits ip "dynamic". That string
			// is non-empty, so it used to sail past the empty-string fallback and
			// be stored under the literal key "dynamic" -- a key no lookup can hit.
			// reactor/config.go:79-81 rejects "dynamic" at peer level outright.
			name: "dynamic_group_placeholder",
			json: `{"bgp":{"group":{"transit":{"connection":{"remote":{"ip":"dynamic"}},"peer":{"dyn-peer":{"role":{"import":"customer"}}}}}}}`,
			want: "dyn-peer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureRoleLog(t)

			configs, nameToIP := extractPeerRoleConfigs(tt.json)

			assert.Empty(t, configs,
				"a peer with no usable remote IP must not produce a config entry under ANY key")
			assert.Empty(t, nameToIP, "no name->IP mapping can exist without an IP")

			log := buf.String()
			assert.Contains(t, log, "role config ignored",
				"the operator must be told the role config is not being applied")
			assert.Contains(t, log, tt.want, "the warning must name the peer")
		})
	}
}

// TestRoleConfigNamedByAddressWithoutConnectionBlockIsKept pins the shape the
// keying fix must NOT reject. A peer named by its own address and carrying no
// connection block is the shorthand used throughout the existing config tests
// (and by operators): the delivered map key is the peer name, and here that
// name is exactly the string every reader looks up, so the fallback is correct.
//
// This is the boundary of the reject: unreachable keys are refused, reachable
// ones are not. Without it, the fix would silently delete working role config
// -- which is how it first broke the RFC 9234 Section 4.1 capability tests.
//
// VALIDATES: a peer whose name parses as an IP address still gets role config
// keyed by that name, with no warning.
// PREVENTS: the unreachable-key rejection over-reaching into valid config.
func TestRoleConfigNamedByAddressWithoutConnectionBlockIsKept(t *testing.T) {
	buf := captureRoleLog(t)

	for _, name := range []string{"10.0.0.1", "2001:db8::1"} {
		t.Run(name, func(t *testing.T) {
			configs, _ := extractPeerRoleConfigs(
				`{"bgp":{"peer":{"` + name + `":{"role":{"import":"customer"}}}}}`)

			require.Contains(t, configs, name,
				"a peer named by its own address is reachable: the name IS the reader's key")
			assert.Equal(t, roleCustomer, configs[name].role)
		})
	}
	assert.Empty(t, buf.String(), "a reachable key must not warn")
}

// TestRoleCapabilityNotDeclaredForUnusablePeer proves the whole surface stays
// consistent: extractRoleCapabilities derives from the same map, so a peer that
// is rejected for keying must not still get a Role capability declared for it.
//
// VALIDATES: no CapabilityDecl is emitted for a peer whose role config was
// rejected for having no usable remote IP.
// PREVENTS: a half-applied peer that advertises a Role capability while none of
// the Section 5 gates that give the role meaning can ever run for it.
func TestRoleCapabilityNotDeclaredForUnusablePeer(t *testing.T) {
	captureRoleLog(t)

	caps := extractRoleCapabilities(`{"bgp":{"peer":{"my-upstream":{"role":{"import":"provider"}}}}}`)
	assert.Empty(t, caps, "no Role capability for a peer whose role config is unusable")
}

// TestRoleConfigWithUsableRemoteIPStillKeyedByAddress is the control. The
// reject path must be scoped to genuinely unusable peers and must not disturb
// the normal shape, including a peer that inherits its remote IP from a
// non-dynamic group.
//
// VALIDATES: a peer with a resolvable remote IP (its own or inherited from its
// group) is still keyed by that address, with the name->IP mapping populated.
// PREVENTS: the fix over-reaching and dropping valid role config.
func TestRoleConfigWithUsableRemoteIPStillKeyedByAddress(t *testing.T) {
	captureRoleLog(t)

	t.Run("peer_own_ip", func(t *testing.T) {
		configs, nameToIP := extractPeerRoleConfigs(
			`{"bgp":{"peer":{"my-upstream":{"connection":{"remote":{"ip":"10.0.0.1"}},"role":{"import":"provider"}}}}}`)

		require.Contains(t, configs, "10.0.0.1", "keyed by remote IP")
		assert.NotContains(t, configs, "my-upstream", "never keyed by name")
		assert.Equal(t, roleProvider, configs["10.0.0.1"].role)
		assert.Equal(t, "10.0.0.1", nameToIP["my-upstream"])
	})

	t.Run("ip_inherited_from_group", func(t *testing.T) {
		// The plugin receives the RAW tree (group fields are not merged into the
		// peer), so PeerRemoteIP's groupMap fallback is what makes this work.
		configs, nameToIP := extractPeerRoleConfigs(
			`{"bgp":{"group":{"upstreams":{"connection":{"remote":{"ip":"10.0.0.2"}},"role":{"import":"customer"},"peer":{"grouped":{}}}}}}`)

		require.Contains(t, configs, "10.0.0.2", "group-inherited remote IP is usable")
		assert.Equal(t, roleCustomer, configs["10.0.0.2"].role)
		assert.Equal(t, "10.0.0.2", nameToIP["grouped"])
	})
}

// TestUnusableRoleConfigDoesNotShadowUsablePeers checks the reject path is
// per-peer: one bad peer must not cost the good ones their config.
//
// VALIDATES: rejecting one peer's unusable role config leaves other peers'
// config intact.
// PREVENTS: an early return that abandons the whole config map on the first
// unusable peer.
func TestUnusableRoleConfigDoesNotShadowUsablePeers(t *testing.T) {
	captureRoleLog(t)

	configs, nameToIP := extractPeerRoleConfigs(`{"bgp":{"peer":{
		"broken":{"role":{"import":"provider"}},
		"good":{"connection":{"remote":{"ip":"10.0.0.3"}},"role":{"import":"customer"}}
	}}}`)

	require.Contains(t, configs, "10.0.0.3", "the usable peer survives")
	assert.Equal(t, roleCustomer, configs["10.0.0.3"].role)
	assert.NotContains(t, configs, "broken", "the unusable peer is not stored under its name")
	assert.Len(t, configs, 1, "exactly one peer is configurable here")
	assert.Equal(t, "10.0.0.3", nameToIP["good"])
	assert.NotContains(t, nameToIP, "broken")
}

// TestRoleCapabilityDeclaredForDynamicGroup covers AC-4's declaration half.
//
// An IXP route server is a listen-range group: it names no peer at all, and its
// members are built from the group's template when a connection arrives. The
// role an operator states on that group used to reach nothing, because the
// traversal never visited the group and the "dynamic" placeholder was rejected
// as an unusable key.
//
// VALIDATES: a dynamic group stating a role declares RFC 9234 capability 9 under
// the group selector, carrying the value the group stated.
// PREVENTS: a route server whose members advertise no Role capability while the
// operator reads the role in the running config.
func TestRoleCapabilityDeclaredForDynamicGroup(t *testing.T) {
	captureRoleLog(t)

	caps := extractRoleCapabilities(`{"bgp":{"group":{"ix":{
		"connection":{"remote":{"ip":"dynamic","range":["192.0.2.0/24"]}},
		"role":{"import":"rs","strict":true}}}}}`)

	require.Len(t, caps, 1, "the group's role must be declared exactly once")
	require.Len(t, caps[0].Peers, 1)
	assert.Equal(t, "group:ix", caps[0].Peers[0],
		"the selector must be the group, which is the identity a dynamic member carries")
	assert.Equal(t, uint8(roleCapCode), caps[0].Code)
	// RFC 9234 Section 4, Table 1: RS is value 1. The assertion is on the STATED
	// value rather than on the presence of a capability, and 1 is not the zero
	// byte a miss would produce -- 0 is Provider.
	assert.Equal(t, "01", caps[0].Payload, "the group's stated role, not a default")
}

// TestRoleConfigForDynamicGroupIsNotKeyedByAnAddress is the collision guard
// (A-7). A group name and a peer name share no uniqueness check in
// config.ResolveBGPTree, so the template must never occupy the key space the
// address readers use.
//
// VALIDATES: the template is stored under the prefixed group selector and under
// no address-shaped key, and a static peer in the same config keeps its own.
// PREVENTS: a group named like a peer answering that peer's role lookup.
func TestRoleConfigForDynamicGroupIsNotKeyedByAnAddress(t *testing.T) {
	captureRoleLog(t)

	configs, nameToIP := extractPeerRoleConfigs(`{"bgp":{
		"peer":{"ix":{"connection":{"remote":{"ip":"10.0.0.1"}},"role":{"import":"provider"}}},
		"group":{"ix":{
			"connection":{"remote":{"ip":"dynamic","range":["192.0.2.0/24"]}},
			"role":{"import":"rs"}}}}}`)

	require.Contains(t, configs, "group:ix", "the template is keyed by the group selector")
	assert.Equal(t, roleRS, configs["group:ix"].role)

	require.Contains(t, configs, "10.0.0.1", "the peer of the same name keeps its address key")
	assert.Equal(t, roleProvider, configs["10.0.0.1"].role,
		"the group's template answered the peer's lookup")

	assert.NotContains(t, configs, "ix", "the bare group name is never a key")
	assert.NotContains(t, configs, dynamicPeerIP, "the placeholder is never a key")
	assert.Equal(t, "10.0.0.1", nameToIP["ix"], "the name map still resolves the peer")
}

// TestRoleConfigStillRejectsANamedPeerInheritingThePlaceholder is the control.
// A NAMED peer inside a dynamic group is not built from the template, so it has
// no address any reader can produce and its role config stays unusable.
//
// VALIDATES: the template branch is scoped to the template visit alone.
// PREVENTS: the fix widening into peers whose role config can still never be
// looked up.
func TestRoleConfigStillRejectsANamedPeerInheritingThePlaceholder(t *testing.T) {
	buf := captureRoleLog(t)

	configs, _ := extractPeerRoleConfigs(`{"bgp":{"group":{"ix":{
		"connection":{"remote":{"ip":"dynamic","range":["192.0.2.0/24"]}},
		"peer":{"named":{"role":{"import":"provider"}}}}}}}`)

	assert.NotContains(t, configs, "named", "a named peer is never keyed by its name")
	assert.NotContains(t, configs, "group:ix", "the group stated no role of its own")
	assert.Contains(t, buf.String(), "no usable remote ip",
		"the operator must be told the named peer's role does nothing")
}
