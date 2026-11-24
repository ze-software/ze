// Design: docs/architecture/plugin/rib-storage-design.md -- source-DOWN owner-release ordering.
// Related: rfc8654_fatal_length_cleanup_test.go -- fatal cleanup and valid-length retention controls.
// Related: rfc8654_rib_recovery_test.go -- alternate-best re-election and treat-as-withdraw controls.
// RFC 8654 Sections 5 and 6 -- see rfc/short/rfc8654.md.
// RFC 4271 Sections 6 and 6.1 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectedOrderGateAPI wraps reactorAPIAdapter to pause one recipient's
// post-fatal SelectRoutes call right before outbound admission runs.
type selectedOrderGateAPI struct {
	*reactorAPIAdapter
	armed    atomic.Bool
	dest     netip.Addr
	entered  chan struct{}
	release  chan struct{}
	once     sync.Once
	released sync.Once
}

func (g *selectedOrderGateAPI) SelectRoutes(ctx context.Context, batch *plugin.SelectedBatch, owner plugin.SelectedOwner) (*rpc.SelectRoutesOutput, error) {
	if g.armed.Load() && batch != nil && batch.Destination() == g.dest {
		g.once.Do(func() { close(g.entered) })
		select {
		case <-g.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return g.reactorAPIAdapter.SelectRoutes(ctx, batch, owner)
}

func (g *selectedOrderGateAPI) unblock() {
	g.released.Do(func() { close(g.release) })
}

func sourceReceiptRecordHeld(id uint64) (exists bool, leaseCount int) {
	sourceReceipts.mu.Lock()
	defer sourceReceipts.mu.Unlock()
	record := sourceReceipts.entries[id]
	if record == nil {
		return false, 0
	}
	return true, len(record.leases)
}

func fatalOrderAdjOutState(peer, source *Peer) (ownedRoutes, owedRecords int, openDone bool) {
	table := &peer.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	for _, entry := range table.routes {
		if entry.owner.source == source {
			ownedRoutes++
		}
	}
	owedRecords = len(table.owed)
	for _, record := range table.owed {
		select {
		case <-record.done:
		default:
			openDone = true
		}
	}
	return ownedRoutes, owedRecords, openDone
}

// TestRFC8654FatalLengthHoldsRouteOwnerUntilRecoveryAdmission pauses recipient
// outbound admission right before fwdBatchHandler executes admitRecovery's
// dispatched batch. While paused after fatal Length 4097 rejection, the
// recipient's Adj-RIB-Out route owner, open owed record, and bgp-rs's engine
// source lease MUST all remain alive; only after unblocking outbound admission
// does the withdrawal or alternate replacement reach recipient TCP and the last
// source owner lease release.
//
// VALIDATES: RFC 4271 Section 6 owner-release ordering across admitRecovery on
// both withdraw-only and alternate-best fatal extended-message teardowns.
// PREVENTS: releasing the last invalid-route/advertisement owner or source
// lease before outbound withdrawal or alternate-best admission completes.
// MUTATION: return from recoverNLRIBatch or admitRecovery without waiting for
// outbound admission to settle, or release the source lease before
// sendDepartureRoutes in bgp-rs settleDepartureLocked: the while-paused
// owner/lease assertions fail immediately.
//
// RFC 8654 Section 5: "Similarly, any speaker that treats an improper BGP Extended
// Message as a fatal error MUST follow the error-handling procedures of [RFC4271]."
// RFC 4271 Section 6: "Before the invalid routes are deleted from the system, it
// advertises, to its peers, either withdraws for the routes marked as invalid, or
// the new best routes before the invalid routes are deleted from the system."
// RFC requirement: RFC8654-5-4 positive -- on fatal Length 4097 teardown in route-server mode, recipient Adj-RIB-Out ownership, open recovery owed records and the final source lease stay held until admitRecovery completes outbound admission to recipient TCP, and release only afterward.
func TestRFC8654FatalLengthHoldsRouteOwnerUntilRecoveryAdmission(t *testing.T) {
	for _, withAlternate := range []bool{false, true} {
		name := "withdraw-no-survivor"
		if withAlternate {
			name = "replace-with-alternate"
		}
		t.Run(name, func(t *testing.T) {
			var armed atomic.Bool
			entered := make(chan struct{})
			release := make(chan struct{})
			var enterOnce, releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }

			peers := extendedRecoveryPeersWithGate(t, withAlternate, func(key fwdKey, items []fwdItem) {
				if armed.Load() {
					for i := range items {
						if items[i].recovery != nil {
							enterOnce.Do(func() { close(entered) })
							<-release
							break
						}
					}
				}
				fwdBatchHandler(key, items)
			})
			// Register unblock AFTER extendedRecoveryPeersWithGate so LIFO
			// cleanup releases any paused forward worker before stopAndWait.
			t.Cleanup(unblock)
			source, recipient := peers[0], peers[1]
			var alternate *lowLivePeer
			if withAlternate {
				alternate = peers[2]
			}

			sourceSession := source.peer.currentSession()
			require.NotNil(t, sourceSession)
			liveReceipt := sourceSession.sourceReceipt.Load()
			require.NotNil(t, liveReceipt)
			receiptID := liveReceipt.ID()
			require.NotZero(t, receiptID)

			preferred := fatalLengthAnnouncement()
			source.send(t, message.PackTo(preferred, nil))
			lowEventually(t, func() bool {
				announced, _ := fatalLengthRecipientRoutes(t, recipient)
				return announced == 3
			}, "initial routes advertised to recipient")

			var backup *message.Update
			if withAlternate {
				alternatePath := []byte{2, 2, 0, 0, 0xfd, 0xec, 0, 0, 0xfd, 0xec}
				backup = &message.Update{
					PathAttributes: append(append([]byte{0x40, 1, 1, 0, 0x40, 2, 10}, alternatePath...), 0x40, 3, 4, 192, 0, 2, 3),
					NLRI:           []byte{24, 203, 0, 114},
				}
				alternate.send(t, message.PackTo(backup, nil))
				lowEventually(t, func() bool {
					return bytes.Equal(extendedRecoveryStoredAttributes("192.0.2.3", []byte{203, 0, 114}), backup.PathAttributes)
				}, "alternate path stored in Adj-RIB-In")
				lowEventually(t, func() bool {
					table := &recipient.peer.adjOut
					table.mu.Lock()
					defer table.mu.Unlock()
					if len(table.owed) != 0 {
						return false
					}
					for _, entry := range table.routes {
						if entry.contested && entry.owner.source == source.peer {
							return true
						}
					}
					return false
				}, "preferred source still owns contested prefix")
			}

			recipient.mu.Lock()
			beforeFatal := len(recipient.frames)
			recipient.mu.Unlock()

			// Arm the recovery gate only after initial setup and alternate election settled.
			armed.Store(true)

			events := &peerDownRecorder{established: make(chan *Peer, 4), closed: make(chan *Peer, 4)}
			source.peer.reactor.addPeerObserver(events)

			// RFC 8654 Section 6: 4097-octet UPDATE header without local capability 6.
			header := append(bytes.Repeat([]byte{0xff}, message.MarkerLen), 0x10, 0x01, byte(msgtype.TypeUPDATE))
			source.send(t, header)

			// Wait for fatal NOTIFICATION 1/2 Data 1001, EOF, and pause at
			// admitRecovery's dispatched outbound admission boundary.
			requireEstablishedReleased(t, &mpLinkNeighbor{peer: source.peer, events: events}, sourceSession)
			lowEventually(t, func() bool { return sourceSession.State() == fsm.StateIdle }, "fatal session Idle")
			select {
			case <-source.stopped:
			case <-time.After(5 * time.Second):
				t.Fatal("fatal-length TCP connection never closed")
			}
			require.NoError(t, source.remote.SetReadDeadline(time.Now().Add(time.Second)))
			var octet [1]byte
			n, err := source.remote.Read(octet[:])
			require.Zero(t, n)
			require.ErrorIs(t, err, io.EOF)
			wantNotify := append(bytes.Repeat([]byte{0xff}, message.MarkerLen), 0, 23, 3, 1, 2, 0x10, 0x01)
			source.mu.Lock()
			var notifications [][]byte
			for _, frame := range source.frames {
				if frame[18] == byte(msgtype.TypeNOTIFICATION) {
					notifications = append(notifications, frame)
				}
			}
			source.mu.Unlock()
			require.Equal(t, [][]byte{wantNotify}, notifications, "exact Bad Message Length NOTIFICATION")

			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("admitRecovery never dispatched to the destination forward worker")
			}

			// WHILE PAUSED before outbound admission executes: the invalid
			// route's Adj-RIB-Out owner, open owed record, and bgp-rs's final
			// source lease MUST all still be held.
			ownedRoutes, owedRecords, openDone := fatalOrderAdjOutState(recipient.peer, source.peer)
			require.Equal(t, 2, ownedRoutes, "recipient Adj-RIB-Out still names the invalid source before outbound admission")
			require.Positive(t, owedRecords, "recovery owed records remain open before outbound admission")
			require.True(t, openDone, "owed record completion channel stays open before flushWrites")

			held, leases := sourceReceiptRecordHeld(receiptID)
			require.True(t, held, "source receipt record must stay alive until outbound admission completes")
			require.Positive(t, leases, "bgp-rs must keep its source lease until admitRecovery completes")

			_, withdrawnBefore := fatalLengthRecipientRoutes(t, recipient)
			require.Zero(t, withdrawnBefore, "no withdrawal reached recipient TCP before admission unblocked")

			// Unblock outbound admission: withdrawal or alternate replacement
			// flushes to recipient TCP, owed records settle, and only then is
			// the final source lease released.
			unblock()

			if withAlternate {
				lowEventually(t, func() bool {
					_, withdrawn := fatalLengthRecipientRoutes(t, recipient)
					return withdrawn == 2
				}, "only prefix without survivor withdrawn on recipient TCP")
				lowEventually(t, func() bool {
					recipient.mu.Lock()
					defer recipient.mu.Unlock()
					for _, frame := range recipient.frames[beforeFatal:] {
						if frame[18] != byte(msgtype.TypeUPDATE) {
							continue
						}
						update, unpackErr := message.UnpackUpdate(frame[message.HeaderLen:])
						require.NoError(t, unpackErr)
						if fatalLengthNLRIMask(t, update.NLRI)&1 == 0 {
							continue
						}
						_, _, gotAS, hasAS := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
						_, _, wantAS, _ := attribute.AttrFind(backup.PathAttributes, attribute.AttrASPath)
						_, _, gotNH, hasNH := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
						_, _, wantNH, _ := attribute.AttrFind(backup.PathAttributes, attribute.AttrNextHop)
						if hasAS && hasNH && bytes.Equal(gotAS, wantAS) && bytes.Equal(gotNH, wantNH) {
							return true
						}
					}
					return false
				}, "alternate replacement advertised to recipient TCP")
			} else {
				lowEventually(t, func() bool {
					_, withdrawn := fatalLengthRecipientRoutes(t, recipient)
					return withdrawn == 3
				}, "both invalid routes withdrawn on recipient TCP")
			}

			lowEventually(t, func() bool {
				owned, owed, _ := fatalOrderAdjOutState(recipient.peer, source.peer)
				return owned == 0 && owed == 0
			}, "recipient Adj-RIB-Out owner and owed records cleared after outbound admission")

			lowEventually(t, func() bool {
				stillHeld, _ := sourceReceiptRecordHeld(receiptID)
				return !stillHeld
			}, "final source receipt lease released only after outbound admission completed")

			if withAlternate {
				altOwned, _, _ := fatalOrderAdjOutState(recipient.peer, alternate.peer)
				require.Equal(t, 1, altOwned, "recipient Adj-RIB-Out now names the surviving alternate")
			}
			require.Equal(t, PeerStateEstablished, recipient.peer.State())
		})
	}
}

// TestRFC8654FatalLengthOrdinaryRIBHoldsSourceOwnerUntilSelectedOutput exercises
// the ordinary BGP speaker path (bgp-rib alone, no bgp-rs). Pausing the
// recipient's post-fatal SelectRoutes call proves that bgp-rib holds its final
// source lease and the engine retains Adj-RIB-Out ownership until the outbound
// withdrawal or alternate replacement completes.
//
// VALIDATES: RFC 4271 Section 6 owner-release ordering in ordinary BGP speaker
// mode (without bgp-rs) on both withdraw-only and alternate-best fatal teardowns.
// PREVENTS: bgp-rib releasing its source lease on peer DOWN before selected
// outbound admission writes the withdrawal or replacement to the recipient.
// MUTATION: omit beginDownSourceHold/finishDownSourceHold in bgp-rib
// handleStructuredState: sourceReceiptRecordHeld drops to false while
// SelectRoutes is still paused.
//
// RFC 8654 Section 5: "Similarly, any speaker that treats an improper BGP Extended
// Message as a fatal error MUST follow the error-handling procedures of [RFC4271]."
// RFC 4271 Section 6: "Before the invalid routes are deleted from the system, it
// advertises, to its peers, either withdraws for the routes marked as invalid, or
// the new best routes before the invalid routes are deleted from the system."
// RFC requirement: RFC8654-5-4 positive -- on fatal Length 4097 teardown in ordinary BGP speaker mode (bgp-rib without bgp-rs), recipient Adj-RIB-Out ownership and bgp-rib's final source lease stay held until SelectRoutes completes the outbound withdrawal or alternate replacement, and release only afterward.
func TestRFC8654FatalLengthOrdinaryRIBHoldsSourceOwnerUntilSelectedOutput(t *testing.T) {
	for _, withAlternate := range []bool{false, true} {
		name := "withdraw-no-survivor"
		if withAlternate {
			name = "replace-with-alternate"
		}
		t.Run(name, func(t *testing.T) {
			t.Cleanup(redistevents.PreserveForTest())
			recipientAddr := netip.MustParseAddr("192.0.2.243")
			var gate *selectedOrderGateAPI
			settings := []*PeerSettings{
				acceptingLiveSettings(t, "192.0.2.241", 65000, 65000),
				acceptingLiveSettings(t, recipientAddr.String(), 65000, 65004),
			}
			if withAlternate {
				settings = append(settings, acceptingLiveSettings(t, "192.0.2.242", 65000, 65000))
			}
			for _, cap := range settings[0].Capabilities {
				_, extended := cap.(*capability.ExtendedMessage)
				require.False(t, extended, "source fixture must not advertise Extended Message")
			}

			rawPeers := lowLiveStart(t, lowLiveOptions{
				withoutRS: true,
				api: func(adapter *reactorAPIAdapter) plugin.ReactorLifecycle {
					gate = &selectedOrderGateAPI{
						reactorAPIAdapter: adapter,
						dest:              recipientAddr,
						entered:           make(chan struct{}),
						release:           make(chan struct{}),
					}
					return gate
				},
			}, settings...)
			t.Cleanup(func() {
				if gate != nil {
					gate.unblock()
				}
			})

			loc := locrib.Default()
			require.NotNil(t, loc)
			protocol := redistevents.RegisterProtocol("rfc8654-ordinary-owner-order-test")
			for _, address := range []string{"198.18.249.1", "198.18.249.2"} {
				host := netip.PrefixFrom(netip.MustParseAddr(address), 32)
				loc.Insert(family.IPv4Unicast, host, locrib.Path{Source: protocol, Metric: 7})
				t.Cleanup(func() { loc.Remove(family.IPv4Unicast, host, protocol, 0) })
				require.True(t, igpcost.Resolve(loc, host.Addr()).Resolved)
			}

			live := make([]*lowLivePeer, 0, len(rawPeers))
			for i, peer := range rawPeers {
				require.False(t, peer.Settings().RSClient)
				live = append(live, connectFatalOrderTCPPeer(t, peer, 0x0a00f201+uint32(i)))
			}
			source, recipient := live[0], live[1]
			var alternate *lowLivePeer
			if withAlternate {
				alternate = live[2]
			}

			sourceSession := source.peer.currentSession()
			require.NotNil(t, sourceSession)
			liveReceipt := sourceSession.sourceReceipt.Load()
			require.NotNil(t, liveReceipt)
			receiptID := liveReceipt.ID()
			require.NotZero(t, receiptID)

			prefix := netip.MustParsePrefix("198.19.248.0/24")
			ownershipLiveSend(t, source, nil, ordinarySelectedAttrs("198.18.249.1", 200), prefix)
			ordinarySelectedStored(t, source, prefix)
			ordinarySelectedBest(t, prefix, "198.18.249.1")
			ordinarySelectedWire(t, recipient, prefix, "198.18.249.1")

			if withAlternate {
				ownershipLiveSend(t, alternate, nil, ordinarySelectedAttrs("198.18.249.2", 100), prefix)
				ordinarySelectedStored(t, alternate, prefix)
				ordinarySelectedBest(t, prefix, "198.18.249.1")
			}

			// Arm the recipient's SelectRoutes gate only after initial setup settled.
			gate.armed.Store(true)

			events := &peerDownRecorder{established: make(chan *Peer, 4), closed: make(chan *Peer, 4)}
			source.peer.reactor.addPeerObserver(events)

			// RFC 8654 Section 6: 4097-octet UPDATE header without local capability 6.
			header := append(bytes.Repeat([]byte{0xff}, message.MarkerLen), 0x10, 0x01, byte(msgtype.TypeUPDATE))
			source.send(t, header)

			requireEstablishedReleased(t, &mpLinkNeighbor{peer: source.peer, events: events}, sourceSession)
			lowEventually(t, func() bool { return sourceSession.State() == fsm.StateIdle }, "fatal session Idle")
			select {
			case <-source.stopped:
			case <-time.After(5 * time.Second):
				t.Fatal("fatal-length TCP connection never closed")
			}
			require.NoError(t, source.remote.SetReadDeadline(time.Now().Add(time.Second)))
			var octet [1]byte
			n, err := source.remote.Read(octet[:])
			require.Zero(t, n)
			require.ErrorIs(t, err, io.EOF)
			wantNotify := append(bytes.Repeat([]byte{0xff}, message.MarkerLen), 0, 23, 3, 1, 2, 0x10, 0x01)
			source.mu.Lock()
			var notifications [][]byte
			for _, frame := range source.frames {
				if frame[18] == byte(msgtype.TypeNOTIFICATION) {
					notifications = append(notifications, frame)
				}
			}
			source.mu.Unlock()
			require.Equal(t, [][]byte{wantNotify}, notifications, "exact Bad Message Length NOTIFICATION")

			select {
			case <-gate.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("ordinary selected output never reached recipient SelectRoutes")
			}

			// WHILE PAUSED before SelectRoutes executes: recipient Adj-RIB-Out
			// still names the invalid source, and bgp-rib's final source lease
			// in sourceReceipts MUST still be held.
			ownedRoutes, _, _ := fatalOrderAdjOutState(recipient.peer, source.peer)
			require.Equal(t, 1, ownedRoutes, "recipient Adj-RIB-Out still names the invalid source before SelectRoutes")

			held, leases := sourceReceiptRecordHeld(receiptID)
			require.True(t, held, "source receipt record must stay alive while ordinary selected withdrawal/replacement is paused")
			require.Positive(t, leases, "bgp-rib must hold its source lease until SelectRoutes completes")

			viewBefore := ownershipLiveRead(t, recipient)
			require.Equal(t, netip.MustParseAddr("198.18.249.1"), ordinarySelectedNextHop(t, viewBefore.routes[prefix]))
			ownershipLiveWithdrawals(t, viewBefore, prefix, 0)

			// Unblock SelectRoutes: recipient TCP receives the withdrawal or
			// alternate replacement, Adj-RIB-Out transitions, and only then
			// bgp-rib releases its final source lease.
			gate.unblock()

			if withAlternate {
				ordinarySelectedWire(t, recipient, prefix, "198.18.249.2")
				viewAfter := ownershipLiveRead(t, recipient)
				ownershipLiveWithdrawals(t, viewAfter, prefix, 0)
			} else {
				lowEventually(t, func() bool {
					view := ownershipLiveRead(t, recipient)
					return view.withdrawals[prefix] == 1
				}, "ordinary recipient receives withdrawal on TCP")
			}

			lowEventually(t, func() bool {
				owned, _, _ := fatalOrderAdjOutState(recipient.peer, source.peer)
				return owned == 0
			}, "recipient Adj-RIB-Out drops invalid source after SelectRoutes")

			lowEventually(t, func() bool {
				stillHeld, _ := sourceReceiptRecordHeld(receiptID)
				return !stillHeld
			}, "bgp-rib releases final source lease only after SelectRoutes completed")

			if withAlternate {
				altOwned, _, _ := fatalOrderAdjOutState(recipient.peer, alternate.peer)
				require.Equal(t, 1, altOwned, "recipient Adj-RIB-Out now names the surviving alternate")
			}
			require.Equal(t, PeerStateEstablished, recipient.peer.State())
		})
	}
}

// extendedRecoveryPeersWithGate mirrors extendedRecoveryPeers and installs
// handler on r.fwdPool before starting any worker goroutine.
func extendedRecoveryPeersWithGate(t *testing.T, alternate bool, handler func(fwdKey, []fwdItem)) []*lowLivePeer {
	t.Helper()
	t.Cleanup(redistevents.PreserveForTest())
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	r.fwdPool.handler = handler
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
	}
	if alternate {
		settings = append(settings, lowLiveSettings("192.0.2.3", 65000, 65004))
	}
	for _, s := range settings {
		s.RSClient = true
		s.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
		s.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
		for _, binding := range []struct{ name, receive string }{
			{"bgp-rib", "update state refresh"},
			{"bgp-rs", "update state open-received refresh"},
			{"bgp-adj-rib-in", "update-received state"},
		} {
			addTestPeerBusGrant(s, testPeerEndpointGrant{
				Name:    binding.name,
				Receive: strings.Fields(binding.receive),
				Send:    []string{"update"},
			})
		}
		require.NoError(t, r.AddPeer(s))
	}
	srv := flowForwardPluginServer(t, r)
	r.SetPluginServer(srv)
	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)

	loc := locrib.Default()
	require.NotNil(t, loc)
	protocol := redistevents.RegisterProtocol("rfc8654-rs-owner-order-test")
	hops := []netip.Addr{netip.MustParseAddr("192.0.2.1")}
	if alternate {
		hops = append(hops, netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.3"))
	}
	for _, hop := range hops {
		host := netip.PrefixFrom(hop, hop.BitLen())
		loc.Insert(family.IPv4Unicast, host, locrib.Path{Source: protocol, Metric: 1})
		t.Cleanup(func() { loc.Remove(family.IPv4Unicast, host, protocol, 0) })
		require.True(t, igpcost.Resolve(loc, hop).Resolved)
	}

	peers := make([]*lowLivePeer, 0, len(settings))
	for i, s := range settings {
		peers = append(peers, connectFatalOrderTCPPeer(t, r.peers[s.PeerKey()], 0x0a000001+uint32(i)))
	}
	return peers
}

func connectFatalOrderTCPPeer(t *testing.T, peer *Peer, routerID uint32) *lowLivePeer {
	t.Helper()
	s := peer.Settings()
	lowEventually(t, func() bool {
		return peer.currentSession() != nil && peer.SessionState() == fsm.StateActive
	}, "peer active")
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	remote, err := (&net.Dialer{}).DialContext(t.Context(), "tcp4", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = remote.Close() })
	server, err := listener.Accept()
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	capture := &lowLivePeer{peer: peer, remote: remote, stopped: make(chan struct{})}
	capture.startCapture(t)
	t.Cleanup(func() {
		_ = remote.Close()
		_ = server.Close()
		<-capture.stopped
	})
	require.NoError(t, peer.acceptConnection(server))
	open := &message.Open{Version: 4, MyAS: uint16(s.PeerAS), HoldTime: 90,
		BGPIdentifier:  routerID,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, byte(s.PeerAS >> 8), byte(s.PeerAS), 2, 6, 1, 4, 0, 1, 0, 1}}
	capture.send(t, message.PackTo(open, nil))
	lowEventually(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm }, "OPEN accepted")
	capture.send(t, message.PackTo(message.NewKeepalive(), nil))
	lowEventually(t, func() bool {
		return peer.State() == PeerStateEstablished && !peer.pendingSync()
	}, "peer initial sync")
	return capture
}
