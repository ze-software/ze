package l2tpauthradius

import (
	"maps"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/radius"
)

func coaMixedFixture(t *testing.T) (string, []byte, *coaFixtureOwners) {
	t.Helper()
	l2tp.PublishService(oneSessionService())
	t.Cleanup(func() { l2tp.PublishService(nil) })
	dispatch := coaFixtureDispatch(t)
	value, loaded := coaFixtureOwnersByTest.Load(t)
	if !loaded {
		t.Fatal("registered authorization owners did not start")
	}
	owners, ok := value.(*coaFixtureOwners)
	if !ok {
		t.Fatalf("registered authorization owners are %T, want *coaFixtureOwners", value)
	}
	session := &subscriber.Session{
		ID: "coa-mixed-owners", AccessType: subscriber.AccessL2TP, Username: "alice",
		TunnelID: 10, SessionID: 20, AcctSessionID: "10-20-1",
		PppInterface: coaFixtureInterface(10, 20), AccessInterface: "eth-coa.20",
	}
	subscriber.DefaultRegistry.Add(session)
	t.Cleanup(func() { subscriber.DefaultRegistry.Remove(session.ID) })
	secret := []byte("test-coa-mixed-owners")
	cl, err := newCoAListener(coaListenerConfig{
		AllowedSources: coaLoopbackSources(), DefaultSecret: secret,
		Bus: &recordingBus{}, Dispatch: dispatch,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl.conn.LocalAddr().String(), secret, owners
}

func mixedCoAAttributes() []radius.Attr {
	return []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("rate:20mbit/8mbit")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("cos:gold")},
	}
}

// TestCoAMixedOwnerFailureRestoresBothControls drives a partial VLAN-map failure
// after the rate owner succeeded and reads both backends after the authenticated
// UDP NAK, including the original upload policer.
// RFC requirement: RFC5176-2.3-6 negative -- when the VLAN owner fails after a
// rate owner succeeds, the signed UDP CoA-NAK leaves both backend rate directions
// and both VLAN maps at their original values, including after a partial write.
func TestCoAMixedOwnerFailureRestoresBothControls(t *testing.T) {
	addr, secret, owners := coaMixedFixture(t)
	owners.mapsBackend.mu.Lock()
	owners.mapsBackend.failWrites = 1
	owners.mapsBackend.mu.Unlock()
	response := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, mixedCoAAttributes())
	wantNAK(t, response, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
	qos, err := owners.rateBackend.ListQdiscs(coaFixtureInterface(10, 20))
	if err != nil {
		t.Fatal(err)
	}
	if qos.Qdisc.RateBps != 10_000_000 || qos.Ingress.RateBps != 5_000_000 {
		t.Errorf("NAK left changed rate state: %+v", qos)
	}
	link, err := owners.mapsBackend.GetLink("eth-coa.20")
	if err != nil {
		t.Fatal(err)
	}
	if len(link.IngressQoSMap) != 0 || len(link.EgressQoSMap) != 0 {
		t.Errorf("NAK left changed VLAN maps: %+v", link)
	}
}

// TestCoAMixedOwnersApplyBeforeACK preserves the useful mixed-control case and
// reads the installed rate pair and VLAN maps after the authenticated UDP ACK.
// RFC requirement: RFC5176-2.3-6 positive -- a signed CoA requesting a rate pair
// and CoS profile for one subscriber receives CoA-ACK with both requested rate
// values and both requested VLAN maps installed in the real owners' backends.
func TestCoAMixedOwnersApplyBeforeACK(t *testing.T) {
	addr, secret, owners := coaMixedFixture(t)
	response := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, mixedCoAAttributes())
	if response.Code != radius.CodeCoAACK {
		t.Fatalf("valid mixed authorization response: got %d, want CoA-ACK", response.Code)
	}
	qos, err := owners.rateBackend.ListQdiscs(coaFixtureInterface(10, 20))
	if err != nil {
		t.Fatal(err)
	}
	if qos.Qdisc.RateBps != 20_000_000 || qos.Ingress.RateBps != 8_000_000 {
		t.Errorf("ACK lacks the requested rate pair: %+v", qos)
	}
	link, err := owners.mapsBackend.GetLink("eth-coa.20")
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(link.IngressQoSMap, map[uint32]uint32{1: 5}) || !maps.Equal(link.EgressQoSMap, map[uint32]uint32{5: 1}) {
		t.Errorf("ACK lacks the requested VLAN maps: %+v", link)
	}
}
