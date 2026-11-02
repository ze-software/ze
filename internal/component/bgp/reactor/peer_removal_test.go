// Related: peer_removal.go — removalLocked and completeRemovals, the one producer under test
package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/test/sim"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// removalDown is one state event a removal path emitted.
type removalDown struct {
	peer   plugin.PeerInfo
	state  rpc.SessionState
	reason string
}

// removalRecorder stands in for the event dispatcher behind Reactor.removalEvents.
// Safe for concurrent use: the idle cleanup emits from a timer callback.
type removalRecorder struct {
	mu    sync.Mutex
	downs []removalDown
}

func (rec *removalRecorder) OnPeerStateChange(peer *plugin.PeerInfo, state rpc.SessionState, reason string) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.downs = append(rec.downs, removalDown{peer: *peer, state: state, reason: reason})
}

func (rec *removalRecorder) seen() []removalDown {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return append([]removalDown(nil), rec.downs...)
}

// recordRemovals puts a recorder where the reactor emits its removal downs.
func recordRemovals(r *Reactor) *removalRecorder {
	rec := &removalRecorder{}
	r.removalEvents = rec
	return rec
}

// requireOneRemovalDown checks that exactly one down with reason removed was
// emitted, for peer, under the identity it had, and with a generation the
// removal issued: above the one the peer held before (before). The Cease may
// advance the peer's generation again, so the one it holds after is no answer.
func requireOneRemovalDown(t *testing.T, rec *removalRecorder, peer *Peer, name, group string, before uint64) {
	t.Helper()
	downs := rec.seen()
	require.Len(t, downs, 1, "the removal must emit exactly one down")
	down := downs[0]
	assert.Equal(t, rpc.SessionStateDown, down.state)
	assert.Equal(t, rpc.ReasonPeerRemoved, down.reason, "the down must name the removal, which is what frees a learned role")
	assert.Equal(t, peer.Settings().Address, down.peer.Address)
	assert.Equal(t, name, down.peer.Name)
	assert.Equal(t, group, down.peer.GroupName)
	assert.Greater(t, down.peer.SessionGeneration, before,
		"the down must carry a generation the removal issued, above every OPEN the removed peer validated")
}

// TestRemovePeerEmitsRemovalDown checks `delete bgp peer`.
//
// VALIDATES: RemovePeer emits one down with reason removed and a fresh
// generation, for the removed peer's address, name and group.
// PREVENTS: the API removal and the other removal paths drifting apart.
func TestRemovePeerEmitsRemovalDown(t *testing.T) {
	r := newRemovalTestReactor()
	rec := recordRemovals(r)
	addr := netip.MustParseAddr("192.0.2.20")
	settings := NewPeerSettings(addr, 65000, 65001, 0x01010101)
	settings.Name = "peer-api"
	settings.GroupName = "edge"
	peer := NewPeer(settings)
	peer.SetReactor(r)
	r.peers[peerKeyFromAddrPort(addr, DefaultBGPPort)] = peer
	before := peer.forwardGeneration.Load()

	require.NoError(t, r.RemovePeer(addr))

	requireOneRemovalDown(t, rec, peer, "peer-api", "edge", before)
}

// TestDynamicGroupRemovalEmitsRemovalDown checks a reload that drops the group
// a dynamic member was accepted from (planDynamicGroupsLocked, removeDynamicPeer).
//
// VALIDATES: the member's removal emits one down with reason removed and a
// fresh generation, under the member's generated name and group.
// PREVENTS: the member's learned role staying in bgp-role, which frees a role
// only on that down, after the member is gone.
func TestDynamicGroupRemovalEmitsRemovalDown(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.44")
	r, member := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
	rec := recordRemovals(r)
	settings := member.Settings()
	require.True(t, settings.IsDynamic)
	before := member.forwardGeneration.Load()

	require.NoError(t, (&reactorAPIAdapter{r: r}).reconcileDynamicGroupsJournaled(
		[]*DynamicGroupConfig{}, "test", &testJournal{}))

	_, exists := r.findPeerByAddr(addr)
	require.False(t, exists, "the member must be gone with its group")
	requireOneRemovalDown(t, rec, member, settings.Name, settings.GroupName, before)
}

// TestDynamicIdleCleanupEmitsRemovalDown checks the timer that removes a
// dynamic member that did not reconnect (scheduleDynamicPeerCleanup).
//
// VALIDATES: the idle removal emits one down with reason removed and a fresh
// generation, under the member's generated name and group.
// PREVENTS: a member whose OPEN validated, and whose session then failed,
// leaving its learned role behind when the timer removes it.
func TestDynamicIdleCleanupEmitsRemovalDown(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.45")
	r, member := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
	rec := recordRemovals(r)
	fake := sim.NewFakeClock(time.Now())
	r.SetClock(fake)
	member.state.Store(int32(PeerStateStopped))
	settings := member.Settings()
	before := member.forwardGeneration.Load()

	r.scheduleDynamicPeerCleanup(member)
	fake.Add(24 * time.Hour)

	_, exists := r.findPeerByAddr(addr)
	require.False(t, exists, "the idle member must be removed")
	requireOneRemovalDown(t, rec, member, settings.Name, settings.GroupName, before)
}

// TestReloadRemoveEmitsRemovalDown checks a reload whose configuration drops a
// configured peer (reconcilePeersJournaled, its remove step).
//
// VALIDATES: the dropped peer's removal emits one down with reason removed and
// a fresh generation, under its name and group.
// PREVENTS: the reload's removal telling the plugins less than `delete bgp
// peer` does.
func TestReloadRemoveEmitsRemovalDown(t *testing.T) {
	r := New(&Config{})
	rec := recordRemovals(r)
	adapter := &reactorAPIAdapter{r: r}
	dropped := NewPeerSettings(mustParseAddr("192.0.2.21"), 65000, 65001, 0)
	dropped.Name = "peerD"
	dropped.GroupName = "edge"
	require.NoError(t, r.AddPeer(dropped))
	peer := r.peers[dropped.PeerKey()]
	require.NotNil(t, peer)
	before := peer.forwardGeneration.Load()

	require.NoError(t, adapter.reconcilePeersJournaled(nil, Globals{}, dynamicGroupsNotRead, "test", &testJournal{}))

	requireOneRemovalDown(t, rec, peer, "peerD", "edge", before)
}

// TestReloadReplacedDynamicMemberEmitsRemovalDown checks a reload that
// replaces a dynamic member with a configured peer at its address.
//
// VALIDATES: the member's removal emits one down with reason removed and a
// fresh generation, under the member's own name and group.
// PREVENTS: the member's learned role staying in bgp-role under a name key no
// peer will ever use again: the prune keeps a member's key while its group
// stays dynamic, so this down is the only thing that frees it.
func TestReloadReplacedDynamicMemberEmitsRemovalDown(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.46")
	configured := NewPeerSettings(addr, 65000, 65010, 0x01020304)
	configured.Connection = ConnectionPassive
	configured.Name = "ix-peer-3"
	r, member := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, []*PeerSettings{configured})
	rec := recordRemovals(r)
	settings := member.Settings()
	require.True(t, settings.IsDynamic)
	before := member.forwardGeneration.Load()

	require.NoError(t, (&reactorAPIAdapter{r: r}).Reload())

	requireOneRemovalDown(t, rec, member, settings.Name, settings.GroupName, before)
}

// TestReloadAddUndoEmitsRemovalDown checks the undo of a reload's add.
//
// VALIDATES: the forward add emits no removal down, and rolling it back emits
// one with reason removed and a fresh generation for the added peer.
// PREVENTS: a learned role an OPEN stored for the added peer surviving the
// rollback: the prune runs only at config-committed, and a rollback never
// commits.
func TestReloadAddUndoEmitsRemovalDown(t *testing.T) {
	r := New(&Config{})
	rec := recordRemovals(r)
	adapter := &reactorAPIAdapter{r: r}
	added := NewPeerSettings(mustParseAddr("192.0.2.22"), 65000, 65003, 0)
	added.Name = "peerE"
	added.GroupName = "core"

	j := &testJournal{}
	require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{added}, Globals{}, dynamicGroupsNotRead, "test", j))
	require.Empty(t, rec.seen(), "an add emits no removal down")
	peer := r.peers[added.PeerKey()]
	require.NotNil(t, peer)
	before := peer.forwardGeneration.Load()

	j.rollback()

	requireOneRemovalDown(t, rec, peer, "peerE", "core", before)
}
