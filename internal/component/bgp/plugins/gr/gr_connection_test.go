// Design: docs/guide/graceful-restart.md -- original physical connection.
package gr

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// grConnectionOpen uses the real OPEN handlers on both receiving rails. The
// helper changes no capability map itself: wire decoding decides retention.
func grConnectionOpen(t *testing.T, gp *grPlugin, source grSource, typed bool) {
	t.Helper()
	caps := [][]byte{buildGRCapTLV(0, 3600, [][4]byte{{0, 1, 1, 0x80}}), buildLLGRCapTLV([][7]byte{{0, 1, 1, 0x80, 0, 0x0e, 0x10}})}
	for _, direction := range []rpc.MessageDirection{rpc.DirectionSent, rpc.DirectionReceived} {
		if typed {
			require.NoError(t, gp.handleStructuredEvent(&rpc.StructuredEvent{EventType: rpc.EventKindOpen, PeerAddress: source.peer(), PeerOwner: source.owner, ConnectionID: source.connectionID, Direction: direction, RawMessage: &bgptypes.RawMessage{RawBytes: buildOpenBody(buildCapabilityParam(caps...))}}))
			continue
		}
		values := make([]map[string]any, 0, len(caps))
		for _, cap := range caps {
			values = append(values, map[string]any{"code": cap[0], "value": hex.EncodeToString(cap[2:])})
		}
		grConnectionJSON(t, gp, source, map[string]any{"message": map[string]any{"type": "open", "direction": direction.String()}, "open": map[string]any{"capabilities": values}})
	}
}

func grConnectionJSON(t *testing.T, gp *grPlugin, source grSource, payload map[string]any) {
	t.Helper()
	payload["peer-owner"] = source.owner
	payload["connection-id"] = strconv.FormatUint(source.connectionID, 10)
	payload["peer"] = map[string]any{"remote": map[string]any{"address": source.peer()}}
	body, err := json.Marshal(map[string]any{"type": "bgp", "bgp": payload})
	require.NoError(t, err)
	require.NoError(t, gp.handleEvent(string(body)))
}

func grConnectionState(t *testing.T, gp *grPlugin, source grSource, typed bool, state rpc.SessionState) {
	t.Helper()
	if typed {
		require.NoError(t, gp.handleStructuredEvent(&rpc.StructuredEvent{EventType: rpc.EventKindState, PeerAddress: source.peer(), PeerOwner: source.owner, ConnectionID: source.connectionID, State: state, Reason: rpc.ReasonTCPFailure}))
		return
	}
	grConnectionJSON(t, gp, source, map[string]any{"message": map[string]any{"type": "state"}, "state": state.String(), "reason": rpc.ReasonTCPFailure})
}

// Hold the real release dispatch, establish a useful successor and same-address
// sibling through OPEN/UP, then let the old local completion return. Neither
// successor loses its negotiated capability; the new DOWN still retains IPv4.
// MUTATION: prune capabilities by owner after RPC instead of matching connection.
func TestGRReleaseCompletionKeepsSuccessorCapability(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "typed"}[typed], func(t *testing.T) {
			old, err := newGRSource("192.0.2.242", math.MaxUint64-1, math.MaxUint64-2)
			require.NoError(t, err)
			fresh, err := newGRSource(old.peer(), old.owner, math.MaxUint64-1)
			require.NoError(t, err)
			sibling, err := newGRSource(old.peer(), math.MaxUint64, math.MaxUint64)
			require.NoError(t, err)
			gp := &grPlugin{peerCaps: make(map[uint64]*grPeerCap), peerLLGRCaps: make(map[uint64]*llgrPeerCap)}
			gp.wireStateCallbacks()
			t.Cleanup(func() { gp.state.removePeer(old.grPeer); gp.state.removePeer(sibling.grPeer) })
			gp.emitHook = func(string) {}
			entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			var mu sync.Mutex
			var commands [][]string
			gp.dispatchHook = func(command string, args ...string) {
				mu.Lock()
				commands = append(commands, append([]string{command}, args...))
				mu.Unlock()
				if command == "request bgp rib source release-routes" {
					close(entered)
					<-release
				}
			}
			grConnectionOpen(t, gp, old, typed)
			// One test helper lifecycle; cleanup releases and joins it even on failure.
			go func() { defer close(joined); gp.releaseRoutes(old) }()

			t.Cleanup(func() { once.Do(func() { close(release) }); <-joined })
			select {
			case <-entered:
			case <-t.Context().Done():
				t.Fatal("old release did not reach its RPC")
			}
			grConnectionOpen(t, gp, fresh, typed)
			grConnectionState(t, gp, fresh, typed, rpc.SessionStateUp)
			grConnectionOpen(t, gp, sibling, typed)
			grConnectionState(t, gp, sibling, typed, rpc.SessionStateUp)
			once.Do(func() { close(release) })
			select {
			case <-joined:
			case <-t.Context().Done():
				t.Fatal("old release completion did not join")
			}
			gp.mu.Lock()
			freshCap := gp.peerCaps[fresh.owner]
			siblingCap := gp.peerCaps[sibling.owner]
			honored := gp.exchangedLLGRLocked(fresh)
			gp.mu.Unlock()
			require.NotNil(t, freshCap)
			require.Equal(t, fresh.connectionID, freshCap.connectionID)
			require.NotNil(t, siblingCap)
			require.Equal(t, sibling.connectionID, siblingCap.connectionID)
			require.NotNil(t, honored)
			require.Equal(t, fresh.connectionID, honored.connectionID)
			grConnectionState(t, gp, fresh, typed, rpc.SessionStateDown)
			grConnectionState(t, gp, sibling, typed, rpc.SessionStateDown)
			require.Equal(t, []family.Family{family.IPv4Unicast}, gp.state.retainedFamilies(fresh))
			require.Equal(t, []family.Family{family.IPv4Unicast}, gp.state.retainedFamilies(sibling))
			mu.Lock()
			defer mu.Unlock()
			require.Contains(t, commands, []string{"request bgp rib source mark-stale", "owner", strconv.FormatUint(fresh.owner, 10), "connection", strconv.FormatUint(fresh.connectionID, 10), "restart-time", "3600"})
			require.Contains(t, commands, []string{"request bgp rib source mark-stale", "owner", strconv.FormatUint(sibling.owner, 10), "connection", strconv.FormatUint(sibling.connectionID, 10), "restart-time", "3600"})
		})
	}
}

// A reconnect retains old physical-source rows until its own EOR. The obsolete
// connection's EOR is inert; the successor's EOR purges the original conn1,
// after which conn2 still has its own useful retention on the next DOWN.
// MUTATION: dispatch EOR purge with the current connection instead of the cycle.
func TestGRSuccessorEORTargetsOriginalConnection(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "typed"}[typed], func(t *testing.T) {
			old, err := newGRSource("192.0.2.243", 1341, 2341)
			require.NoError(t, err)
			fresh, err := newGRSource(old.peer(), old.owner, 2342)
			require.NoError(t, err)
			gp := &grPlugin{peerCaps: make(map[uint64]*grPeerCap), peerLLGRCaps: make(map[uint64]*llgrPeerCap)}
			gp.wireStateCallbacks()
			t.Cleanup(func() { gp.state.removePeer(old.grPeer) })
			gp.emitHook = func(string) {}
			var commands [][]string
			gp.dispatchHook = func(command string, args ...string) { commands = append(commands, append([]string{command}, args...)) }
			grConnectionOpen(t, gp, old, typed)
			grConnectionState(t, gp, old, typed, rpc.SessionStateUp)
			grConnectionState(t, gp, old, typed, rpc.SessionStateDown)
			grConnectionOpen(t, gp, fresh, typed)
			grConnectionState(t, gp, fresh, typed, rpc.SessionStateUp)
			require.Equal(t, []family.Family{family.IPv4Unicast}, gp.state.retainedFamilies(old))
			before := len(commands)
			eor := func(source grSource) {
				grConnectionJSON(t, gp, source, map[string]any{"message": map[string]any{"type": "eor"}, "eor": map[string]any{"family": "ipv4/unicast"}})
			}
			eor(old)
			require.Len(t, commands, before, "old EOR cannot complete the reconnect")
			eor(fresh)
			require.Len(t, commands, before+1)
			require.Equal(t, []string{"request bgp rib source purge-stale", "owner", "1341", "connection", "2341", "family", "ipv4/unicast"}, commands[len(commands)-1])
			grConnectionState(t, gp, fresh, typed, rpc.SessionStateDown)
			require.Equal(t, []family.Family{family.IPv4Unicast}, gp.state.retainedFamilies(fresh))
		})
	}
}

// The actual timer callback enters LLGR and blocks at its first RIB RPC while
// the peer reconnects. The LLGR period has begun and its timer keeps running,
// so on return the entry sends the rest of its sequence, in order and to the
// original connection: the LLGR_STALE attachment, the level-2 marking and the
// readvertisement (RFC 9494 Section 4.2). The successor still starts a useful
// GR cycle.
// MUTATION: answer llgrFamilyActive false once a successor has re-established.
func TestGRReconnectRevokesHeldLLGREntryTail(t *testing.T) {
	old, err := newGRSource("192.0.2.246", 1346, 2346)
	require.NoError(t, err)
	fresh, err := newGRSource(old.peer(), old.owner, 2347)
	require.NoError(t, err)
	gp := &grPlugin{peerCaps: make(map[uint64]*grPeerCap), peerLLGRCaps: make(map[uint64]*llgrPeerCap)}
	gp.wireStateCallbacks()
	t.Cleanup(func() { gp.state.removePeer(old.grPeer) })
	gp.emitHook = func(string) {}
	entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	var holdEntryOnce sync.Once
	var mu sync.Mutex
	var commands [][]string
	gp.dispatchHook = func(command string, args ...string) {
		mu.Lock()
		commands = append(commands, append([]string{command}, args...))
		mu.Unlock()
		if command == "request bgp rib source delete-with-community" {
			// Only the first one is the old connection's LLGR entry, which this
			// test holds. The successor's own DOWN sends the command again for
			// the routes it retains (RFC 9494 Section 4.2), and those pass.
			holdEntryOnce.Do(func() {
				close(entered)
				<-release
			})
		}
	}
	grConnectionOpen(t, gp, old, true)
	grConnectionState(t, gp, old, true, rpc.SessionStateUp)
	grConnectionState(t, gp, old, true, rpc.SessionStateDown)
	gp.state.mu.Lock()
	captured := gp.state.peers[old.owner]
	gp.state.mu.Unlock()
	require.NotNil(t, captured)
	// Stop the long real timer before driving its exact captured callback, so
	// this deterministic test does not retain an unjoined timer after return.
	require.NotNil(t, captured.restartTimer)
	captured.restartTimer.Stop()
	// One captured timer lifecycle; cleanup always releases and joins it.
	go func() { defer close(joined); gp.state.handleTimerExpired(old, captured) }()

	t.Cleanup(func() { once.Do(func() { close(release) }); <-joined })
	select {
	case <-entered:
	case <-t.Context().Done():
		t.Fatal("LLGR entry did not reach its first RPC")
	}
	grConnectionOpen(t, gp, fresh, true)
	grConnectionState(t, gp, fresh, true, rpc.SessionStateUp)
	mu.Lock()
	before := len(commands)
	mu.Unlock()
	once.Do(func() { close(release) })
	select {
	case <-joined:
	case <-t.Context().Done():
		t.Fatal("LLGR entry tail did not join")
	}
	mu.Lock()
	require.Equal(t, [][]string{
		{"request bgp rib source attach-community", "owner", "1346", "connection", "2346", "family", "ipv4/unicast", "community", "ffff0006"},
		{"request bgp rib source mark-stale", "owner", "1346", "connection", "2346", "restart-time", "0", "stale-level", "2", "family", "ipv4/unicast"},
		{"clear bgp rib out-excluding-owner", "owner", "1346", "family", "ipv4/unicast"},
	}, commands[before:], "a reconnect does not stop an LLGR entry whose period has begun")
	mu.Unlock()
	require.Equal(t, []family.Family{family.IPv4Unicast}, gp.state.retainedFamilies(old))
	grConnectionState(t, gp, fresh, true, rpc.SessionStateDown)
	require.Equal(t, []family.Family{family.IPv4Unicast}, gp.state.retainedFamilies(fresh))
}

// A DOWN completion can wait in retain/mark RPCs while a reconnect cancels its
// immediate LLGR entry. Returning from those RPCs cannot restart that canceled
// cycle, and the valid retained source remains available for successor EOR.
// MUTATION: admit the completion solely by state pointer after reconnect.
func TestGRReconnectRevokesDeferredImmediateEntry(t *testing.T) {
	old, err := newGRSource("192.0.2.247", 1347, 2348)
	require.NoError(t, err)
	fresh, err := newGRSource(old.peer(), old.owner, 2349)
	require.NoError(t, err)
	manager := newGRStateManager(nil)
	t.Cleanup(func() { manager.removePeer(old.grPeer) })
	gr := &grPeerCap{connectionID: old.connectionID, Families: []grCapFamily{{Family: family.IPv4Unicast, ForwardState: true}}}
	llgr := &llgrPeerCap{connectionID: old.connectionID, Families: []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 3600}, {Family: family.IPv6Unicast, ForwardState: true, LLST: 3600}}}
	active, complete := manager.onSessionDownDeferred(old, gr, llgr, false)
	require.True(t, active)
	require.NotNil(t, complete)
	successorGR := &grPeerCap{connectionID: fresh.connectionID, Families: []grCapFamily{{Family: family.IPv4Unicast, ForwardState: true}, {Family: family.IPv6Unicast, ForwardState: true}}}
	successorLLGR := &llgrPeerCap{connectionID: fresh.connectionID, Families: llgr.Families}
	original, purged, _ := manager.onSessionReestablished(fresh, successorGR, successorLLGR)
	require.Equal(t, old, original)
	require.Empty(t, purged)
	entered := false
	manager.onLLGREnter = func(grSource, family.Family, uint32) { entered = true }
	complete()
	require.False(t, entered, "the already-canceled immediate LLGR completion cannot reenter")
	manager.mu.Lock()
	require.Empty(t, manager.peers[old.owner].llgrFamilies)
	manager.mu.Unlock()
	original, purge := manager.onEORReceived(fresh, family.IPv6Unicast)
	require.True(t, purge)
	require.Equal(t, old, original)
	require.Equal(t, []family.Family{family.IPv4Unicast}, manager.retainedFamilies(old))
}

// Peer removal ends the actual original owner even when no physical Session
// exists. Both event rails must remove its old retained state and keep the
// useful same-address sibling's capability and retained family intact.
func TestGRNeverConnectedRemovalEndsOnlyActualOwner(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "typed"}[typed], func(t *testing.T) {
			old, err := newGRSource("192.0.2.248", 1348, 2350)
			require.NoError(t, err)
			sibling, err := newGRSource(old.peer(), 1349, 2351)
			require.NoError(t, err)
			gp := &grPlugin{peerCaps: make(map[uint64]*grPeerCap), peerLLGRCaps: make(map[uint64]*llgrPeerCap)}
			gp.wireStateCallbacks()
			t.Cleanup(func() { gp.state.removePeer(old.grPeer); gp.state.removePeer(sibling.grPeer) })
			gp.dispatchHook = func(string, ...string) {}
			gp.emitHook = func(string) {}
			for _, source := range []grSource{old, sibling} {
				grConnectionOpen(t, gp, source, typed)
				grConnectionState(t, gp, source, typed, rpc.SessionStateDown)
			}
			if typed {
				require.NoError(t, gp.handleStructuredEvent(&rpc.StructuredEvent{EventType: rpc.EventKindState, PeerAddress: old.peer(), PeerOwner: old.owner, State: rpc.SessionStateDown, Reason: rpc.ReasonPeerRemoved}))
			} else {
				body, err := json.Marshal(map[string]any{"type": "bgp", "bgp": map[string]any{"peer-owner": old.owner, "connection-id": "0", "peer": map[string]any{"remote": map[string]any{"address": old.peer()}}, "message": map[string]any{"type": "state"}, "state": "down", "reason": rpc.ReasonPeerRemoved}})
				require.NoError(t, err)
				require.NoError(t, gp.handleEvent(string(body)))
			}
			require.False(t, gp.state.peerActive(old))
			require.NotContains(t, gp.peerCaps, old.owner)
			require.True(t, gp.state.peerActive(sibling))
			require.NotNil(t, gp.peerCaps[sibling.owner])
			require.Equal(t, []family.Family{family.IPv4Unicast}, gp.state.retainedFamilies(sibling))
		})
	}
}
