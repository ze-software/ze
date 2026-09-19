package wire

import (
	"bytes"
	"slices"
	"testing"
)

func TestPayloadNotifyRoundtrip(t *testing.T) {
	notifyData := make([]byte, 20)
	for i := range notifyData {
		notifyData[i] = byte(i + 50)
	}
	p := PayloadNotify{
		ProtocolID:       0,
		SPISize:          0,
		NotifyMsgType:    NotifyNATDetectionSourceIP,
		NotificationData: notifyData,
	}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadNotify
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.ProtocolID != 0 {
		t.Errorf("ProtocolID = %d, want 0", got.ProtocolID)
	}
	if got.NotifyMsgType != NotifyNATDetectionSourceIP {
		t.Errorf("NotifyMsgType = %d, want %d", got.NotifyMsgType, NotifyNATDetectionSourceIP)
	}
	if !bytes.Equal(got.NotificationData, notifyData) {
		t.Error("NotificationData mismatch")
	}
}

func TestPayloadNotifyWithSPI(t *testing.T) {
	p := PayloadNotify{
		ProtocolID:    ProtocolESP,
		SPISize:       4,
		NotifyMsgType: NotifyRekeySA,
		SPI:           []byte{0xAA, 0xBB, 0xCC, 0xDD},
	}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadNotify
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.ProtocolID != ProtocolESP {
		t.Errorf("ProtocolID = %d, want %d", got.ProtocolID, ProtocolESP)
	}
	if got.SPISize != 4 {
		t.Errorf("SPISize = %d, want 4", got.SPISize)
	}
	if !bytes.Equal(got.SPI, p.SPI) {
		t.Errorf("SPI = %x, want %x", got.SPI, p.SPI)
	}
}

// TestPrivateNotifyTypesAreStatusTypes holds every private-use member of the
// notify registry to the one rule that makes registering it legal.
//
// It lives in this package, beside the map, because the obligation is a property
// of the REGISTRY and not of any caller. Asserting it from a consumer would need
// the set exported, and would only judge the members that consumer happens to
// reference; this judges all of them, including one registered today and first
// transmitted next month.
//
// It carries no conformance tag, deliberately. RFC7296-2.21.2-3 already
// has a positive and a negative carrier in
// internal/component/ike/engine/notify_error_test.go, so a third would add a
// claim to the public ledger without adding a proven requirement to it, and a
// tag owes a recorded discrimination break of its own. This guard earns its
// place by being STRICTER than the ledger asks, not by appearing on it.
//
// The obligation it enforces is still RFC 7296 Section 2.21.2: "Extension
// documents may define new error notifications with these semantics, but MUST
// NOT use them unless the peer has been shown to understand them, such as by
// using the Vendor ID payload." Ze sends no Vendor ID, so it may register a
// private-use notify type only as a STATUS type, which Section 3.10.1 has an
// unrecognized peer ignore instead.
//
// VALIDATES: no member of notifyTypePrivate is an error type, and no member
// answers RFC-defined.
// PREVENTS: the hole that opened when the path probe's private status type was
// registered. Until then the registry held exactly the RFC-defined types, so
// NotifyTypeRecognized doubled as an RFC check and a consumer test read it as
// one. Adding a private member made those two sets differ in silence: the
// consumer test kept passing while its conclusion stopped following, and a
// private-use ERROR type added the same way would have inherited the same green.
func TestPrivateNotifyTypesAreStatusTypes(t *testing.T) {
	// Pinned rather than skipped-when-empty. A loop over an empty set passes
	// against any implementation, which is the vacuous green this repository
	// bans, and the membership is itself the thing worth pinning: adding or
	// removing a private-use type is a deliberate act that owes a reader a
	// reason, so it reddens here first.
	private := notifyPrivateTypes()
	want := []uint16{NotifyZePathProbePadding}
	if !slices.Equal(private, want) {
		t.Fatalf("the private-use notify set is %v, want %v. A member added here is "+
			"a type RFC 7296 does not define: say why it is one ze may send, and check "+
			"it against the status-type rule below", private, want)
	}

	for _, value := range private {
		if NotifyIsError(value) {
			t.Errorf("private-use notify type %d (%s) is an ERROR type. RFC 7296 Section "+
				"2.21.2 forbids ze using an extension error notification unless the peer "+
				"has been shown to understand it, and ze sends no Vendor ID payload",
				value, NotifyTypeName(value))
		}
		if notifyRFCDefined(value) {
			t.Errorf("notify type %d is listed private-use and also answers RFC-defined, "+
				"so the two sets disagree and neither can be trusted", value)
		}
		if !NotifyTypeRecognized(value) {
			t.Errorf("private-use notify type %d is not in the registry at all, so nothing "+
				"can spell it in a log line, which is the only reason to list it", value)
		}
	}
}
