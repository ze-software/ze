package l2tpauthradius

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/radius"
)

// TestDynamicAuthorizationRefusesUnimplementedIdentification sends valid wire
// encodings of every identification attribute this listener cannot evaluate.
// Each request also carries the three working selectors, so ignoring the extra
// selector reaches an actual session and cannot pass this test by dropping all
// requests. Both the backend state and the session teardown count must survive.
// RFC requirement: RFC5176-2.3-5 negative -- each of the eleven unimplemented
// identification types in a signed CoA or Disconnect is refused with NAK 401;
// both backend rate directions, the registry entry and the teardown count stay
// unchanged even though all three implemented selectors identify a live session.
func TestDynamicAuthorizationRefusesUnimplementedIdentification(t *testing.T) {
	selectors := []struct {
		name string
		attr radius.Attr
	}{
		{"nas-ip-address", radius.Attr{Type: radius.AttrNASIPAddress, Value: net.IPv4(192, 0, 2, 1).To4()}},
		{"nas-ipv6-address", radius.Attr{Type: radius.AttrNASIPv6Address, Value: net.ParseIP("2001:db8::1").To16()}},
		{"nas-identifier", radius.Attr{Type: radius.AttrNASIdentifier, Value: radius.AttrString("other-nas")}},
		{"framed-ip-address", radius.Attr{Type: radius.AttrFramedIPAddress, Value: net.IPv4(192, 0, 2, 20).To4()}},
		{"framed-interface-id", radius.Attr{Type: radius.AttrFramedInterfaceID, Value: []byte{0, 0, 0, 0, 0, 0, 0, 20}}},
		{"framed-ipv6-prefix", radius.Attr{Type: radius.AttrFramedIPv6Prefix, Value: []byte{0, 64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 1}}},
		{"called-station-id", radius.Attr{Type: radius.AttrCalledStationID, Value: radius.AttrString("00-11-22-33-44-55")}},
		{"calling-station-id", radius.Attr{Type: radius.AttrCallingStationID, Value: radius.AttrString("00-11-22-33-44-66")}},
		{"acct-multi-session-id", radius.Attr{Type: radius.AttrAcctMultiSessionID, Value: radius.AttrString("multi-10")}},
		{"nas-port-id", radius.Attr{Type: radius.AttrNASPortID, Value: radius.AttrString("port-20")}},
		{"chargeable-user-identity", radius.Attr{Type: radius.AttrChargeableUserID, Value: radius.AttrString("alice-charge")}},
	}
	for _, operation := range []struct {
		name string
		code uint8
	}{
		{"coa", radius.CodeCoARequest},
		{"disconnect", radius.CodeDisconnectRequest},
	} {
		for _, selector := range selectors {
			t.Run(operation.name+"/"+selector.name, func(t *testing.T) {
				addr, secret, backend := coaAppliedFixture(t)
				service, valid := l2tp.LookupService().(*fakeL2TPService)
				if !valid {
					t.Fatal("fixture did not publish its session service")
				}
				attrs := []radius.Attr{
					{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
					{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
					{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
					selector.attr,
				}
				if operation.code == radius.CodeCoARequest {
					attrs = append(attrs, radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")})
				}
				response := sendAppliedCoA(t, addr, operation.code, secret, attrs)
				wantNAK(t, response, nakCode(operation.code), radius.ErrorCauseUnsupportedAttribute)
				wantCoAInitialRate(t, backend)
				qos, err := backend.ListQdiscs("ppp-coa-apply")
				if err != nil {
					t.Fatal(err)
				}
				if qos.Ingress.RateBps != 5_000_000 {
					t.Errorf("unsupported selector changed the upload policer: %+v", qos.Ingress)
				}
				if got := service.teardowns.Load(); got != 0 {
					t.Errorf("unsupported selector terminated %d sessions", got)
				}
				if _, found := subscriber.DefaultRegistry.Get("coa-apply-regression"); !found {
					t.Error("unsupported selector removed the registered subscriber")
				}
			})
		}
	}
}

// TestCoAImplementedIdentificationStillApplies retains a useful accepted request
// beside the unsupported-selector refusals and reads the backend's actual rate.
// RFC requirement: RFC5176-2.3-5 positive -- the three implemented identification
// attributes and an explicit supported rate pair receive signed CoA-ACK, with
// both requested rate values installed in the actual shaping backend.
func TestCoAImplementedIdentificationStillApplies(t *testing.T) {
	addr, secret, backend := coaAppliedFixture(t)
	response := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
		{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit/5mbit")},
	})
	if response.Code != radius.CodeCoAACK {
		t.Fatalf("supported selectors response: got %d, want CoA-ACK", response.Code)
	}
	qos, err := backend.ListQdiscs("ppp-coa-apply")
	if err != nil {
		t.Fatal(err)
	}
	if qos.Qdisc.RateBps != 20_000_000 {
		t.Errorf("ACK lacks the requested download rate: %+v", qos.Qdisc)
	}
	if qos.Ingress.RateBps != 5_000_000 {
		t.Errorf("ACK lacks the requested upload policer: %+v", qos.Ingress)
	}
}
