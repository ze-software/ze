// Design: docs/architecture/meta/role.md -- Learned role lifecycle
// Related: role.go -- forgetRemovedPeer, recordNoRemoteRole, remoteRoleRecorded
// Related: removal.go -- the removal report that frees a learned role, on both transports
//
// Spec: spec-bgp-role-peer-removed-notice (AC-1, AC-2), closed; the
// behavior it proves is in role.md, "Learned role lifecycle".

package role

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPeerRemovedClearsLearnedRole drives the removal through the state event
// an in-process plugin is delivered, the reactor's removal report.
//
// VALIDATES: a removal the reactor announces (a state down with reason
// rpc.ReasonPeerRemoved) deletes the learned role of a dynamic group's member,
// keyed by its own name, and a peer created again under that name whose OPEN
// carries no Role is not reported as withdrawing the old one.
// PREVENTS: the deleted peer's role being read by the next peer of that name.
func TestPeerRemovedClearsLearnedRole(t *testing.T) {
	clearFilterState(t)
	configs := map[string]*peerRoleConfig{configjson.CapabilityGroupKey("edge"): {role: roleProvider}}
	setFilterState(configs, nil)

	const name = "peer-192.0.2.9"
	addr := netip.MustParseAddr("192.0.2.9")
	open := openWithRole(name, 3) // Customer
	open.Group = "edge"
	applyValidateOpen(configs, nil, open)
	require.True(t, remoteRoleRecorded(name, ""), "the OPEN's role is recorded under the peer's name")

	require.NoError(t, handleRoleStructuredEvents([]any{&rpc.StructuredEvent{
		PeerAddress: addr.String(), PeerName: name,
		EventType: rpc.EventKindState, State: rpc.SessionStateDown, Reason: rpc.ReasonPeerRemoved,
	}}))

	assert.False(t, remoteRoleRecorded(name, ""), "the removed peer's role must be gone")
	assert.Empty(t, recordNoRemoteRole(name, "edge"),
		"a peer created again under the name must find no previous role to withdraw")
}

// TestRolelessPeerStoresNoLearnedRole covers a peer whose config sets no role
// import: its lists carry no bgp-role member, so no filter reads its learned
// role and the implicit state grant never delivers its removal.
//
// VALIDATES: an OPEN from such a peer, Role capability or none, stores nothing.
// PREVENTS: a learned role that no removal can ever free.
func TestRolelessPeerStoresNoLearnedRole(t *testing.T) {
	clearFilterState(t)
	setFilterState(map[string]*peerRoleConfig{}, nil)

	const name = "peer-192.0.2.10"
	applyValidateOpen(nil, nil, openWithRole(name, 3)) // Customer
	assert.False(t, remoteRoleRecorded(name, ""), "a roleless peer's OPEN role must not be stored")
	applyValidateOpen(nil, nil, openWithoutRole(name))
	assert.False(t, remoteRoleRecorded(name, ""), "a roleless peer's OPEN without a role must not be stored")
}

// TestPeerRemovedClearsRoleKeyedByAddress covers a configured peer, whose
// role is stored under its address (filterKeyLocked maps the name to it),
// through the JSON state event a forked plugin is delivered.
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

	require.NoError(t, handleRoleEvent(`{"type":"bgp","bgp":{"message":{"type":"state"},`+
		`"peer":{"name":"edge1","remote":{"address":"10.0.0.1","as":65001}},"state":"down","reason":"removed"}}`))

	assert.False(t, remoteRoleRecorded("10.0.0.1", ""), "the removed peer's role must be gone")
	assert.True(t, remoteRoleRecorded("10.0.0.2", ""), "another peer's role must stay")
}

// TestRemovalQueuedBehindReCreatedPeerKeepsItsRole reproduces the interleaving
// of the two loops a plugin is delivered on: the removal down of a peer is
// still queued on the event loop when a peer created again under the same
// name validates its OPEN on the callback loop.
//
// METHOD: the generations are the ones the reactor stamps. The old session's
// OPEN is stamped 4, the removal issues 5 as it deletes the peer, and the new
// peer's OPEN is stamped with the clock reading after it, 5. The down is then
// handled, after the new OPEN, on both transports.
//
// VALIDATES: the queued removal leaves the re-created peer's role in place,
// and a removal naming a generation above the stored OPEN frees it.
// PREVENTS: a stale removal deleting the new session's learned role, so the
// export set read the peer as `unknown` until its next OPEN.
func TestRemovalQueuedBehindReCreatedPeerKeepsItsRole(t *testing.T) {
	clearFilterState(t)
	configs := map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}
	nameToIP := map[string]string{"edge1": "10.0.0.1"}
	setFilterState(configs, nameToIP)

	old := openWithRole("edge1", 3) // Customer
	old.Generation = 4
	applyValidateOpen(configs, nameToIP, old)
	require.True(t, remoteRoleRecorded("10.0.0.1", ""))

	const removedGeneration = 5
	recreated := openWithRole("edge1", 3) // Customer
	recreated.Generation = removedGeneration
	applyValidateOpen(configs, nameToIP, recreated)

	require.NoError(t, handleRoleStructuredEvents([]any{&rpc.StructuredEvent{
		PeerAddress: "10.0.0.1", PeerName: "edge1", EventType: rpc.EventKindState,
		State: rpc.SessionStateDown, Reason: rpc.ReasonPeerRemoved, SessionGeneration: removedGeneration,
	}}))
	assert.True(t, remoteRoleRecorded("10.0.0.1", ""),
		"a removal issued before the re-created peer's OPEN must keep that OPEN's role")

	require.NoError(t, handleRoleEvent(`{"type":"bgp","bgp":{"message":{"type":"state"},`+
		`"peer":{"name":"edge1","remote":{"address":"10.0.0.1","as":65001}},"state":"down","reason":"removed",`+
		`"session-generation":"5"}}`))
	assert.True(t, remoteRoleRecorded("10.0.0.1", ""),
		"the forked transport must order the removal the same way")

	require.NoError(t, handleRoleEvent(`{"type":"bgp","bgp":{"message":{"type":"state"},`+
		`"peer":{"name":"edge1","remote":{"address":"10.0.0.1","as":65001}},"state":"down","reason":"removed",`+
		`"session-generation":"6"}}`))
	assert.False(t, remoteRoleRecorded("10.0.0.1", ""),
		"the removal of the peer that stored the role must free it")
}
