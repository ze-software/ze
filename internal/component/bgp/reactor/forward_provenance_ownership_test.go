// Design: docs/architecture/bgp/structural-forwarding.md -- source-owned forwarding.
// Related: forward_withdrawal_ownership_test.go -- actual writer and wire history.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRSWithdrawalOwnershipSentReplay preserves forwarded ownership across an
// actual sent-route refresh: the original source can still withdraw afterward.
func TestRSWithdrawalOwnershipSentReplay(t *testing.T) {
	f := ownershipRailNew(t, nil)
	f.r.config = &Config{LocalAS: 65000}
	body := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 11) // RFC 4271 Section 4.3.
	f.send(t, "cached", f.source, f.ctxID, body)
	f.await(t, 11, syncOrderPrefixWire)
	batch := ownershipReplayBatch(t, f, body)
	api := &reactorAPIAdapter{r: f.r}
	sel := selector.ParseDefault(f.destination.Settings().Address.String())
	if err := api.AnnounceNLRIBatch(t.Context(), sel, batch, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	f.marker(t, "cached", 80)
	f.assertHistory(t, []byte{11, 11}, 0)
	f.send(t, "cached", f.source, f.ctxID, syncOrderWithdrawBody)
	f.marker(t, "cached", 81)
	f.assertHistory(t, []byte{11, 11}, 1)
}

// TestRSWithdrawalOwnershipStaleSentWork sends old captured replay and cleanup
// after a fresh advertisement. It covers another source and a newer revision
// from the SAME source; neither automatic operation can overwrite the winner.
func TestRSWithdrawalOwnershipStaleSentWork(t *testing.T) {
	for _, replacement := range []string{"other-source", "same-source"} {
		t.Run(replacement, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			f.r.config = &Config{LocalAS: 65000}
			body := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 21) // RFC 4271 Section 4.3.
			f.send(t, "cached", f.source, f.ctxID, body)
			f.await(t, 21, syncOrderPrefixWire)
			stale := ownershipReplayBatch(t, f, body)
			source := f.other
			if replacement == "same-source" {
				source = f.source
			}
			newBody := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 22) // RFC 4271 Section 4.3.
			f.send(t, "cached", source, f.ctxID, newBody)
			f.await(t, 22, syncOrderPrefixWire)
			current := ownershipReplayBatch(t, f, newBody)
			api := &reactorAPIAdapter{r: f.r}
			sel := selector.ParseDefault(f.destination.Settings().Address.String())
			if err := api.AnnounceNLRIBatch(t.Context(), sel, stale, plugin.OperatorSender()); err != nil {
				t.Fatal(err)
			}
			if err := api.WithdrawNLRIBatch(t.Context(), sel, stale, plugin.OperatorSender()); err != nil {
				t.Fatal(err)
			}
			f.marker(t, "cached", 82)
			f.assertHistory(t, []byte{21, 22}, 0)
			if err := api.WithdrawNLRIBatch(t.Context(), sel, current, plugin.OperatorSender()); err != nil {
				t.Fatal(err)
			}
			f.marker(t, "cached", 83)
			f.assertHistory(t, []byte{21, 22}, 1)
		})
	}
}

// TestRSWithdrawalOwnershipStoredReplay reaches the public stored-route relay,
// then withdraws its retained source while another source owns the destination.
func TestRSWithdrawalOwnershipStoredReplay(t *testing.T) {
	f := ownershipRailNew(t, nil)
	f.r.clock = clock.RealClock{}
	f.source.recvCtxID = f.ctxID
	body := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 31) // RFC 4271 Section 4.3.
	parsed, err := message.UnpackUpdate(body)
	if err != nil {
		t.Fatal(err)
	}
	_, _, nextHop, found := attribute.AttrFind(parsed.PathAttributes, attribute.AttrNextHop)
	if !found {
		t.Fatal("stored announcement has no next hop")
	}
	// AdjRIBInManager.buildReplayRoutes carries the retained next hop separately,
	// even when the immutable attributes also include legacy NEXT_HOP.
	stored := rpc.StoredRoute{SourcePeer: f.source.Settings().Address.String(), Family: family.IPv4Unicast.String(),
		AttrHex: hex.EncodeToString(parsed.PathAttributes), NLRIHex: hex.EncodeToString(syncOrderPrefixWire),
		NextHopHex: hex.EncodeToString(nextHop), MsgID: nextMsgID(), NLRIFraming: rpc.NLRIFramingPrefixOnly}
	api := &reactorAPIAdapter{r: f.r}
	if err := api.RelayStoredRoute(f.destination.Settings().Address, []rpc.StoredRoute{stored}, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	f.await(t, 31, syncOrderPrefixWire)
	f.send(t, "cached", f.other, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 32)) // RFC 4271 Section 4.3.
	f.await(t, 32, syncOrderPrefixWire)
	stored.Withdraw, stored.AttrHex = true, ""
	if err := api.RelayStoredRoute(f.destination.Settings().Address, []rpc.StoredRoute{stored}, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	f.marker(t, "cached", 84)
	f.assertHistory(t, []byte{31, 32}, 0)
	f.send(t, "cached", f.other, f.ctxID, syncOrderWithdrawBody)
	f.marker(t, "cached", 85)
	f.assertHistory(t, []byte{31, 32}, 1)
}

// TestRSWithdrawalOwnershipCollapsedMixed preserves individual ingress paths
// when two announcements in ONE body collapse into one slot. A mixed stale
// withdrawal and fresh sibling then survives normalization into separate UPDATEs.
func TestRSWithdrawalOwnershipCollapsedMixed(t *testing.T) {
	for _, rail := range []string{"cached", "direct", "pool-fallback"} {
		t.Run(rail, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
			ctxID, err := bgpctx.Registry.Register(ctx)
			if err != nil {
				t.Fatal(err)
			}
			// RFC 7911 Section 3: distinct received identifiers precede each NLRI.
			paths := append(pathsLimitNLRI(1, syncOrderPrefix), pathsLimitNLRI(2, syncOrderPrefix)...)
			body := ownershipRailBody(family.IPv4Unicast, false, paths, 41) // RFC 4271 Section 4.3.
			f.send(t, rail, f.source, ctxID, body)
			f.await(t, 41, syncOrderPrefixWire)
			sibling := []byte{24, 203, 0, 113}
			body = ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(3, "203.0.113.0/24"), 42) // RFC 7911 Section 3.
			mixed, err := message.UnpackUpdate(body)
			if err != nil {
				t.Fatal(err)
			}
			mixed.WithdrawnRoutes = pathsLimitNLRI(1, syncOrderPrefix)
			f.send(t, rail, f.source, ctxID, fwdPackUpdateBody(mixed)) // RFC 4271 Section 4.3.
			f.await(t, 42, sibling)
			f.assertHistory(t, []byte{41, 41}, 0)
			f.send(t, rail, f.source, ctxID, ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(2, syncOrderPrefix), 0)) // RFC 7911 Section 5.
			f.marker(t, rail, 86)
			f.assertHistory(t, []byte{41, 41}, 1)
		})
	}
}

// ownershipReplayBatch carries the same immutable sent attributes and the
// actual writer receipt, without querying or seeding production ownership.
func ownershipReplayBatch(t *testing.T, f *ownershipRailFixture, body []byte) bgptypes.NLRIBatch {
	t.Helper()
	parsed, err := message.UnpackUpdate(body) // RFC 4271 Section 4.3.
	if err != nil {
		t.Fatal(err)
	}
	batch := adjOutBatch(syncOrderPrefix, "10.0.0.1")
	batch.Wire = attribute.NewAttributesWire(parsed.PathAttributes, f.ctxID)
	batch.Replay = true
	session := f.destination.currentSession()
	session.writeMu.Lock()
	batch.SentOwnerMessage = session.sentReceipt.MessageID()
	session.writeMu.Unlock()
	if batch.SentOwnerMessage == 0 {
		t.Fatal("successful actual-wire announcement has no sent receipt")
	}
	return batch
}

// TestRSWithdrawalOwnershipMixedSynthesizedIntent proves that denial of an
// announcement cannot turn an original received withdrawal into synthesis.
// The denied, previously unseen announcement still emits its exact withdrawal.
func TestRSWithdrawalOwnershipMixedSynthesizedIntent(t *testing.T) {
	for _, rail := range []string{"cached", "direct", "pool-fallback"} {
		t.Run(rail, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			f.send(t, rail, f.other, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 51)) // RFC 4271 Section 4.3.
			f.await(t, 51, syncOrderPrefixWire)
			ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
			ctxID, err := bgpctx.Registry.Register(ctx)
			if err != nil {
				t.Fatal(err)
			}
			body := ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(2, "203.0.113.0/24"), 52) // RFC 7911 Section 3.
			mixed, err := message.UnpackUpdate(body)
			if err != nil {
				t.Fatal(err)
			}
			mixed.WithdrawnRoutes = pathsLimitNLRI(1, syncOrderPrefix)
			deny := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
				mods.SetWithdraw()
				return true
			}
			// Reactor initialization installs registered filters on both rails.
			f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(deny)
			f.r.egressFilters = []filterapi.EgressFilterFunc{deny}
			f.send(t, rail, f.source, ctxID, fwdPackUpdateBody(mixed)) // RFC 4271 Section 4.3.
			f.r.orderedEgressSteps = nil
			f.r.egressFilters = nil
			f.marker(t, rail, 87)
			f.assertHistory(t, []byte{51}, 0)
			withdrawals := 0
			for _, event := range ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load()) { // RFC 4271 Section 4.3.
				if event.family == family.IPv4Unicast && event.key == string([]byte{24, 203, 0, 113}) {
					if !event.withdraw {
						t.Fatal("policy-denied announcement escaped on the wire")
					}
					withdrawals++
				}
			}
			if withdrawals != 1 {
				t.Fatalf("initial synthesized withdrawals = %d, want exactly one", withdrawals)
			}
		})
	}
}

// TestRSWithdrawalOwnershipUnknownPathIDsDoNotAllocate checks both observable
// refusal and bounded allocator state for unknown received ADD-PATH withdrawals.
func TestRSWithdrawalOwnershipUnknownPathIDsDoNotAllocate(t *testing.T) {
	// RegisterPeer reuses IDs by address, while the allocator outlives fixtures.
	// This source belongs only to this test. Register its cleanup before the
	// rail's cleanup so all forwarding workers stop before its IDs are released.
	source := NewPeer(&PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("198.18.0.88"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020358,
	})
	t.Cleanup(func() { fwdPathIDs.releaseSource(source.SourceID()) })
	fwdPathIDs.mu.RLock()
	initial := len(fwdPathIDs.byPath[source.SourceID()]) + len(fwdPathIDs.bySource[source.SourceID()])
	fwdPathIDs.mu.RUnlock()
	if initial != 0 {
		t.Fatalf("isolated source starts with %d path-ID mappings", initial)
	}
	f := ownershipRailNew(t, nil)
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	source.state.Store(int32(PeerStateEstablished))
	source.negotiated.Store(f.source.negotiated.Load())
	source.recvCtxID = ctxID
	source.sendCtx.Store(f.source.sendCtx.Load())
	source.sendCtxID = f.ctxID
	source.refreshForwardFacts()
	f.r.peers[source.Settings().PeerKey()] = source
	f.destination.sendCtx.Store(ctx)
	f.destination.sendCtxID = ctxID
	f.destination.session.setSendCtxID(ctxID)
	f.destination.refreshForwardFacts()
	for id := range uint32(32) {
		body := ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(id, syncOrderPrefix), 0) // RFC 7911 Sections 3 and 5.
		f.send(t, "cached", source, ctxID, body)
	}
	// Keep one full withdrawal batch alive beyond the writer fence. Allocation
	// followed by cache-eviction cleanup must not satisfy the zero-state check.
	var withdrawn []byte
	for id := range uint32(200) {
		// RFC 7911 Section 3.
		withdrawn = append(withdrawn, pathsLimitNLRI(1_000_000+id, syncOrderPrefix)...)
	}
	// RFC 4271 Section 4.3 and RFC 7911 Sections 3 and 5.
	body := ownershipRailBody(family.IPv4Unicast, true, withdrawn, 0)
	update := f.publish(t, source, ctxID, body)
	batchID := update.WireUpdate.MessageID()
	f.r.recentUpdates.retainN(batchID, 1)
	defer f.r.recentUpdates.Release(batchID)
	api := &reactorAPIAdapter{r: f.r}
	if err := api.ForwardUpdatesDirect([]uint64{batchID}, []netip.AddrPort{f.destination.Settings().PeerKey()}, "route-server", plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	// The existing source supplies the writer fence without legitimately
	// allocating an unframed identifier for the source under measurement.
	f.marker(t, "cached", 88)
	for _, event := range ownershipRailRead(t, f.conn.written(), ctx) { // RFC 4271 Section 4.3.
		if event.withdraw {
			t.Fatal("unknown received path withdrawal reached the wire")
		}
	}
	if !f.r.recentUpdates.Contains(batchID) {
		t.Fatal("the 200-ID UPDATE was evicted before the allocation assertion")
	}
	fwdPathIDs.mu.RLock()
	allocated := len(fwdPathIDs.byPath[source.SourceID()]) + len(fwdPathIDs.bySource[source.SourceID()])
	fwdPathIDs.mu.RUnlock()
	if allocated != 0 {
		t.Fatalf("unknown withdrawals retained %d path-ID mappings", allocated)
	}
}

// TestRSWithdrawalOwnershipInitialExpiryOrdering restores actual sent history
// on a new Session while an already-captured expiry is waiting behind the
// initial fence. It must restore first, then withdraw, never resurrect afterward.
func TestRSWithdrawalOwnershipInitialExpiryOrdering(t *testing.T) {
	f := ownershipRailNew(t, nil)
	f.r.config = &Config{LocalAS: 65000}
	body := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 61) // RFC 4271 Section 4.3.
	f.send(t, "cached", f.source, f.ctxID, body)
	f.await(t, 61, syncOrderPrefixWire)
	history := ownershipReplayBatch(t, f, body)
	old := f.destination.currentSession()
	if err := old.teardownAutomatic(message.NotifyCeaseOutOfResources, "test reconnect"); err != nil {
		t.Fatal(err)
	}
	next, conn := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.destination.sendCtxID)
	f.destination.setState(PeerStateStopped)
	f.source.setState(PeerStateStopped) // The exact original source is GR-retained.
	f.destination.mu.Lock()
	f.destination.session = next.session
	f.destination.mu.Unlock()
	f.destination.setState(PeerStateEstablished)
	f.destination.sendingInitialRoutes.Store(0)
	f.destination.initialSyncEOROwed.Store(true)
	f.conn = conn
	f.destination.resetAPISync([]string{"rib-history"})
	raiseTestReplayFence(f.destination, "rib-history")
	api := &reactorAPIAdapter{r: f.r}
	sel := selector.ParseDefault(f.destination.Settings().Address.String())
	if err := api.WithdrawNLRIBatch(t.Context(), sel, history, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	if !f.destination.forwardOrderHold(false) {
		t.Fatal("captured expiry did not remain behind the initial replay fence")
	}
	if len(parseWireUpdates(t, conn.written())) != 0 {
		t.Fatal("expiry wrote before initial replay")
	}
	history.InitialReplay = next.session.initialReplay
	history.InitialSourcePeer = f.source.Settings().Address.String()
	history.InitialSourceID = uint32(f.source.SourceID())
	history.InitialSourceOwner = f.source.sourceOwner
	if err := api.AnnounceNLRIBatch(t.Context(), sel, history, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	if err := f.destination.SignalAPIReady(plugin.ProcessSender("rib-history"), history.InitialReplay); err != nil {
		t.Fatal(err)
	}
	if f.destination.forwardOrderHold(false) {
		t.Fatal("matching initial replay completion did not release queued expiry")
	}
	marker := []byte{24, 198, 51, 89}
	f.send(t, "cached", f.other, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, marker, 89))
	f.await(t, 89, marker)
	f.assertHistory(t, []byte{61}, 1)
	for _, update := range parseWireUpdates(t, conn.written()) {
		if update.endOfRIB {
			t.Fatal("ownership filtering emitted a phantom End-of-RIB")
		}
	}
}

func TestRecoveryRefusesFilteredCandidateWithdrawalAfterLookup(t *testing.T) {
	f := ownershipRailNew(t, nil)
	f.r.clock = clock.RealClock{}
	f.source.recvCtxID = f.ctxID
	f.send(t, "cached", f.other, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 72))
	f.await(t, 72, syncOrderPrefixWire)
	api := &reactorAPIAdapter{r: f.r}
	session, sequence := recoverySnapshot(f.destination)
	sources := api.recoverySources()
	candidate, err := message.UnpackUpdate(ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 73))
	if err != nil {
		t.Fatal(err)
	}
	lookup := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
		// C's removal reaches the actual direct writer after selection, while
		// A still owns the destination. No withdrawal may reach the wire.
		f.send(t, "direct", f.source, f.ctxID, syncOrderWithdrawBody)
		return []ribevents.RecoveryRoute{{Source: f.source.Settings().Address,
			NLRI: syncOrderPrefixWire, SentNLRI: syncOrderPrefixWire,
			Attributes: candidate.PathAttributes, NextHop: []byte{10, 0, 0, 1}, MessageID: 81}}, nil
	})
	t.Cleanup(lookup.Close)
	retry, err := api.relayRecovery(t.Context(), ribevents.RecoveryRequest{
		Source: f.other.Settings().Address, Destination: f.destination.Settings().Address,
		Family: family.IPv4Unicast, NLRIs: [][]byte{syncOrderPrefixWire},
	}, f.destination, session, sequence, plugin.OperatorSender(), sources)
	if err != nil || !retry {
		t.Fatalf("filtered source removal failed to invalidate cold selection: retry=%v err=%v", retry, err)
	}
	f.assertHistory(t, []byte{72}, 0)
}
