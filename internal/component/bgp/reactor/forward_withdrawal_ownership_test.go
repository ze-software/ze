// Design: docs/architecture/bgp/structural-forwarding.md -- final-writer route ownership.
// Related: forward_initial_sync_order_test.go -- real forwarding pool and recording connection.
// RFC 4271 Section 9 -- see rfc/short/rfc4271.md.
// RFC 7911 Section 5 -- see rfc/short/rfc7911.md.
// RFC 8277 Section 2.4 -- see rfc/short/rfc8277.md.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRSWithdrawalOwnershipRails distinguishes destination ownership from a
// per-source accepted-route set. Each source legitimately advertised P, but only
// the last writer owns its non-ADD-PATH destination slot. The marker fences the
// negative assertion; the final current-owner withdrawal is a separate control.
func TestRSWithdrawalOwnershipRails(t *testing.T) {
	for _, rail := range []string{"cached", "direct", "pool-fallback", "export-fallback"} {
		t.Run(rail, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			// RFC 4271 Section 4.3.
			a := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1)
			// RFC 4271 Section 4.3.
			b := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 2)
			f.send(t, rail, f.source, f.ctxID, a)
			f.await(t, 1, syncOrderPrefixWire)
			f.send(t, rail, f.other, f.ctxID, b)
			f.await(t, 2, syncOrderPrefixWire)
			f.send(t, rail, f.source, f.ctxID, syncOrderWithdrawBody)
			f.marker(t, rail, 3)
			f.assertHistory(t, []byte{1, 2}, 0)
			f.send(t, rail, f.other, f.ctxID, syncOrderWithdrawBody)
			f.marker(t, rail, 4)
			f.assertHistory(t, []byte{1, 2}, 1)
		})
	}
}

// TestRSWithdrawalOwnershipBufferedDirect exercises A/B/withdraw while all
// UPDATEs are still in the actual destination bufWriter, before any flush.
// No sent-event callback is installed: an unapplied sent projection cannot be
// the authority for either the unknown-source rejection or the positive send.
func TestRSWithdrawalOwnershipBufferedDirect(t *testing.T) {
	f := ownershipRailNew(t, nil)
	// RFC 4271 Section 4.3.
	f.sendUnflushed(t, f.source, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1))
	// RFC 4271 Section 4.3.
	f.sendUnflushed(t, f.other, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 2))
	f.sendUnflushed(t, f.source, syncOrderWithdrawBody)
	f.flush(t)
	f.marker(t, "direct", 3)
	f.assertHistory(t, []byte{1, 2}, 0)
	f.send(t, "direct", f.other, f.ctxID, syncOrderWithdrawBody)
	f.marker(t, "direct", 4)
	f.assertHistory(t, []byte{1, 2}, 1)
}

// TestRSWithdrawalOwnershipBeforeSentProjection proves the wire decision does
// not wait for an asynchronous sent-RIB projection. This fixture deliberately
// has no event consumer; actual TCP output is the only ownership evidence.
func TestRSWithdrawalOwnershipBeforeSentProjection(t *testing.T) {
	for _, rail := range []string{"cached", "direct", "pool-fallback"} {
		t.Run(rail, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			// RFC 4271 Section 4.3.
			f.send(t, rail, f.source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1))
			f.await(t, 1, syncOrderPrefixWire)
			f.send(t, rail, f.other, f.ctxID, syncOrderWithdrawBody)
			f.marker(t, rail, 2)
			f.assertHistory(t, []byte{1}, 0)
			f.send(t, rail, f.source, f.ctxID, syncOrderWithdrawBody)
			f.marker(t, rail, 3)
			f.assertHistory(t, []byte{1}, 1)
		})
	}
}

// TestRSWithdrawalOwnershipSourceGeneration holds an old withdrawal in the
// real worker, restarts its source, and then advertises the fresh generation.
// The marker proves that the queued old withdrawal was consumed, not just late.
func TestRSWithdrawalOwnershipSourceGeneration(t *testing.T) {
	for _, rail := range []string{"cached", "pool-fallback"} {
		t.Run(rail, func(t *testing.T) {
			gate := ownershipRailNewGate()
			f := ownershipRailNew(t, gate)
			// RFC 4271 Section 4.3.
			f.send(t, rail, f.source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1))
			f.await(t, 1, syncOrderPrefixWire)
			gate.armed.Store(true)
			f.send(t, rail, f.source, f.ctxID, syncOrderWithdrawBody)
			awaitEntered(t, gate.entered)
			if gate.source != f.source || gate.generation != f.source.forwardGeneration.Load() || gate.messageID != f.id {
				t.Fatal("gate did not hold the recipient's withdrawal from the old source generation")
			}
			f.source.setState(PeerStateStopped)
			f.source.setState(PeerStateEstablished)
			if gate.generation == f.source.forwardGeneration.Load() {
				t.Fatal("source restart did not advance its forwarding generation")
			}
			// RFC 4271 Section 4.3.
			f.send(t, rail, f.source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 2))
			gate.release()
			f.marker(t, rail, 3)
			f.assertHistory(t, []byte{1, 2}, 0)
			f.send(t, rail, f.source, f.ctxID, syncOrderWithdrawBody)
			f.marker(t, rail, 4)
			f.assertHistory(t, []byte{1, 2}, 1)
		})
	}
}

// TestRSWithdrawalOwnershipRetainedSource accepts a current-generation
// withdrawal of the same source's retained advertisement, with a NEW message
// ID, but never grants that source permission over a later owner's replacement.
// The rail models retained writer state, not GR capability/timer negotiation.
func TestRSWithdrawalOwnershipRetainedSource(t *testing.T) {
	for _, replaced := range []bool{false, true} {
		name := "retained"
		if replaced {
			name = "replaced"
		}
		t.Run(name, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			// RFC 4271 Section 4.3.
			f.send(t, "cached", f.source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1))
			f.await(t, 1, syncOrderPrefixWire)
			want := []byte{1}
			withdrawals := 1
			if replaced {
				// RFC 4271 Section 4.3.
				f.send(t, "cached", f.other, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 2))
				f.await(t, 2, syncOrderPrefixWire)
				want = append(want, 2)
				withdrawals = 0
			}
			f.source.setState(PeerStateStopped)
			f.source.setState(PeerStateEstablished)
			f.send(t, "cached", f.source, f.ctxID, syncOrderWithdrawBody)
			f.marker(t, "cached", 3)
			f.assertHistory(t, want, withdrawals)
		})
	}
}

// TestRSWithdrawalOwnershipDestinationGeneration queues an old-session
// withdrawal, replaces the destination session, then drains the old work.
// A fresh announcement and marker must reach the replacement connection without
// importing the old withdrawal, even though it was legitimate on the old one.
func TestRSWithdrawalOwnershipDestinationGeneration(t *testing.T) {
	for _, rail := range []string{"cached", "pool-fallback"} {
		t.Run(rail, func(t *testing.T) {
			gate := ownershipRailNewGate()
			f := ownershipRailNew(t, gate)
			// RFC 4271 Section 4.3.
			f.send(t, rail, f.source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1))
			f.await(t, 1, syncOrderPrefixWire)
			gate.armed.Store(true)
			f.send(t, rail, f.source, f.ctxID, syncOrderWithdrawBody)
			awaitEntered(t, gate.entered)
			next, conn := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.destination.sendCtxID)
			f.destination.setState(PeerStateStopped)
			f.destination.mu.Lock()
			f.destination.session = next.session
			f.destination.mu.Unlock()
			f.destination.setState(PeerStateEstablished)
			f.destination.sendingInitialRoutes.Store(0)
			f.conn = conn
			// RFC 4271 Section 4.3.
			f.send(t, rail, f.other, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 2))
			gate.release()
			f.marker(t, rail, 3)
			f.assertHistory(t, []byte{2}, 0)
			f.send(t, rail, f.other, f.ctxID, syncOrderWithdrawBody)
			f.marker(t, rail, 4)
			f.assertHistory(t, []byte{2}, 1)
		})
	}
}

// TestRSWithdrawalOwnershipNativeIdentity uses the registered native splitters
// on actual output for body, MP, labeled and VPN routes. Ingress identifiers
// zero, seven and seventeen are negotiated; egress IDs are learned from the
// announcements, never assumed equal to the ingress identifiers.
func TestRSWithdrawalOwnershipNativeIdentity(t *testing.T) {
	families := []family.Family{
		family.IPv4Unicast, family.IPv6Unicast,
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
	}
	for _, fam := range families {
		for _, rail := range []string{"cached", "direct", "pool-fallback"} {
			t.Run(fam.String()+"/"+rail, func(t *testing.T) {
				f := ownershipRailNew(t, nil)
				ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: true})
				ctxID, err := bgpctx.Registry.Register(ctx)
				if err != nil {
					t.Fatal(err)
				}
				f.destination.sendCtx.Store(ctx)
				f.destination.sendCtxID = ctxID
				f.destination.session.setSendCtxID(ctxID)
				f.destination.negotiated.Load().families[fam] = true
				f.destination.refreshForwardFacts()
				for _, id := range []uint32{0, 7, 17} {
					// RFC 7911 Section 3 and RFC 8277 Section 2.4.
					raw := ownershipRailNative(fam, id, 1, false, 0)
					// RFC 4271 Section 4.3 and RFC 4760 Section 3.
					f.send(t, rail, f.source, ctxID, ownershipRailBody(fam, false, raw, byte(id+1)))
					f.awaitNative(t, fam, byte(id+1), raw[4:])
				}
				// RFC 4271 Section 4.3 and RFC 7911 Section 3.
				before := ownershipRailRead(t, f.conn.written(), ctx)
				// RFC 4271 Section 4.3, RFC 4760 Section 4 and RFC 8277 Section 2.4.
				f.send(t, rail, f.other, ctxID, ownershipRailBody(fam, true, ownershipRailNative(fam, 7, 1, true, 0x80), 0))
				// Same source, unknown path ID and (for VPN) unknown RD.
				// RFC 4271 Section 4.3, RFC 4760 Section 4 and RFC 8277 Section 2.4.
				f.send(t, rail, f.source, ctxID, ownershipRailBody(fam, true, ownershipRailNative(fam, 99, 1, true, 0xde), 0))
				if fam.SAFI == family.SAFIVPN {
					// RFC 4760 Section 4 and RFC 8277 Section 2.4.
					f.send(t, rail, f.source, ctxID, ownershipRailBody(fam, true, ownershipRailNative(fam, 7, 2, true, 0xde), 0))
				}
				f.marker(t, rail, 90)
				// RFC 7911 Section 5 and RFC 8277 Section 2.4.
				ownershipRailAssertNative(t, before, ownershipRailRead(t, f.conn.written(), ctx), fam, nil)
				for i, id := range []uint32{0, 7, 17} {
					compatibility := byte(0x80)
					if i == 1 {
						compatibility = 0xde
					}
					// RFC 4271 Section 4.3, RFC 4760 Section 4 and RFC 8277 Section 2.4.
					f.send(t, rail, f.source, ctxID, ownershipRailBody(fam, true, ownershipRailNative(fam, id, 1, true, compatibility), 0))
				}
				f.marker(t, rail, 91)
				// RFC 7911 Section 5 and RFC 8277 Section 2.4.
				ownershipRailAssertNative(t, before, ownershipRailRead(t, f.conn.written(), ctx), fam, []byte{1, 8, 18})
			})
		}
	}
}

// TestRSWithdrawalOwnershipCollapsedPaths distinguishes source identity from
// source PATH identity: path two replaces path one at a non-ADD-PATH recipient,
// so path one's later withdrawal cannot erase path two from that recipient.
func TestRSWithdrawalOwnershipCollapsedPaths(t *testing.T) {
	for _, rail := range []string{"cached", "direct", "pool-fallback"} {
		t.Run(rail, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
			id, err := bgpctx.Registry.Register(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []uint32{1, 2} {
				// RFC 4271 Section 4.3 and RFC 7911 Section 3.
				f.send(t, rail, f.source, id, ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(path, syncOrderPrefix), byte(path)))
				f.await(t, byte(path), syncOrderPrefixWire)
			}
			// RFC 4271 Section 4.3 and RFC 7911 Section 3.
			f.send(t, rail, f.source, id, ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(1, syncOrderPrefix), 0))
			f.marker(t, rail, 3)
			f.assertHistory(t, []byte{1, 2}, 0)
			// RFC 4271 Section 4.3 and RFC 7911 Section 3.
			f.send(t, rail, f.source, id, ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(2, syncOrderPrefix), 0))
			f.marker(t, rail, 4)
			f.assertHistory(t, []byte{1, 2}, 1)
		})
	}
}

// TestRSWithdrawalOwnershipFamilyRDIsolation advertises identical IPv4 prefix
// bytes under unicast, labeled and two VPN RDs at the SAME time. Withdrawing a
// labeled slot and RD two must leave unicast and RD one's newer owner intact.
func TestRSWithdrawalOwnershipFamilyRDIsolation(t *testing.T) {
	for _, rail := range []string{"cached", "direct", "pool-fallback"} {
		t.Run(rail, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			labeled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
			vpn := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
			f.destination.negotiated.Load().families[labeled] = true
			f.destination.negotiated.Load().families[vpn] = true
			f.destination.refreshForwardFacts()
			families := []family.Family{family.IPv4Unicast, labeled, vpn, vpn}
			rds := []byte{1, 1, 1, 2}
			want := make(map[family.Family]map[string]byte)
			keys := make([]string, len(families))
			for i, fam := range families {
				// RFC 7911 Section 3 and RFC 8277 Section 2.4.
				raw := ownershipRailNative(fam, 0, rds[i], false, 0)[4:]
				var scratch [nlrisplit.PrefixKeyScratchSize]byte
				// RFC 8277 Section 2.4.
				key, err := nlrisplit.GetPrefixKey(fam)(raw, scratch[:], false)
				if err != nil {
					t.Fatal(err)
				}
				keys[i] = string(key)
				if want[fam] == nil {
					want[fam] = make(map[string]byte)
				}
				want[fam][keys[i]] = byte(i + 1)
				// RFC 4271 Section 4.3 and RFC 4760 Section 3.
				f.send(t, rail, f.source, f.ctxID, ownershipRailBody(fam, false, raw, byte(i+1)))
				f.awaitNative(t, fam, byte(i+1), raw)
			}
			// RFC 7911 Section 3 and RFC 8277 Section 2.4.
			replacement := ownershipRailNative(vpn, 0, 1, false, 0)[4:]
			// RFC 4760 Section 3.
			f.send(t, rail, f.other, f.ctxID, ownershipRailBody(vpn, false, replacement, 5))
			f.awaitNative(t, vpn, 5, replacement)
			want[vpn][keys[2]] = 5
			for _, i := range []int{1, 2, 3} {
				// RFC 8277 Section 2.4.
				raw := ownershipRailNative(families[i], 0, rds[i], true, 0xde)[4:]
				// RFC 4271 Section 4.3 and RFC 4760 Section 4.
				f.send(t, rail, f.source, f.ctxID, ownershipRailBody(families[i], true, raw, 0))
			}
			delete(want[labeled], keys[1])
			delete(want[vpn], keys[3])
			f.marker(t, rail, 90)
			current := make(map[family.Family]map[string]byte)
			var removed []string
			// RFC 4271 Section 4.3 and RFC 4760 Sections 3 and 4.
			for _, event := range ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load()) {
				if event.origin == 90 {
					continue
				}
				if current[event.family] == nil {
					current[event.family] = make(map[string]byte)
				}
				if event.withdraw {
					removed = append(removed, event.family.String()+":"+event.key)
					delete(current[event.family], event.key)
				} else {
					current[event.family][event.key] = event.origin
				}
			}
			wantRemoved := []string{labeled.String() + ":" + keys[1], vpn.String() + ":" + keys[3]}
			if !reflect.DeepEqual(removed, wantRemoved) {
				t.Errorf("native withdrawal history = %x, want %x", removed, wantRemoved)
			}
			if !reflect.DeepEqual(current, want) {
				t.Errorf("recipient family/RD attributes = %#v, want %#v", current, want)
			}
		})
	}
}

// ownershipRailFixture owns no goroutine; the existing rail owns and joins its
// worker pool. Zero is uninitialized and only ownershipRailNew constructs it.
type ownershipRailFixture struct {
	r           *Reactor
	source      *Peer
	other       *Peer
	destination *Peer
	conn        *recordingConn
	ctxID       bgpctx.ContextID
	id          uint64
}

func ownershipRailNew(t *testing.T, gate *ownershipRailGate) *ownershipRailFixture {
	t.Helper()
	handler := fwdBatchHandler
	if gate != nil {
		handler = gate.handle
	}
	r, source, destination, conn, ctxID := newSyncOrderRailWith(t, handler)
	if gate != nil {
		gate.destination = fwdKey{peerAddr: destination.Settings().PeerKey()}
		t.Cleanup(gate.release)
	}
	destination.sendingInitialRoutes.Store(0)
	other := NewPeer(&PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.3"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020303,
	})
	other.state.Store(int32(PeerStateEstablished))
	other.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{family.IPv4Unicast: true},
	})
	other.sendCtx.Store(source.sendCtx.Load())
	other.sendCtxID = ctxID
	other.refreshForwardFacts()
	r.peers[other.Settings().PeerKey()] = other
	return &ownershipRailFixture{r: r, source: source, other: other, destination: destination, conn: conn, ctxID: ctxID, id: 91000}
}

func (f *ownershipRailFixture) publish(t *testing.T, source *Peer, ctxID bgpctx.ContextID, body []byte) *ReceivedUpdate {
	t.Helper()
	f.id++
	update := syncOrderPublish(t, f.r, ctxID, f.id, body)
	update.WireUpdate.SetSourceID(source.SourceID())
	update.SourcePeerIP = source.Settings().Address
	update.SourcePeerStr = source.Settings().Address.String()
	update.receivedPeer = source
	update.receivedGeneration = source.forwardGeneration.Load()
	return update
}

func (f *ownershipRailFixture) send(t *testing.T, rail string, source *Peer, ctxID bgpctx.ContextID, body []byte) {
	t.Helper()
	update := f.publish(t, source, ctxID, body)
	api := &reactorAPIAdapter{r: f.r}
	if rail == "cached" {
		if err := api.ForwardUpdatesDirect([]uint64{f.id}, []netip.AddrPort{f.destination.Settings().PeerKey()}, "route-server", plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		return
	}
	if rail == "export-fallback" {
		// Keep the active export chain on the real cached fallback. The existing
		// policy seam accepts its input; it does not stand in for the writer.
		f.r.api = &pluginserver.Server{}
		f.r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
			return PolicyResponse{Action: PolicyAccept}
		}
		f.r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
		f.destination.settings.ExportFilters = []filterapi.FilterRef{{Name: "policy:ownership-rail"}}
		f.destination.refreshForwardFacts()
		skipped, _ := reactorForwardRS(f.r, update, f.id, source.Settings().Address, source)
		if !reflect.DeepEqual(skipped, []netip.AddrPort{f.destination.Settings().PeerKey()}) {
			t.Fatalf("fast-rail skipped destinations = %v", skipped)
		}
		if err := api.ForwardUpdatesDirect([]uint64{f.id}, skipped, "route-server", plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		return
	}
	if rail == "pool-fallback" {
		f.destination.session.writeMu.Lock()
		reactorForwardRS(f.r, update, f.id, source.Settings().Address, source)
		f.destination.session.writeMu.Unlock()
		return
	}
	reactorForwardRS(f.r, update, f.id, source.Settings().Address, source)
	f.flush(t)
}

func (f *ownershipRailFixture) sendUnflushed(t *testing.T, source *Peer, body []byte) {
	t.Helper()
	update := f.publish(t, source, f.ctxID, body)
	reactorForwardRS(f.r, update, f.id, source.Settings().Address, source)
}

func (f *ownershipRailFixture) flush(t *testing.T) {
	t.Helper()
	session := f.destination.session
	session.writeMu.Lock()
	err := session.flushWrites()
	session.writeMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *ownershipRailFixture) marker(t *testing.T, rail string, value byte) {
	t.Helper()
	raw := []byte{24, 198, 51, value}
	// RFC 4271 Section 4.3.
	f.send(t, rail, f.source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, raw, value))
	f.await(t, value, raw)
}

func (f *ownershipRailFixture) await(t *testing.T, value byte, raw []byte) {
	t.Helper()
	f.awaitNative(t, family.IPv4Unicast, value, raw)
}

func (f *ownershipRailFixture) awaitNative(t *testing.T, fam family.Family, value byte, raw []byte) {
	t.Helper()
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	// RFC 8277 Section 2.4.
	key, err := nlrisplit.GetPrefixKey(fam)(raw, scratch[:], false)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		// RFC 4271 Section 4.3 and RFC 7911 Section 3.
		for _, event := range ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load()) {
			if event.family == fam && event.key == string(key) && !event.withdraw && event.origin == value {
				return
			}
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("recipient never received family %v NLRI %x origin %d", fam, raw, value)
		}
	}
}

func (f *ownershipRailFixture) assertHistory(t *testing.T, origins []byte, withdrawals int) {
	t.Helper()
	var got []byte
	removed := 0
	var current byte
	// RFC 4271 Section 9.
	for _, event := range ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load()) {
		if event.family != family.IPv4Unicast || event.key != string(syncOrderPrefixWire) {
			continue
		}
		if event.withdraw {
			removed++
			current = 0
		} else {
			got = append(got, event.origin)
			current = event.origin
		}
	}
	if !bytes.Equal(got, origins) {
		t.Errorf("recipient announcement attributes = %v, want %v", got, origins)
	}
	if removed != withdrawals {
		t.Errorf("recipient P withdrawal history = %d, want %d", removed, withdrawals)
	}
	wantCurrent := byte(0)
	if withdrawals == 0 {
		wantCurrent = origins[len(origins)-1]
	}
	if current != wantCurrent {
		t.Errorf("recipient current P origin = %d, want %d", current, wantCurrent)
	}
	// Retain the committed body-level parser as an independent wire check.
	count := 0
	// RFC 4271 Section 4.3.
	for _, event := range parseWireUpdates(t, f.conn.written()) {
		if event.withdraws {
			count++
		}
	}
	if count != withdrawals {
		t.Errorf("body-level P withdrawals = %d, want %d", count, withdrawals)
	}
}

// ownershipRailEvent is decoded receiver history, not a production inventory.
// Its zero is an empty observation. All slices are consumed before read returns.
type ownershipRailEvent struct {
	family   family.Family
	key      string
	path     uint32
	origin   byte
	withdraw bool
}

// RFC 4271 Section 4.3: "This is a variable-length field that contains a list of IP
// address prefixes for the routes that are being withdrawn from service."
// Body offsets: [0:2] withdrawn length, [2:2+W] withdrawn, [2+W:4+W] attribute
// length, [4+W:4+W+A] attributes, [4+W+A:] announced NLRI.
func ownershipRailRead(t *testing.T, frames []byte, ctx *bgpctx.EncodingContext) []ownershipRailEvent {
	t.Helper()
	var events []ownershipRailEvent
	for len(frames) >= message.HeaderLen {
		n := int(binary.BigEndian.Uint16(frames[16:18]))
		if n < message.HeaderLen {
			t.Fatal("invalid BGP frame length")
		}
		if n > len(frames) {
			break
		}
		body := frames[message.HeaderLen:n]
		frames = frames[n:]
		// RFC 4271 Section 4.3.
		sections, err := wire.ParseUpdateSections(body)
		if err != nil {
			t.Fatal(err)
		}
		origin := byte(0)
		iter := attribute.NewAttrIterator(sections.Attrs(body))
		for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
			if code == attribute.AttrMED && len(value) == 4 {
				origin = value[3]
			}
		}
		add := func(fam family.Family, raw []byte, withdraw bool) {
			split := nlrisplit.Get(fam)
			if withdraw {
				split = nlrisplit.GetWithdraw(fam)
			}
			if split == nil {
				t.Fatalf("no registered splitter for %v", fam)
			}
			// RFC 7911 Section 3 and RFC 8277 Section 2.4.
			_, err := split(raw, ctx.AddPath(fam), func(nlri []byte) bool {
				event := ownershipRailEvent{family: fam, origin: origin, withdraw: withdraw}
				if ctx.AddPath(fam) {
					// RFC 7911 Section 3: "In order to carry the Path Identifier in an
					// UPDATE message, the NLRI encoding MUST be extended by prepending
					// the Path Identifier field, which is of four octets."
					event.path = binary.BigEndian.Uint32(nlri[:4])
					nlri = nlri[4:]
				}
				var scratch [nlrisplit.PrefixKeyScratchSize]byte
				// RFC 8277 Section 2.4.
				key, err := nlrisplit.GetPrefixKey(fam)(nlri, scratch[:], withdraw)
				if err != nil {
					t.Fatal(err)
				}
				event.key = string(key)
				events = append(events, event)
				return true
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		// RFC 4271 Section 4.3.
		add(family.IPv4Unicast, sections.Withdrawn(body), true)
		iter = attribute.NewAttrIterator(sections.Attrs(body))
		for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
			if code != attribute.AttrMPReachNLRI && code != attribute.AttrMPUnreachNLRI {
				continue
			}
			reach := code == attribute.AttrMPReachNLRI
			// RFC 4760 Sections 3 and 4.
			fam, off, err := pathsLimitMP(value, reach)
			if err != nil {
				t.Fatal(err)
			}
			// RFC 4760 Sections 3 and 4.
			add(fam, value[off:], !reach)
		}
		// RFC 4271 Section 4.3.
		add(family.IPv4Unicast, sections.NLRI(body), false)
	}
	return events
}

// RFC 4271 Section 4.3: "An UPDATE message is used to advertise feasible routes
// that share common path attributes to a peer, or to withdraw multiple unfeasible
// routes from service (see 3.1)."
// MED is the distinct attribute fingerprint; the route server preserves it.
func ownershipRailBody(fam family.Family, withdraw bool, raw []byte, origin byte) []byte {
	// RFC 4271 Section 4.3 and RFC 4760 Sections 3 and 4.
	u := pathsLimitUpdate(fam, withdraw, raw)
	if !withdraw {
		if fam != family.IPv4Unicast {
			nextHop := netip.MustParseAddr("2001:db8::1").AsSlice()
			if fam.AFI == family.AFIIPv4 {
				nextHop = netip.MustParseAddr("10.0.0.1").AsSlice()
			}
			if fam.SAFI == family.SAFIVPN {
				nextHop = append(make([]byte, 8), nextHop...)
			}
			// RFC 4760 Section 3: "The semantics of NLRI is identified by a
			// combination of <AFI, SAFI> carried in the attribute."
			// MP_REACH offsets: [0:2] AFI, [2] SAFI, [3] next-hop length,
			// [4:4+N] next hop, [4+N] reserved, [5+N:] native NLRI.
			value := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), byte(len(nextHop))}
			value = append(value, nextHop...)
			// RFC 4760 Section 3: "A 1 octet field that MUST be set to 0, and
			// SHOULD be ignored upon receipt."
			value = append(value, 0)
			value = append(value, raw...)
			u.PathAttributes = append([]byte{0x90, byte(attribute.AttrMPReachNLRI), byte(len(value) >> 8), byte(len(value))}, value...)
		}
		attrs := bytes.Clone(syncOrderAnnounceBody[4:24])
		attrs = append(attrs, 0x80, byte(attribute.AttrMED), 4, 0, 0, 0, origin)
		u.PathAttributes = append(attrs, u.PathAttributes...)
	}
	// RFC 4271 Section 4.3.
	return fwdPackUpdateBody(u)
}

// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored."
// Extended NLRI offsets: [0:4] Path Identifier, [4] bit length, [5:8] label or
// Compatibility, [8:16] RD for VPN only, then significant prefix octets.
func ownershipRailNative(fam family.Family, id uint32, rd byte, withdraw bool, compatibility byte) []byte {
	prefix := syncOrderPrefix
	if fam.AFI == family.AFIIPv6 {
		prefix = "2001:db8:1::/48"
	}
	// RFC 7911 Section 3.
	raw := pathsLimitNLRI(id, prefix)
	if fam.SAFI == family.SAFIUnicast {
		return raw
	}
	label := []byte{0, 1, 1}
	if withdraw {
		// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field
		// SHOULD be set to 0x800000." Tests also exercise a nonrecommended value.
		label = []byte{compatibility, 0, 0}
	}
	out := append(bytes.Clone(raw[:5]), label...)
	out[4] += 24
	if fam.SAFI == family.SAFIVPN {
		out = append(out, 0, 0, 0xfd, 0xe8, 0, 0, 0, rd)
		out[4] += 64
	}
	return append(out, raw[5:]...)
}

func ownershipRailAssertNative(t *testing.T, before, after []ownershipRailEvent, fam family.Family, removed []byte) {
	t.Helper()
	want := make(map[uint32]ownershipRailEvent)
	for _, event := range before {
		if event.family == fam && !event.withdraw {
			want[event.path] = event
		}
	}
	current := make(map[uint32]ownershipRailEvent)
	var gotRemoved []byte
	for _, event := range after {
		if event.family != fam {
			continue
		}
		// IPv4 causal markers have different native prefix keys.
		if event.origin >= 90 && !event.withdraw {
			continue
		}
		if event.withdraw {
			previous, ok := want[event.path]
			if !ok {
				t.Errorf("withdrawal has never-advertised destination ID %d", event.path)
				continue
			}
			if event.key != previous.key {
				t.Errorf("withdrawal changed native identity: %x, want %x", event.key, previous.key)
			}
			gotRemoved = append(gotRemoved, previous.origin)
			delete(current, event.path)
		} else {
			current[event.path] = event
		}
	}
	if !bytes.Equal(gotRemoved, removed) {
		t.Errorf("withdrawn route attributes by learned destination ID = %v, want %v", gotRemoved, removed)
	}
	for id, event := range want {
		if bytes.IndexByte(removed, event.origin) >= 0 {
			delete(want, id)
		}
	}
	if !reflect.DeepEqual(current, want) {
		t.Errorf("recipient native routes = %+v, want %+v", current, want)
	}
}

// ownershipRailGate pauses one armed production batch for its destination.
// Native forwarding fans out to other workers, which MUST NOT claim this gate.
// The fixture MUST bind destination before dispatch and release before its pool
// stops; ownershipRailNew registers that cleanup last. The captured source,
// generation and message ID are read only after entered closes, before release.
type ownershipRailGate struct {
	destination fwdKey
	source      *Peer
	generation  uint64
	messageID   uint64
	armed       atomic.Bool
	entered     chan struct{}
	open        chan struct{}
	once        sync.Once
}

func ownershipRailNewGate() *ownershipRailGate {
	return &ownershipRailGate{entered: make(chan struct{}), open: make(chan struct{})}
}

func (g *ownershipRailGate) handle(key fwdKey, items []fwdItem) {
	if key == g.destination && fwdBatchHasRealItem(items) && g.armed.CompareAndSwap(true, false) {
		for i := range items {
			if items[i].peer != nil {
				g.source = items[i].receivedPeer
				g.generation = items[i].receivedGeneration
				g.messageID = items[i].sourceMessageID
				break
			}
		}
		close(g.entered)
		<-g.open
	}
	fwdBatchHandler(key, items)
}

// release MUST run before the fixture's pool cleanup; repeated calls are safe.
func (g *ownershipRailGate) release() {
	g.once.Do(func() { close(g.open) })
}
