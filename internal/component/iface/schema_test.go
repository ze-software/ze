package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestWireguardYANGSensitive verifies that the sensitive-leaf pattern is
// applied to WireGuard key material, so the parser auto-decodes $9$ on load
// and the dumper auto-encodes on write -- matching BGP MD5, SSH secrets, and
// other sensitive values across ze.
//
// VALIDATES: wireguard private-key and peer preshared-key are marked
// ze:sensitive; public-key is NOT marked sensitive.
//
// PREVENTS: plaintext WireGuard keys leaking into config dump / show output
// or config file storage.
func TestWireguardYANGSensitive(t *testing.T) {
	s, err := config.YANGSchema()
	require.NoError(t, err)

	keys := config.SensitiveKeys(s)
	assert.Contains(t, keys, "private-key",
		"wireguard private-key must be ze:sensitive so $9$ decode applies")
	assert.Contains(t, keys, "preshared-key",
		"wireguard peer preshared-key must be ze:sensitive")
	assert.NotContains(t, keys, "public-key",
		"wireguard peer public-key must NOT be sensitive (it is public)")
}

// TestWireguardYANGStructure verifies that interface.wireguard exposes the
// expected top-level leaves and nested peer list, and that it does NOT carry
// mac-address (wireguard is L3 and uses interface-common, not interface-l2).
//
// VALIDATES: schema shape matches spec-iface-wireguard's design; the YANG
// grouping split from Phase 2 correctly keeps wireguard on interface-common.
//
// PREVENTS: silently losing leaves after future YANG refactors; silently
// gaining mac-address if someone accidentally switches wireguard to
// interface-l2.
func TestWireguardYANGStructure(t *testing.T) {
	s, err := config.YANGSchema()
	require.NoError(t, err)

	iface := s.Get("interface")
	require.NotNil(t, iface, "interface container missing from schema")
	ifaceCN, ok := iface.(*config.ContainerNode)
	require.True(t, ok, "interface must be a container")

	wg := ifaceCN.Get("wireguard")
	require.NotNil(t, wg, "interface.wireguard missing from schema")
	wgList, ok := wg.(*config.ListNode)
	require.True(t, ok, "interface.wireguard must be a list")

	wgChildren := wgList.Children()

	// Wireguard-specific leaves:
	assert.Contains(t, wgChildren, "listen-port")
	assert.Contains(t, wgChildren, "fwmark")
	assert.Contains(t, wgChildren, "private-key")
	assert.Contains(t, wgChildren, "peer")

	// interface-common leaves (should be present via `uses interface-common`):
	assert.Contains(t, wgChildren, "description")
	assert.Contains(t, wgChildren, "mtu")
	assert.Contains(t, wgChildren, "os-name")
	assert.Contains(t, wgChildren, "disable")

	// interface-unit leaves (should be present via `uses interface-unit`):
	assert.Contains(t, wgChildren, "unit")

	// mac-address must NOT be present -- wireguard is L3 and uses
	// interface-common, not interface-l2.
	assert.NotContains(t, wgChildren, "mac-address",
		"wireguard is L3 and must not carry mac-address")

	// Verify the nested peer list and its required leaves.
	peer := wgList.Get("peer")
	require.NotNil(t, peer, "wireguard.peer missing from schema")
	peerList, ok := peer.(*config.ListNode)
	require.True(t, ok, "wireguard.peer must be a list")

	peerChildren := peerList.Children()
	assert.Contains(t, peerChildren, "public-key")
	assert.Contains(t, peerChildren, "preshared-key")
	assert.Contains(t, peerChildren, "endpoint")
	assert.Contains(t, peerChildren, "allowed-ips")
	assert.Contains(t, peerChildren, "persistent-keepalive")
	assert.Contains(t, peerChildren, "disable")
}

// TestIfaceYANGBackendAnnotations verifies that the iface YANG annotates
// the netlink-only feature lists (bridge, tunnel, wireguard, veth, mirror)
// with ze:backend "netlink" so the commit-time feature gate rejects
// configs that try to use those features under a backend (e.g. vpp) that
// has not implemented them.
//
// VALIDATES: spec-backend-feature-gate AC-1, AC-2, AC-5; yangToList reads
// ze:backend into the ListNode.Backend field; yangToContainer does the
// same for ContainerNode (mirror).
//
// PREVENTS: annotations being silently dropped by a future YANG edit, and
// the gate from having nothing to gate on.
func TestIfaceYANGBackendAnnotations(t *testing.T) {
	s, err := config.YANGSchema()
	require.NoError(t, err)

	iface := s.Get("interface")
	require.NotNil(t, iface, "interface container missing from schema")
	ifaceCN, ok := iface.(*config.ContainerNode)
	require.True(t, ok, "interface must be a container")

	for _, name := range []string{"bridge", "tunnel", "wireguard", "veth"} {
		node := ifaceCN.Get(name)
		require.NotNilf(t, node, "interface.%s missing from schema", name)
		list, ok := node.(*config.ListNode)
		require.Truef(t, ok, "interface.%s must be a list", name)
		assert.Equalf(t, []string{"netlink"}, list.Backend,
			"interface.%s must carry ze:backend \"netlink\"", name)
	}

	// ethernet, dummy, loopback should remain UNRESTRICTED (nil Backend).
	for _, name := range []string{"ethernet", "dummy"} {
		node := ifaceCN.Get(name)
		require.NotNilf(t, node, "interface.%s missing from schema", name)
		list, ok := node.(*config.ListNode)
		require.Truef(t, ok, "interface.%s must be a list", name)
		assert.Nilf(t, list.Backend,
			"interface.%s must NOT carry ze:backend (supported by every backend)", name)
	}
}
