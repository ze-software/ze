package l2tpauthradius

import (
	"context"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// coaIncarnationFixture publishes the same live tuple through the subscriber
// registry and the L2TP snapshot, with the exact full accounting id of its new
// incarnation. The real shaper owns the snapshot's PPP interface.
func coaIncarnationFixture(t *testing.T) (string, []byte, *coaApplyBackend) {
	t.Helper()
	addr, secret, backend := coaAppliedFixture(t)
	session, found := subscriber.DefaultRegistry.Get("coa-apply-regression")
	if !found {
		t.Fatal("fixture has no live subscriber")
	}
	session.AcctSessionID = "10-20-new"
	subscriber.DefaultRegistry.Add(&session)
	service := oneSessionService()
	service.snap.Tunnels[0].Sessions[0].PppInterface = session.PppInterface
	l2tp.PublishService(service)
	return addr, secret, backend
}

// TestCoAOldAccountingIdentityCannotSelectNewIncarnation sends a signed request
// whose numeric tuple and other selectors match a live session but whose full
// accounting id belongs to its old incarnation. The snapshot fallback must not
// reinterpret that complete id as a tuple prefix.
// RFC requirement: RFC5176-3.3-1 negative -- an old complete accounting id cannot
// select the live registry and L2TP snapshot's new incarnation of the same
// numeric tuple: signed CoA-NAK 503 leaves both backend rate directions unchanged.
func TestCoAOldAccountingIdentityCannotSelectNewIncarnation(t *testing.T) {
	addr, secret, backend := coaIncarnationFixture(t)
	response := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-old")},
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
		{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")},
	})
	wantNAK(t, response, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)
	wantCoAInitialRate(t, backend)
	qos, err := backend.ListQdiscs("ppp-coa-apply")
	if err != nil {
		t.Fatal(err)
	}
	if qos.Ingress.RateBps != 5_000_000 {
		t.Errorf("old accounting identity changed the upload policer: %+v", qos.Ingress)
	}
}

// TestDisconnectOldAccountingIdentityCannotTerminateNewIncarnation keeps the
// same exact-identity guard on Disconnect's L2TP snapshot route.
// RFC requirement: RFC5176-3.3-1 negative -- an old complete accounting id cannot
// terminate the new incarnation of the same L2TP tuple: signed Disconnect-NAK 503
// leaves the actual session teardown count at zero.
func TestDisconnectOldAccountingIdentityCannotTerminateNewIncarnation(t *testing.T) {
	addr, secret, backend := coaIncarnationFixture(t)
	service, valid := l2tp.LookupService().(*fakeL2TPService)
	if !valid {
		t.Fatal("fixture has no published L2TP snapshot")
	}
	response := sendAppliedCoA(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-old")},
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
	})
	wantNAK(t, response, radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotFound)
	wantCoAInitialRate(t, backend)
	if got := service.teardowns.Load(); got != 0 {
		t.Errorf("old accounting identity terminated %d current sessions", got)
	}
}

// TestCoALiveAccountingIdentityStillApplies proves the exact current identity
// can select the same live tuple and install the requested rate.
func TestCoALiveAccountingIdentityStillApplies(t *testing.T) {
	addr, secret, backend := coaIncarnationFixture(t)
	response := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-new")},
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
		{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")},
	})
	if response.Code != radius.CodeCoAACK {
		t.Fatalf("live accounting identity response: got %d, want CoA-ACK", response.Code)
	}
	qos, err := backend.ListQdiscs("ppp-coa-apply")
	if err != nil {
		t.Fatal(err)
	}
	if qos.Qdisc.RateBps != 20_000_000 {
		t.Errorf("live identity ACK lacks the requested rate: %+v", qos.Qdisc)
	}
}

// TestCoALiveAccountingOwnerIdentitySurvivesRegistryMiss uses the complete id
// actually held by the accountant while the subscriber registry holds its own
// different session identifier. Refusing every registry miss would lose this
// normal RADIUS path along with the stale-id defect.
// RFC requirement: RFC5176-3.3-1 positive -- the actual accountant's complete
// issued id, User-Name and NAS-Port select a live L2TP session despite a different
// registry identifier; signed CoA-ACK accompanies an installed 20M download rate.
func TestCoALiveAccountingOwnerIdentitySurvivesRegistryMiss(t *testing.T) {
	addr, secret, backend := coaIncarnationFixture(t)
	seedCoAAccountingIdentity(t, &acctSession{
		tunnelID: 10, sessionID: 20, username: "alice",
		acctSessID: "10-20-radius-current", pppInterface: "ppp-coa-apply",
	})
	response := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-radius-current")},
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
		{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")},
	})
	if response.Code != radius.CodeCoAACK {
		t.Fatalf("actual accounting owner identity response: got %d, want CoA-ACK", response.Code)
	}
	qos, err := backend.ListQdiscs("ppp-coa-apply")
	if err != nil {
		t.Fatal(err)
	}
	if qos.Qdisc.RateBps != 20_000_000 {
		t.Errorf("accounting owner ACK lacks the requested rate: %+v", qos.Qdisc)
	}
}

// seedCoAAccountingIdentity installs an explicit complete accounting fixture id
// and restores the prior tuple at cleanup. It models the accountant's retained
// issued value, rather than teaching production to derive an id from a tuple.
func seedCoAAccountingIdentity(t *testing.T, accounting *acctSession) {
	t.Helper()
	_, cancel := context.WithCancel(t.Context())
	accounting.cancel = cancel
	accounting.startTime = time.Now()
	key := sessionKey{tunnelID: accounting.tunnelID, sessionID: accounting.sessionID}
	acctInstance.mu.Lock()
	previous := acctInstance.sessions[key]
	acctInstance.sessions[key] = accounting
	acctInstance.mu.Unlock()
	t.Cleanup(func() {
		acctInstance.mu.Lock()
		if acctInstance.sessions[key] == accounting {
			if previous == nil {
				delete(acctInstance.sessions, key)
			} else {
				acctInstance.sessions[key] = previous
			}
		}
		acctInstance.mu.Unlock()
		cancel()
	})
}

// seedCoAServiceAccounting supplies the explicit complete ids used by the
// legacy socket fixtures, including Disconnect fixtures that need no shaper.
// The snapshots alone never supply this authority to production code.
func seedCoAServiceAccounting(t *testing.T) {
	t.Helper()
	service := l2tp.LookupService()
	if service == nil {
		return
	}
	snapshot := service.Snapshot()
	for i := range snapshot.Tunnels {
		for j := range snapshot.Tunnels[i].Sessions {
			session := &snapshot.Tunnels[i].Sessions[j]
			var id textbuf.Buffer
			seedCoAAccountingIdentity(t, &acctSession{
				tunnelID: snapshot.Tunnels[i].LocalTID, sessionID: session.LocalSID,
				username: session.Username, pppInterface: session.PppInterface,
				acctSessID: id.Uint16(snapshot.Tunnels[i].LocalTID).Byte('-').Uint16(session.LocalSID).Str("-1").String(),
			})
		}
	}
}
