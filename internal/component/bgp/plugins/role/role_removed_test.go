// Design: docs/architecture/meta/role.md -- Learned role lifecycle
// Related: role.go -- forgetRemovedPeer, recordNoRemoteRole, remoteRoleRecorded
// Related: register.go -- the PeerRemoved hook on the bgp-role filter registration
//
// Spec: spec-bgp-role-peer-removed-notice (AC-1, AC-2), closed; the
// behavior it proves is in role.md, "Learned role lifecycle".

package role

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// TestPeerRemovedClearsLearnedRole drives the removal through the registry the
// reactor calls, not through the handler, so it proves the registration too.
//
// VALIDATES: a removal the reactor announces (filterapi.NotifyPeerRemoved)
// deletes the learned role of a created peer, keyed by its generated name, and
// a peer created again under that name whose OPEN carries no Role is not
// reported as withdrawing the old one.
// PREVENTS: the deleted peer's role being read by the next peer of that name.
func TestPeerRemovedClearsLearnedRole(t *testing.T) {
	clearFilterState(t)
	setFilterState(map[string]*peerRoleConfig{}, nil)

	const name = "peer-192.0.2.9"
	addr := netip.MustParseAddr("192.0.2.9")
	applyValidateOpen(nil, nil, openWithRole(name, 3)) // Customer
	require.True(t, remoteRoleRecorded(name, ""), "the OPEN's role is recorded under the peer's name")

	filterapi.NotifyPeerRemoved(filterapi.PeerFilterInfo{Address: addr, Name: name})

	assert.False(t, remoteRoleRecorded(name, ""), "the removed peer's role must be gone")
	assert.Empty(t, recordNoRemoteRole(name, ""),
		"a peer created again under the name must find no previous role to withdraw")
}

// TestPeerRemovedClearsRoleKeyedByAddress covers a configured peer, whose
// role is stored under its address (filterKeyLocked maps the name to it).
//
// VALIDATES: the removal clears the address key, through the name mapping and
// through the address itself, and leaves another peer's role alone.
// PREVENTS: a name-only clear that misses a role stored by address, and a
// clear wide enough to drop a live peer's role.
func TestPeerRemovedClearsRoleKeyedByAddress(t *testing.T) {
	clearFilterState(t)
	configs := map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}, "10.0.0.2": {role: roleProvider}}
	setFilterState(configs, map[string]string{"edge1": "10.0.0.1", "edge2": "10.0.0.2"})

	applyValidateOpen(configs, map[string]string{"edge1": "10.0.0.1", "edge2": "10.0.0.2"}, openWithRole("edge1", 3))
	applyValidateOpen(configs, map[string]string{"edge1": "10.0.0.1", "edge2": "10.0.0.2"}, openWithRole("edge2", 3))
	require.True(t, remoteRoleRecorded("10.0.0.1", ""))

	filterapi.NotifyPeerRemoved(filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), Name: "edge1"})

	assert.False(t, remoteRoleRecorded("10.0.0.1", ""), "the removed peer's role must be gone")
	assert.True(t, remoteRoleRecorded("10.0.0.2", ""), "another peer's role must stay")
}
