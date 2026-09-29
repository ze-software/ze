// Design: docs/guide/l2tp.md -- CoA/DM listener
// RFC: rfc/short/rfc5176.md -- Section 2.3 unsupported Attribute values and atomic CoA
// Related: coa.go -- coaChange, unsupportedAttrValue, applySubscriberCoA
// Related: rfc5176_walk_test.go -- coaTestListener, recordingBus, wantNAK

// RFC 5176 Section 2.3 makes every attribute of a CoA-Request or a
// Disconnect-Request mandatory, so a value the NAS cannot act on is refused even
// when a supported value rides beside it. It also makes a CoA-Request atomic:
// every check is made before the first change leaves. These tests drive both
// through the real listener socket.
package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/radius"
)

// subscriberSession adds an L2TP subscriber session whose Acct-Session-Id is
// "10-20-1" to the process registry, and removes it when the test ends.
func subscriberSession(t *testing.T, id, accessInterface string) {
	t.Helper()
	svc := subscriber.LookupService()
	if svc == nil {
		t.Fatal("no subscriber service")
	}
	svc.Registry.Add(&subscriber.Session{
		ID:              id,
		AccessType:      subscriber.AccessL2TP,
		AccessInterface: accessInterface,
		TunnelID:        10,
		SessionID:       20,
		AcctSessionID:   "10-20-1",
	})
	t.Cleanup(func() { svc.Registry.Remove(id) })
}

// vsa answers a Vendor-Specific attribute, failing the test if it cannot be
// encoded. EncodeVSA writes the outer Type and Length octets, which the
// attribute value does not carry.
func vsa(t *testing.T, vendorID uint32, vendorType uint8, value string) radius.Attr {
	t.Helper()
	raw, err := radius.EncodeVSA(vendorID, vendorType, []byte(value))
	if err != nil {
		t.Fatal(err)
	}
	return radius.Attr{Type: radius.AttrVendorSpecific, Value: raw[2:]}
}

// RFC requirement: RFC5176-2.3-5 positive -- a CoA-Request carrying a supported
// rate Filter-Id beside an attribute value the NAS does not support (a second
// Filter-Id that is neither a rate nor a CoS profile, a second rate, or a
// Vendor-Specific of a vendor the NAS does not read) is answered with a CoA-NAK
// carrying Error-Cause 407, and the supported change does not reach the event bus.
func TestRFC5176CoAWithOneUnsupportedValueBesideSupportedOnesIsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-mixed-values-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, oneSessionService())

	for _, tc := range []struct {
		name  string
		extra radius.Attr
	}{
		{"filter-id not-a-rate", radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("not-a-rate")}},
		{"second rate filter-id", radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")}},
		{"unknown vendor", vsa(t, 99999, 1, "anything")},
	} {
		resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
			{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
			tc.extra,
		})
		if resp.Code != radius.CodeCoANAK {
			t.Errorf("%s: code %d, want %d (CoA-NAK)", tc.name, resp.Code, radius.CodeCoANAK)
			continue
		}
		wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseInvalidAttributeValue)
	}
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("events emitted behind a CoA-NAK: got %d, want 0", got)
	}
}

// RFC requirement: RFC5176-2.3-5 positive -- a Disconnect-Request carrying a
// NAS-Port whose value is not four octets is answered with a Disconnect-NAK
// carrying Error-Cause 407, and one carrying a Vendor-Specific attribute, which
// this NAS reads in no Disconnect-Request, with a Disconnect-NAK carrying
// Error-Cause 401. Neither tears the session down.
func TestRFC5176DisconnectWithAnUnsupportedValueIsNAKed(t *testing.T) {
	secret := []byte("test-rfc5176-dm-values-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, nil, fake)

	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrNASPort, Value: []byte{0, 20}},
	})
	wantNAK(t, resp, radius.CodeDisconnectNAK, radius.ErrorCauseInvalidAttributeValue)

	resp = sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		vsa(t, radius.VendorMikrotik, radius.MikrotikRateLimit, "10M/10M"),
	})
	wantNAK(t, resp, radius.CodeDisconnectNAK, radius.ErrorCauseUnsupportedAttribute)

	if got := fake.teardowns.Load(); got != 0 {
		t.Errorf("teardowns for a refused Disconnect-Request: got %d, want 0", got)
	}
}

// RFC requirement: RFC5176-2.3-5 negative -- a CoA-Request whose every value is
// supported (a rate Filter-Id and a CoS Filter-Id) is answered with a CoA-ACK, and
// a Disconnect-Request whose NAS-Port is four octets naming the session is
// answered with a Disconnect-ACK, so the NAKs above are specific to the value.
// RFC requirement: RFC5176-2.3-6 negative -- the same CoA-Request against a
// subscriber session that has an access interface is ACKed with every requested
// change made: two rate-change events and one CoS-change event.
func TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed(t *testing.T) {
	secret := []byte("test-rfc5176-all-supported-secret")
	bus := &recordingBus{}
	fake := oneSessionService()
	addr := coaTestListener(t, secret, bus, fake)
	subscriberSession(t, "sub-rfc5176-supported", "l2tp-ppp0")

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("cos:gold")},
	})
	if resp.Code != radius.CodeCoAACK {
		t.Errorf("CoA code %d, want %d (CoA-ACK)", resp.Code, radius.CodeCoAACK)
	}
	if got := len(bus.recorded()); got != 3 {
		t.Errorf("events emitted: got %d, want 3 (subscriber rate, l2tp rate, cos)", got)
	}

	var nasPort [4]byte
	nasPort[3] = 20
	resp = sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrNASPort, Value: nasPort[:]},
	})
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("Disconnect code %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns: got %d, want 1", got)
	}
}

// RFC requirement: RFC5176-2.3-6 positive -- a CoA-Request asking for a rate and
// a CoS profile on a subscriber session with no access interface cannot be carried
// out whole: it is answered with a CoA-NAK carrying Error-Cause 506, and the rate
// change it could have made alone does not reach the event bus.
func TestRFC5176SubscriberCoAThatCannotBeCarriedOutWholeMakesNoChange(t *testing.T) {
	secret := []byte("test-rfc5176-sub-atomic-secret")
	bus := &recordingBus{}
	addr := coaTestListener(t, secret, bus, oneSessionService())
	subscriberSession(t, "sub-rfc5176-no-interface", "")

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("cos:gold")},
	})
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
	if got := len(bus.recorded()); got != 0 {
		t.Errorf("events emitted behind a CoA-NAK: got %d, want 0", got)
	}
}
