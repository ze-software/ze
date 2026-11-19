// Design: docs/architecture/l2tp/bng-5-pppoe.md -- exact retained ownership
// VALIDATES: completed absence, same-connection cleanup, and exact ledger release.
package vpp

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/rtproto"
	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ethernet_types"
	"go.fd.io/govpp/binapi/memclnt"
)

func isolateReconciliation(t *testing.T) {
	t.Helper()
	macReservations.Lock()
	previous := macReservations.held
	macReservations.held = make(map[[6]byte]*macReservation)
	macReservations.Unlock()
	previousMaps := preparedVPPMaps
	preparedVPPMaps = newControlMapOwners()
	report.ResetForTest()
	t.Cleanup(func() {
		macReservations.Lock()
		macReservations.held = previous
		macReservations.Unlock()
		preparedVPPMaps = previousMaps
		report.ResetForTest()
	})
}

func retainReconciliation(t *testing.T, client *binaryAPI, sid uint16, owner *session) retainedSession {
	t.Helper()
	mac := [6]byte{2, 0, 0, 0, 0, byte(sid)}
	reservation, err := reserveMAC(mac, client)
	if err != nil {
		t.Fatal(err)
	}
	pending := pendingMutation{api: client, request: dataplane.Request{AccessName: "wan", PeerMAC: mac, SessionID: sid},
		table: 41, access: 11, local: [6]byte{2, 0, 0, 0, 0, 9}, session: owner, failure: errors.New("retained mutation")}
	reservation.retain(pending)
	return retainedSession{reservation, reservation.pending}
}

type reconciliationQuery struct {
	api.Stream
	details []*pppoeapi.PppoeSessionDetails
	query   bool
	replies []api.Message
}

func (s *reconciliationQuery) SendMsg(message api.Message) error {
	if _, ok := message.(*pppoeapi.PppoeSessionDump); ok {
		s.query = true
		for _, detail := range s.details {
			s.replies = append(s.replies, detail)
		}
		return nil
	}
	if _, ok := message.(*memclnt.ControlPing); ok && s.query {
		s.replies = append(s.replies, &memclnt.ControlPingReply{})
		return nil
	}
	return s.Stream.SendMsg(message)
}
func (s *reconciliationQuery) RecvMsg() (api.Message, error) {
	if !s.query {
		return s.Stream.RecvMsg()
	}
	if len(s.replies) == 0 {
		return nil, errors.New("query completion missing")
	}
	message := s.replies[0]
	s.replies = s.replies[1:]
	return message, nil
}

// Visible unknown state must stay reserved; an ordinary matching dump does
// not establish who created it or permit any native delete.
func TestVPPReconcileUnknownPresentDoesNotAdopt(t *testing.T) {
	isolateReconciliation(t)
	var streams []*testStream
	client := &binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		stream := &testStream{ctx: ctx, replies: []api.Message{&pppoeapi.PppoeSessionDetails{SessionID: 19, ClientMac: ethernet_types.MacAddress{2, 0, 0, 0, 0, 19}}, &memclnt.ControlPingReply{}}}
		streams = append(streams, stream)
		return stream, nil
	}}
	item := retainReconciliation(t, client, 19, nil)
	status, err := Reconcile(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if status.PendingSessions != 1 {
		t.Fatalf("status = %+v", status)
	}
	if macReservations.held[item.reservation.mac] != item.reservation {
		t.Fatal("visible unknown creation was released")
	}
	for _, stream := range streams {
		for _, message := range stream.sent {
			switch message.(type) {
			case *pppoeapi.PppoeSessionDump, *memclnt.ControlPing:
			default:
				t.Fatalf("unknown creation gained mutation authority: %T", message)
			}
		}
	}
	if len(checkReconciliation(diagnostic.DoctorCheckContext{})) == 0 {
		t.Fatal("retained creation disappeared from doctor")
	}
}

// A completed empty dump proves an unconfirmed creation is absent and frees
// only that reservation. An incomplete read leaves it reserved with its cause.
func TestVPPReconcileUnknownRequiresCompletedAbsence(t *testing.T) {
	for _, incomplete := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "incomplete"}[incomplete], func(t *testing.T) {
			isolateReconciliation(t)
			want := errors.New("dump completion lost")
			client := &binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
				stream := &testStream{ctx: ctx, replies: []api.Message{&memclnt.ControlPingReply{}}}
				if incomplete {
					stream.err = want
				}
				return stream, nil
			}}
			item := retainReconciliation(t, client, 19, nil)
			status, err := Reconcile(t.Context())
			if incomplete {
				if !errors.Is(err, want) {
					t.Fatalf("read error = %v", err)
				}
				if status.PendingSessions != 1 || macReservations.held[item.reservation.mac] != item.reservation {
					t.Fatal("incomplete read released ownership")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if status.PendingSessions != 0 || len(macReservations.held) != 0 {
					t.Fatal("completed absence retained a reservation")
				}
				if len(checkReconciliation(diagnostic.DoctorCheckContext{})) != 0 {
					t.Fatal("confirmed absence left a stale warning")
				}
			}
		})
	}
}

// One native dump serves every unknown reservation sharing its captured API;
// reconciliation must not scan the whole table once per subscriber.
func TestVPPReconcileSharesAbsenceRead(t *testing.T) {
	isolateReconciliation(t)
	dumps := 0
	client := &binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		dumps++
		return &testStream{ctx: ctx, replies: []api.Message{&memclnt.ControlPingReply{}}}, nil
	}}
	retainReconciliation(t, client, 19, nil)
	retainReconciliation(t, client, 20, nil)
	status, err := Reconcile(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if dumps != 1 || status.PendingSessions != 0 {
		t.Fatalf("dumps=%d, status=%+v", dumps, status)
	}
}

// Retry uses the original native owner after exact identity readback. It
// withdraws the retained path before freeing the interface, without reopening.
func TestVPPReconcileRetriesConfirmedOwnerInPlace(t *testing.T) {
	isolateReconciliation(t)
	wire := &ownershipWire{deleteRouteRetval: -1}
	owner := ownershipSession(t, wire)
	owner.mac = [6]byte{2, 0, 0, 0, 0, 19}
	route := sessionRoute{destination: netip.MustParsePrefix("192.0.2.7/32"), protocol: rtproto.Iface}
	owner.routes[route] = struct{}{}
	wire.paths = 1
	if err := owner.Close(); err == nil {
		t.Fatal("native withdrawal failure not retained")
	}
	open := owner.api.open
	owner.api.open = func(ctx context.Context) (api.Stream, error) {
		stream, err := open(ctx)
		if err != nil {
			return nil, err
		}
		return &reconciliationQuery{Stream: stream, details: []*pppoeapi.PppoeSessionDetails{{SwIfIndex: 73, EncapIfIndex: 11, SessionID: 19,
			ClientMac: ethernet_types.MacAddress(owner.mac), LocalMac: ethernet_types.MacAddress{2, 0, 0, 0, 0, 9}, DecapVrfID: 41}}}, nil
	}
	retainReconciliation(t, owner.api, 19, owner)
	wire.deleteRouteRetval = 0
	status, err := Reconcile(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if status.PendingSessions != 0 || wire.paths != 0 || wire.sessionDeletes != 1 || wire.unsafeRecycle {
		t.Fatalf("status=%+v, native=%+v", status, wire)
	}
	if err := owner.Close(); err != nil {
		t.Fatalf("closed alias retained obsolete native error: %v", err)
	}
}

// A reused native index is not a deletion receipt. Identity mismatch must
// refuse before admin, family, path or session cleanup can touch that index.
func TestVPPReconcileRefusesReusedNativeIndex(t *testing.T) {
	isolateReconciliation(t)
	wire := &ownershipWire{deleteRouteRetval: -1}
	owner := ownershipSession(t, wire)
	owner.mac = [6]byte{2, 0, 0, 0, 0, 19}
	owner.routes[sessionRoute{destination: netip.MustParsePrefix("192.0.2.7/32"), protocol: rtproto.Iface}] = struct{}{}
	wire.paths = 1
	if err := owner.Close(); err == nil {
		t.Fatal("native cleanup failure not retained")
	}
	before := wire.pathDeletes
	open := owner.api.open
	owner.api.open = func(ctx context.Context) (api.Stream, error) {
		stream, err := open(ctx)
		if err != nil {
			return nil, err
		}
		return &reconciliationQuery{Stream: stream, details: []*pppoeapi.PppoeSessionDetails{{SwIfIndex: 73, SessionID: 42}}}, nil
	}
	retainReconciliation(t, owner.api, 19, owner)
	status, err := Reconcile(t.Context())
	if err == nil || status.PendingSessions != 1 {
		t.Fatalf("foreign state = %+v,%v", status, err)
	}
	if wire.pathDeletes != before || wire.sessionDeletes != 0 || wire.paths != 1 {
		t.Fatal("reconciliation mutated a reused native index")
	}
}

// A completed old cleanup and a process retirement release exact pointers;
// neither may remove a replacement or another live API owner's reservation.
func TestVPPReconcileReleaseKeepsReplacement(t *testing.T) {
	isolateReconciliation(t)
	oldAPI, otherAPI := &binaryAPI{}, &binaryAPI{}
	old := retainReconciliation(t, oldAPI, 19, nil)
	replacement := &macReservation{mac: old.reservation.mac, pending: &pendingMutation{api: otherAPI}}
	macReservations.held[old.reservation.mac] = replacement
	releaseRetainedSession(old)
	if macReservations.held[old.reservation.mac] != replacement {
		t.Fatal("old completion removed replacement")
	}
	retireCoveredSessions(func(client *binaryAPI) bool { return client == oldAPI })
	if macReservations.held[old.reservation.mac] != replacement {
		t.Fatal("other process retirement removed replacement")
	}
	retireCoveredSessions(func(client *binaryAPI) bool { return client == otherAPI })
	if len(macReservations.held) != 0 {
		t.Fatal("matching retired process remained reserved")
	}
}

// A transport epoch change cannot turn an earlier uncertain reciprocal map
// into a borrowed mapping, even when both numeric coordinates still match.
func TestVPPReconcileControlMapCrossEpochStaysReserved(t *testing.T) {
	isolateReconciliation(t)
	owners := newControlMapOwners()
	key := controlMapKey{generation: 1, pair: controlPair{11, 42}}
	owners.entries[key] = &controlMapUnconfirmed{failure: errors.New("add reply lost")}
	if err := owners.refuseOverlap(controlMapKey{generation: 3, pair: key.pair}); err == nil {
		t.Fatal("reconnect borrowed an uncertain old mapping")
	}
}

// Proven process retirement forgets native obligations without a delete and
// keeps active lease aliases releasable. Other process state remains reserved.
func TestVPPReconcileControlMapRetirementPreservesAliases(t *testing.T) {
	isolateReconciliation(t)
	owners := newControlMapOwners()
	retired := controlMapKey{generation: 1, pair: controlPair{11, 42}}
	other := controlMapKey{generation: 3, pair: controlPair{12, 43}}
	owners.entries[retired] = &controlMapLeaseState{owned: true, references: 1, api: &binaryAPI{open: func(context.Context) (api.Stream, error) {
		t.Fatal("retirement sent a native request")
		return nil, nil
	}}}
	owners.entries[other] = &controlMapUnconfirmed{failure: errors.New("another live mutation")}
	pending, err := owners.reconcileCovered(t.Context(), func(key controlMapKey) bool { return key == retired })
	if err != nil || pending != 1 {
		t.Fatalf("retirement=%d,%v", pending, err)
	}
	if err := (&controlMapLease{owners: owners, key: retired}).Close(); err != nil {
		t.Fatal(err)
	}
	if _, exists := owners.entries[retired]; exists {
		t.Fatal("retired final alias remained held")
	}
	if _, exists := owners.entries[other]; !exists {
		t.Fatal("other live mutation was erased")
	}
}

// A joined exact process retires the closed owner's native obligations and
// cached failure without sending even a readback to its disconnected API.
func TestVPPReconcileSessionRetirementClearsCachedNativeFailure(t *testing.T) {
	isolateReconciliation(t)
	wire := &ownershipWire{deleteRouteRetval: -1}
	owner := ownershipSession(t, wire)
	owner.mac = [6]byte{2, 0, 0, 0, 0, 19}
	owner.routes[sessionRoute{destination: netip.MustParsePrefix("192.0.2.7/32"), protocol: rtproto.Iface}] = struct{}{}
	wire.paths = 1
	if err := owner.Close(); err == nil {
		t.Fatal("native failure was not retained")
	}
	retainReconciliation(t, owner.api, 19, owner)
	owner.api.open = func(context.Context) (api.Stream, error) {
		t.Fatal("process retirement performed native I/O")
		return nil, nil
	}
	retireCoveredSessions(func(client *binaryAPI) bool { return client == owner.api })
	if !owner.api.processExited() {
		t.Fatal("exact local retirement was forgotten")
	}
	if len(owner.routes) != 0 {
		t.Fatal("departed native route remained owned")
	}
	if len(macReservations.held) != 0 {
		t.Fatal("departed subscriber remained reserved")
	}
	if err := owner.Close(); err != nil {
		t.Fatalf("closed alias retained native failure: %v", err)
	}
	if len(checkReconciliation(diagnostic.DoctorCheckContext{})) != 0 {
		t.Fatal("retirement left stale provider warning")
	}
}

// Only the covered map API is marked retired. Active aliases stay releasable
// while unknown maps from a different epoch remain reserved.
func TestVPPReconcileMapProcessMarksSharedAPIAliases(t *testing.T) {
	isolateReconciliation(t)
	owners := newControlMapOwners()
	oldAPI, otherAPI := &binaryAPI{}, &binaryAPI{}
	oldKey := controlMapKey{generation: 7, pair: controlPair{11, 42}}
	otherKey := controlMapKey{generation: 9, pair: controlPair{12, 43}}
	owners.entries[oldKey] = &controlMapLeaseState{api: oldAPI, owned: true, references: 1}
	owners.entries[otherKey] = &controlMapUnconfirmed{api: otherAPI, failure: errors.New("still live")}
	owners.retireCovered(func(key controlMapKey) bool { return key == oldKey })
	if !oldAPI.processExited() {
		t.Fatal("active session API did not retain its exact process proof")
	}
	if otherAPI.processExited() {
		t.Fatal("other epoch inherited process death")
	}
	if err := (&controlMapLease{owners: owners, key: oldKey}).Close(); err != nil {
		t.Fatal(err)
	}
	if _, exists := owners.entries[oldKey]; exists {
		t.Fatal("old alias did not release")
	}
	if _, exists := owners.entries[otherKey]; !exists {
		t.Fatal("other epoch reservation was erased")
	}
}
