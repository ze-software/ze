// Design: docs/architecture/bgp/structural-forwarding.md -- recipient-owned replay identity.
// Related: policy_reconcile_test.go -- replay and cache-eviction ordering regression.
package reactor

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// recipientIdentityRail owns an isolated allocator until its real forward worker
// stops and its recipient releases every advertised slot. No test runs in parallel.
func recipientIdentityRail(t *testing.T, gate *ownershipRailGate) *ownershipRailFixture {
	t.Helper()
	saved := fwdPathIDs
	fwdPathIDs = newFwdPathIDTable()
	t.Cleanup(func() { fwdPathIDs = saved })
	f := ownershipRailNew(t, gate)
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.ctxID = ctxID
	f.source.recvCtxID = ctxID
	f.other.recvCtxID = ctxID
	f.destination.sendCtx.Store(ctx)
	f.destination.sendCtxID = ctxID
	f.destination.session.setSendCtxID(ctxID)
	f.destination.refreshForwardFacts()
	t.Cleanup(func() {
		if gate != nil {
			gate.release()
		}
		f.r.fwdPool.Stop()
		f.destination.adjOut.reset()
		fwdPathIDs.releaseSource(f.source.SourceID())
		fwdPathIDs.releaseSource(f.other.SourceID())
		fwdPathIDs.mu.RLock()
		remaining := len(fwdPathIDs.used)
		fwdPathIDs.mu.RUnlock()
		if remaining != 0 {
			t.Errorf("recipient teardown left %d identifier reservations", remaining)
		}
	})
	return f
}

func recipientIdentityDrain(t *testing.T, f *ownershipRailFixture) []ownershipRailEvent {
	t.Helper()
	if err := f.r.fwdPool.barrierPeer(t.Context(), f.destination.Settings().PeerKey()); err != nil {
		t.Fatal(err)
	}
	return ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load())
}

func recipientIdentitySend(t *testing.T, f *ownershipRailFixture, source *Peer, path uint32, withdraw bool, origin byte) []ownershipRailEvent {
	t.Helper()
	body := ownershipRailBody(family.IPv4Unicast, withdraw, pathsLimitNLRI(path, syncOrderPrefix), origin)
	f.send(t, "cached", source, f.ctxID, body)
	return recipientIdentityDrain(t, f)
}

// recipientIdentityEvict publishes and removes a real received withdrawal without
// forwarding it. The cache releases its mapping while the recipient still holds
// the previously advertised route, as after a held withdrawal's forced eviction.
func recipientIdentityEvict(t *testing.T, f *ownershipRailFixture, source *Peer, path uint32) {
	t.Helper()
	body := ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(path, syncOrderPrefix), 0)
	f.publish(t, source, f.ctxID, body)
	if !f.r.recentUpdates.Delete(f.id) {
		t.Fatal("received withdrawal was not evicted")
	}
}

// TestRecipientAddPathIdentitySurvivesCacheEviction reannounces after mapping
// eviction and forces the allocator back to an advertised zero identifier. Same-
// source and other-source siblings retain distinct identities through withdrawal.
func TestRecipientAddPathIdentitySurvivesCacheEviction(t *testing.T) {
	f := recipientIdentityRail(t, nil)
	first := recipientIdentitySend(t, f, f.source, 7, false, 11)
	if len(first) != 1 || first[0].withdraw || first[0].path != 0 {
		t.Fatalf("initial zero-identifier advertisement = %+v", first)
	}
	recipientIdentityEvict(t, f, f.source, 7)
	fwdPathIDs.mu.Lock()
	fwdPathIDs.next = 0
	fwdPathIDs.mu.Unlock()
	recipientIdentitySend(t, f, f.source, 0, false, 12)
	recipientIdentitySend(t, f, f.other, 0, false, 13)
	events := recipientIdentitySend(t, f, f.source, 7, false, 14)
	if len(events) != 4 {
		t.Fatalf("advertisements = %+v, want four", events)
	}
	if events[0].path != events[3].path {
		t.Fatalf("reannouncement changed recipient identity: %+v", events)
	}
	if events[0].path == events[1].path || events[0].path == events[2].path || events[1].path == events[2].path {
		t.Fatalf("allocator wrap or ingress identity merged sibling paths: %+v", events)
	}
	for _, path := range []struct {
		source *Peer
		id     uint32
	}{{f.source, 7}, {f.source, 0}, {f.other, 0}} {
		events = recipientIdentitySend(t, f, path.source, path.id, true, 0)
	}
	if len(events) != 7 {
		t.Fatalf("announcement/withdrawal history = %+v, want seven events", events)
	}
	for i := range 3 {
		if !events[4+i].withdraw || events[4+i].path != events[i].path {
			t.Fatalf("withdrawal lost its recipient identity: %+v", events)
		}
	}
	f.destination.adjOut.mu.Lock()
	bindings, routes := len(f.destination.adjOut.sourcePaths), len(f.destination.adjOut.routes)
	f.destination.adjOut.mu.Unlock()
	if bindings != 0 || routes != 0 {
		t.Fatalf("withdrawal left %d source bindings and %d advertisements", bindings, routes)
	}
	fwdPathIDs.mu.RLock()
	remaining := len(fwdPathIDs.used)
	mappings := len(fwdPathIDs.bySource) + len(fwdPathIDs.byPath)
	fwdPathIDs.mu.RUnlock()
	if remaining != 0 {
		t.Fatalf("withdrawal/cache eviction left %d identifier reservations", remaining)
	}
	if mappings != 0 {
		t.Fatalf("completed withdrawals retained %d source mapping tables", mappings)
	}
}

// TestRecipientAddPathCollisionPreservesLocalOwner materializes a forward before
// a direct local writer claims its proposed ID. Both paths must reach the wire;
// neither reannouncement nor withdrawal may replace or remove the local path.
func TestRecipientAddPathCollisionPreservesLocalOwner(t *testing.T) {
	gate := ownershipRailNewGate()
	f := recipientIdentityRail(t, gate)
	gate.armed.Store(true)
	body := ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(7, syncOrderPrefix), 21)
	f.send(t, "cached", f.source, f.ctxID, body)
	awaitEntered(t, gate.entered)
	local := ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(0, syncOrderPrefix), 22)
	if err := f.destination.session.SendUpdate(ownershipWriterUpdate(t, local)); err != nil {
		t.Fatal(err)
	}
	gate.release()
	events := recipientIdentityDrain(t, f)
	if len(events) != 2 || events[0].path != 0 || events[1].path == 0 {
		t.Fatalf("local/colliding forwarded advertisements = %+v", events)
	}
	forwardedID := events[1].path
	events = recipientIdentitySend(t, f, f.source, 7, false, 23)
	if len(events) != 3 || events[2].path != forwardedID {
		t.Fatalf("collision reannouncement changed identity: %+v", events)
	}
	events = recipientIdentitySend(t, f, f.source, 7, true, 0)
	if len(events) != 4 || !events[3].withdraw || events[3].path != forwardedID {
		t.Fatalf("collision withdrawal did not preserve the local path: %+v", events)
	}
	localWithdrawal := ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(0, syncOrderPrefix), 0)
	if err := f.destination.session.SendUpdate(ownershipWriterUpdate(t, localWithdrawal)); err != nil {
		t.Fatal(err)
	}
	events = recipientIdentityDrain(t, f)
	if len(events) != 5 || !events[4].withdraw || events[4].path != 0 {
		t.Fatalf("explicit local withdrawal lost its identity: %+v", events)
	}
	f.destination.adjOut.mu.Lock()
	bindings, routes := len(f.destination.adjOut.sourcePaths), len(f.destination.adjOut.routes)
	f.destination.adjOut.mu.Unlock()
	if bindings != 0 || routes != 0 {
		t.Fatalf("completed withdrawals left %d source bindings and %d advertisements", bindings, routes)
	}
	fwdPathIDs.mu.RLock()
	remaining := len(fwdPathIDs.used)
	mappings := len(fwdPathIDs.bySource) + len(fwdPathIDs.byPath)
	fwdPathIDs.mu.RUnlock()
	if remaining != 0 || mappings != 0 {
		t.Fatalf("completed withdrawals left %d identifier reservations and %d source mapping tables", remaining, mappings)
	}
}

// TestRecipientAddPathCombinedCollisionReservesStagedIDs holds a real two-path
// forward until both source mappings are evicted. After allocator wrap, the
// second path's collision must not reuse the first path's newly staged ID.
func TestRecipientAddPathCombinedCollisionReservesStagedIDs(t *testing.T) {
	gate := ownershipRailNewGate()
	f := recipientIdentityRail(t, gate)
	gate.armed.Store(true)
	nlri := append(pathsLimitNLRI(7, syncOrderPrefix), pathsLimitNLRI(8, syncOrderPrefix)...)
	body := ownershipRailBody(family.IPv4Unicast, false, nlri, 25)
	f.send(t, "cached", f.source, f.ctxID, body)
	awaitEntered(t, gate.entered)
	local := ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(1, syncOrderPrefix), 26)
	if err := f.destination.session.SendUpdate(ownershipWriterUpdate(t, local)); err != nil {
		t.Fatal(err)
	}
	recipientIdentityEvict(t, f, f.source, 7)
	recipientIdentityEvict(t, f, f.source, 8)
	fwdPathIDs.mu.Lock()
	fwdPathIDs.next = 0
	fwdPathIDs.mu.Unlock()
	gate.release()
	events := recipientIdentityDrain(t, f)
	if len(events) != 3 || events[0].withdraw || events[0].path != 1 || events[1].withdraw || events[2].withdraw {
		t.Fatalf("local and two forwarded advertisements = %+v", events)
	}
	if events[1].path != 0 || events[2].path == events[1].path || events[2].path == events[0].path {
		t.Fatalf("combined forward reused a staged or locally owned identifier: %+v", events)
	}
	firstID, secondID := events[1].path, events[2].path
	events = recipientIdentitySend(t, f, f.source, 7, true, 0)
	if len(events) != 4 || !events[3].withdraw || events[3].path != firstID {
		t.Fatalf("first ingress path lost its withdrawal identity: %+v", events)
	}
	events = recipientIdentitySend(t, f, f.source, 8, true, 0)
	if len(events) != 5 || !events[4].withdraw || events[4].path != secondID {
		t.Fatalf("second ingress path lost its withdrawal identity: %+v", events)
	}
	localWithdrawal := ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(1, syncOrderPrefix), 0)
	if err := f.destination.session.SendUpdate(ownershipWriterUpdate(t, localWithdrawal)); err != nil {
		t.Fatal(err)
	}
	events = recipientIdentityDrain(t, f)
	if len(events) != 6 || !events[5].withdraw || events[5].path != 1 {
		t.Fatalf("forwarded withdrawals removed the local owner: %+v", events)
	}
	f.destination.adjOut.mu.Lock()
	bindings, routes := len(f.destination.adjOut.sourcePaths), len(f.destination.adjOut.routes)
	f.destination.adjOut.mu.Unlock()
	if bindings != 0 || routes != 0 {
		t.Fatalf("combined withdrawals left %d source bindings and %d advertisements", bindings, routes)
	}
	fwdPathIDs.mu.RLock()
	remaining := len(fwdPathIDs.used)
	fwdPathIDs.mu.RUnlock()
	if remaining != 0 {
		t.Fatalf("combined withdrawals left %d identifier reservations", remaining)
	}
}

// TestRecipientAddPathLocalReplacementRemovesBinding overwrites a forwarded
// slot through the direct local writer. Its old source must no longer resolve
// that slot through the derived index or withdraw the new local owner.
func TestRecipientAddPathLocalReplacementRemovesBinding(t *testing.T) {
	f := recipientIdentityRail(t, nil)
	events := recipientIdentitySend(t, f, f.source, 7, false, 31)
	if len(events) != 1 {
		t.Fatalf("initial advertisement = %+v", events)
	}
	local := ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(events[0].path, syncOrderPrefix), 32)
	if err := f.destination.session.SendUpdate(ownershipWriterUpdate(t, local)); err != nil {
		t.Fatal(err)
	}
	events = recipientIdentitySend(t, f, f.source, 7, true, 0)
	if len(events) != 2 || events[1].withdraw {
		t.Fatalf("old source withdrew a direct local replacement: %+v", events)
	}
	f.destination.adjOut.mu.Lock()
	bindings := len(f.destination.adjOut.sourcePaths)
	f.destination.adjOut.mu.Unlock()
	if bindings != 0 {
		t.Fatalf("local replacement retained %d source bindings", bindings)
	}
}

// TestRecipientAddPathReservationsEndWithSession covers clear, failed writer and
// replacement-session boundaries after real cache eviction. No old source index
// or reservation may survive the advertised state that owned it.
func TestRecipientAddPathReservationsEndWithSession(t *testing.T) {
	for _, boundary := range []string{"clear", "write-failure", "replacement-session"} {
		t.Run(boundary, func(t *testing.T) {
			f := recipientIdentityRail(t, nil)
			events := recipientIdentitySend(t, f, f.source, 7, false, 41)
			if len(events) != 1 {
				t.Fatalf("initial advertisement = %+v", events)
			}
			oldID := events[0].path
			recipientIdentityEvict(t, f, f.source, 7)
			switch boundary {
			case "clear":
				f.destination.adjOut.reset()
			case "write-failure":
				s := f.destination.session
				s.writeMu.Lock()
				s.retireWrite(errors.New("recipient identity test write failure"))
				s.writeMu.Unlock()
			case "replacement-session":
				next, _ := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.ctxID)
				f.destination.mu.Lock()
				f.destination.session = next.session
				f.destination.session.adjOut = &f.destination.adjOut
				f.destination.mu.Unlock()
				local := ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(19, syncOrderPrefix), 42)
				if err := f.destination.session.SendUpdate(ownershipWriterUpdate(t, local)); err != nil {
					t.Fatal(err)
				}
			}
			f.destination.adjOut.mu.Lock()
			bindings := len(f.destination.adjOut.sourcePaths)
			f.destination.adjOut.mu.Unlock()
			if bindings != 0 {
				t.Fatalf("session boundary retained %d old source bindings", bindings)
			}
			fwdPathIDs.mu.RLock()
			_, retained := fwdPathIDs.used[oldID]
			fwdPathIDs.mu.RUnlock()
			if retained {
				t.Fatal("session boundary retained the retired path identifier")
			}
		})
	}
}

// TestRecipientAddPathCollisionReservationReleasedOnRefusal fills PATHS-LIMIT
// with a local path after a forward was materialized. The refused collision
// must release its temporary ID; withdrawing the local path permits a retry.
func TestRecipientAddPathCollisionReservationReleasedOnRefusal(t *testing.T) {
	gate := ownershipRailNewGate()
	f := recipientIdentityRail(t, gate)
	s := f.destination.session
	s.writeMu.Lock()
	s.initPathsLimit(&capability.EncodingCaps{
		AddPathMode:    map[family.Family]capability.AddPathMode{family.IPv4Unicast: capability.AddPathSend},
		PathsLimitSend: map[family.Family]uint16{family.IPv4Unicast: 1},
	})
	s.writeMu.Unlock()
	gate.armed.Store(true)
	body := ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(7, syncOrderPrefix), 51)
	f.send(t, "cached", f.source, f.ctxID, body)
	awaitEntered(t, gate.entered)
	local := ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(0, syncOrderPrefix), 52)
	if err := s.SendUpdate(ownershipWriterUpdate(t, local)); err != nil {
		t.Fatal(err)
	}
	gate.release()
	events := recipientIdentityDrain(t, f)
	if len(events) != 1 || events[0].path != 0 {
		t.Fatalf("PATHS-LIMIT did not preserve only the local path: %+v", events)
	}
	fwdPathIDs.mu.RLock()
	identifiers, references := len(fwdPathIDs.used), fwdPathIDs.used[0]
	fwdPathIDs.mu.RUnlock()
	if identifiers != 1 || references != 2 {
		t.Fatalf("refused collision reservations = %d identifiers, %d zero references; want one identifier held by source mapping and local slot", identifiers, references)
	}
	localWithdrawal := ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(0, syncOrderPrefix), 0)
	if err := s.SendUpdate(ownershipWriterUpdate(t, localWithdrawal)); err != nil {
		t.Fatal(err)
	}
	events = recipientIdentitySend(t, f, f.source, 7, false, 53)
	if len(events) != 3 || !events[1].withdraw || events[2].withdraw {
		t.Fatalf("removing the local path did not permit the forward retry: %+v", events)
	}
}
