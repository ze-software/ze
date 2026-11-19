// Design: docs/architecture/api/process-protocol.md -- bound server lifetime and final flush.
package reactor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"
)

func boundRelaySecondTarget(t *testing.T, f *boundRelayWireFixture, address string, port uint16) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newGroupUpdatesPeer(t, address, "absent")
	peer.settings.Port = port
	peer.settings.ProcessBindings = f.target.peer.settings.ProcessBindings
	peer.settings.ExportPolicy = f.target.peer.settings.ExportPolicy
	peer.sendCtx.Store(f.target.peer.sendCtx.Load())
	peer.sendCtxID = f.target.peer.sendCtxID
	peer.session.sendCtxID = peer.sendCtxID
	peer.session.initialReplay.Store(f.target.peer.session.initialReplay.Load() + 1)
	peer.establishedSession.Store(peer.session)
	peer.reactor = f.target.api.r
	peer.session.egressFinalAccepts = f.target.peer.session.egressFinalAccepts
	peer.refreshForwardFacts()
	f.target.api.r.peers[peer.settings.PeerKey()] = peer
	f.target.api.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: peer.settings.PeerKey()}, 4096)
	return peer, conn
}

func TestBoundUPGenerationRemainsOnCapturedSession(t *testing.T) {
	registerSyncOwedFenceOwner(t)
	peer, original := establishSettledPeer(t, "192.0.2.9", true)
	published := peer.forwardGeneration.Load()
	require.Positive(t, published)
	ordinary := establishedPeerInfo(peer)
	require.Equal(t, published, ordinary.SessionGeneration)
	require.Equal(t, original.connectionID, ordinary.ConnectionID)
	replacement := NewSession(peer.settings)
	peer.mu.Lock()
	peer.session = replacement
	peer.mu.Unlock()
	peer.establishedSession.Store(replacement)
	peer.setState(PeerStateConnecting)
	peer.enterEstablished(replacement)
	require.NotEqual(t, published, peer.forwardGeneration.Load())
	attached := boundPeerInfo(peer, original)
	require.Equal(t, published, attached.SessionGeneration, "delayed attached UP keeps original published generation")
	require.Equal(t, original.connectionID, attached.ConnectionID)
	require.Equal(t, peer.forwardGeneration.Load(), boundPeerInfo(peer, replacement).SessionGeneration)
}

func TestFastPathSkipCapturesSameAddressOriginalTargets(t *testing.T) {
	contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	encoding := bgpctx.Registry.Get(contextID)
	source := makeRSPeer(t, "10.0.0.1", 65001, encoding, contextID)
	first := makeRSPeer(t, "10.0.0.2", 65002, encoding, contextID)
	second := makeRSPeer(t, "10.0.0.2", 65003, encoding, contextID)
	second.settings.Port = 2179
	sourceSessionFixture(t, first)
	sourceSessionFixture(t, second)
	first.session.initialReplay.Store(9001)
	second.session.initialReplay.Store(9002)
	first.settings.ExportPolicy = mustPolicy(t, frefs("bgp-rs:missing-original-chain"), first.settings.ExportPolicy.Final())
	second.settings.ExportPolicy = first.settings.ExportPolicy
	first.refreshForwardFacts()
	second.refreshForwardFacts()
	reactor := &Reactor{peers: map[netip.AddrPort]*Peer{source.settings.PeerKey(): source, first.settings.PeerKey(): first, second.settings.PeerKey(): second}}
	wire := wireu.NewWireUpdate([]byte{0, 0, 0, 0}, contextID)
	update := &ReceivedUpdate{WireUpdate: wire, SourcePeerIP: source.settings.Address}
	skipped, _, _ := reactorForwardRSSection(reactor, update, wire, nil, 50, source.settings.Address, source)
	require.Len(t, skipped, 2)
	want := map[uint64]uint64{first.sourceOwner: 9001, second.sourceOwner: 9002}
	for _, target := range skipped {
		require.Equal(t, want[target.DestinationOwner], target.DestinationSession)
	}
	first.session.initialReplay.Add(10)
	second.session.initialReplay.Add(10)
	receipt := types.UpdateReceipt{Peer: source.settings.Address, PeerOwner: source.sourceOwner, MessageID: 50, SourcePolicyEpoch: 1, SourceReceipt: 3, ReactorForwarded: true, FastPathSkipped: skipped}
	raw, err := json.Marshal(receipt)
	require.NoError(t, err)
	parsed, err := types.ParseUpdateReceipt(raw)
	require.NoError(t, err)
	require.NoError(t, parsed.Validate(source.addrString, source.sourceOwner, 50, 1, 3))
	require.Equal(t, skipped, parsed.FastPathSkipped)
	for _, target := range parsed.FastPathSkipped {
		require.Equal(t, want[target.DestinationOwner], target.DestinationSession, "transport never borrows later current tokens")
	}
}

// One sequence covers original useful output, an actual held socket, and the
// real authenticated connection's source ownership. Every held writer is joined.
func TestBoundRelayServerTransportsJoinHeldLiveLifetime(t *testing.T) {
	for _, socket := range []bool{false, true} {
		for _, ending := range []string{"upgrade-down", "grant-cut"} {
			t.Run(map[bool]string{false: "native/", true: "framed/"}[socket]+ending, func(t *testing.T) {
				f := newBoundRelayWireFixture(t, false)
				work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
				defer cancel()
				_, client, proc := selectedRefreshCleanupSDK(t, f.target, socket, work)
				actualOwner, err := proc.SelectedOwner()
				require.NoError(t, err)
				input := rpc.RelayStoredRouteInput{Destination: f.target.peer.settings.Address.String(), DestinationOwner: f.target.peer.sourceOwner, DestinationSession: f.target.peer.session.initialReplay.Load(), Routes: []rpc.StoredRoute{f.route}}
				require.NoError(t, client.RelayStoredRoute(work, input))
				require.Len(t, updateBodies(t, f.target.conn.written()), 1, "useful output crosses actual server before the held case")
				sourceReceipts.mu.Lock()
				require.Empty(t, sourceReceipts.entries[f.route.SourceReceipt].leases)
				sourceReceipts.mu.Unlock()
				writer, remote := net.Pipe()
				release := make(chan struct{})
				var releaseOnce sync.Once
				releaseHeld := func() { releaseOnce.Do(func() { close(release) }) }
				t.Cleanup(releaseHeld)
				held := &selectedRefreshHeldConn{Conn: writer, writing: make(chan struct{}), interrupted: make(chan struct{}), release: release}
				t.Cleanup(func() { _ = writer.Close(); _ = remote.Close() })
				f.target.peer.session.conn = held
				f.target.peer.session.bufWriter = bufio.NewWriterSize(held, 4096)
				result := make(chan error, 1)
				input.Routes[0].MsgID++
				go func() { result <- client.RelayStoredRoute(work, input) }()
				select {
				case <-held.writing:
				case err := <-result:
					t.Fatalf("bound server returned before actual flush: %v", err)
				case <-work.Done():
					t.Fatal("bound relay did not reach held socket")
				}
				cleanup := make(chan error, 1)
				if ending == "upgrade-down" {
					leaseInput := rpc.SourceLeaseInput{PeerOwner: f.source.sourceOwner, SourceReceipt: f.route.SourceReceipt, Action: rpc.SourceLeaseAcquire}
					_, err := client.SourceLease(work, leaseInput)
					require.NoError(t, err)
					leaseInput.Action = rpc.SourceLeaseRetain
					_, err = client.SourceLease(work, leaseInput)
					require.NoError(t, err)
					f.source.setState(PeerStateStopped)
					cleanup <- nil
				} else {
					f.source.mu.Lock()
					before := f.source.settings.ProcessBindings
					f.source.settings.ProcessBindings = nil
					f.source.mu.Unlock()
					go func() { cleanup <- f.source.revokeRemovedSourceGrants(f.target.api.r, before, nil) }()
				}
				select {
				case <-held.interrupted:
				case <-work.Done():
					t.Fatal("source lifetime did not cancel the actual held socket")
				}
				select {
				case err := <-result:
					t.Fatalf("server acknowledged before actual writer joined: %v", err)
				default:
				}
				if ending == "grant-cut" {
					select {
					case err := <-cleanup:
						t.Fatalf("source cleanup acknowledged while actual writer held: %v", err)
					default:
					}
				}
				releaseHeld()
				select {
				case err := <-result:
					require.ErrorIs(t, err, context.Canceled)
					_, joined := errors.AsType[*rpc.CachedWriteJoinedError](err)
					require.True(t, joined, "the actual server returns exact final target settlement")
				case <-work.Done():
					t.Fatal("bound server failed to join released socket")
				}
				select {
				case err := <-cleanup:
					require.NoError(t, err)
				case <-work.Done():
					t.Fatal("source cleanup failed to join released writer")
				}
				if ending == "upgrade-down" {
					lease, err := sourceReceipts.ownedLease(f.route.SourceReceipt, actualOwner)
					require.NoError(t, err)
					retained, err := lease.begin(f.route.SourcePolicyEpoch)
					require.NoError(t, err)
					require.NoError(t, retained.current(), "only independently admitted retained work remains authorized")
					retained.complete()
					leaseInput := rpc.SourceLeaseInput{PeerOwner: f.source.sourceOwner, SourceReceipt: f.route.SourceReceipt, Action: rpc.SourceLeaseRelease}
					_, err = client.SourceLease(work, leaseInput)
					require.NoError(t, err)
				}
				sourceReceipts.mu.Lock()
				require.Empty(t, sourceReceipts.entries[f.route.SourceReceipt].leases)
				sourceReceipts.mu.Unlock()
			})
		}
	}
}

func TestBoundReadyCreditsOnlyOriginalOwnerAndSession(t *testing.T) {
	registerSyncOwedFenceOwner(t)
	first, session := establishSettledPeer(t, "192.0.2.8", true)
	sibling, _ := establishSettledPeer(t, "192.0.2.8", true)
	sibling.settings.Port = 2179
	reactor := &Reactor{peers: map[netip.AddrPort]*Peer{first.settings.PeerKey(): first, sibling.settings.PeerKey(): sibling}}
	sender, err := plugin.ProcessSender(syncOwedFenceOwner).WithDestination(first.sourceOwner, session.initialReplay.Load())
	require.NoError(t, err)
	require.NoError(t, reactor.SignalPeerAPIReady(t.Context(), first.addrString, sender, session.initialReplay.Load()))
	require.False(t, first.initialUpdateOwed.Load())
	require.True(t, sibling.initialUpdateOwed.Load(), "same address does not credit a sibling")
	canceled, stop := context.WithCancel(t.Context())
	stop()
	require.ErrorIs(t, reactor.SignalPeerAPIReady(canceled, first.addrString, sender, session.initialReplay.Load()), context.Canceled)
	session.initialReplay.Add(1)
	require.Error(t, reactor.SignalPeerAPIReady(t.Context(), first.addrString, sender, session.initialReplay.Load()))
	require.True(t, sibling.initialUpdateOwed.Load())
}
