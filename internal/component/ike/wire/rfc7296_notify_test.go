package wire

import (
	"bytes"
	"errors"
	"testing"
)

// RFC requirement: RFC7296-3.10-4 positive -- a notification that carries an SPI concerns a Child
// SA, so its Protocol ID octet holds AH (2) or ESP (3). RFC 7296 Section 3.10 states "For
// notifications concerning Child SAs, this field MUST contain either (2) to indicate AH or
// (3) to indicate ESP". Both values encode into octet 0 and survive a round trip.
//
// RFC requirement: RFC7296-3.10-4 negative -- no other Protocol ID is accepted beside an SPI. A
// notification that names IKE (1), or a reserved value, is refused with ErrNotifyProtocolID.
// Section 3.10 pairs this rule with a second one. An IKE SA notification carries an empty
// SPI. An SPI-bearing notification is therefore always a Child SA notification.
func TestNtfyChildSAProtocolIDIsAHOrESP(t *testing.T) {
	for _, tc := range []struct {
		name  string
		proto uint8
	}{
		{"AH", ProtocolAH},
		{"ESP", ProtocolESP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &PayloadNotify{
				ProtocolID: tc.proto, SPISize: 4, SPI: []byte{9, 8, 7, 6},
				NotifyMsgType: NotifyRekeySA,
			}
			buf := make([]byte, p.Len())
			n := p.WriteTo(buf, 0)
			if buf[0] != tc.proto {
				t.Errorf("encoded Protocol ID = %d, want %d", buf[0], tc.proto)
			}
			var got PayloadNotify
			if err := got.ReadFrom(buf[:n]); err != nil {
				t.Fatalf("PayloadNotify.ReadFrom: %v", err)
			}
			if got.ProtocolID != tc.proto {
				t.Errorf("recovered Protocol ID = %d, want %d", got.ProtocolID, tc.proto)
			}
			if !bytes.Equal(got.SPI, []byte{9, 8, 7, 6}) {
				t.Errorf("recovered SPI = %x, want 09080706", got.SPI)
			}
		})
	}

	// Negative: an SPI-bearing notification that names any other protocol is refused.
	for _, proto := range []uint8{0, ProtocolIKE, 4, 255} {
		body := []byte{proto, 4, 0x40, 0x09, 1, 2, 3, 4}
		var got PayloadNotify
		if err := got.ReadFrom(body); !errors.Is(err, ErrNotifyProtocolID) {
			t.Errorf("ReadFrom(Protocol ID %d with a 4-octet SPI) = %v, want ErrNotifyProtocolID",
				proto, err)
		}
	}
}

// RFC requirement: RFC7296-3.10-5 positive -- when the SPI field is empty the Protocol ID octet goes
// on the wire as zero. RFC 7296 Section 3.10 states "If the SPI field is empty, this field
// MUST be sent as zero and MUST be ignored on receipt". PayloadNotify.WriteTo derives the
// octet from the SPI octets it writes, so a stale Protocol ID never reaches a peer.
//
// RFC requirement: RFC7296-3.10-5 negative -- the zeroing is conditional on an empty SPI. A
// notification that does carry an SPI keeps its Protocol ID. That octet names the Child SA
// protocol. The codec has not dropped the Protocol ID field.
func TestNtfyEmptySPISendsProtocolIDZero(t *testing.T) {
	// An SPI Size of zero leaves the SPI field empty, whatever the caller set.
	empty := &PayloadNotify{ProtocolID: ProtocolESP, NotifyMsgType: NotifyInitialContact}
	buf := make([]byte, empty.Len())
	n := empty.WriteTo(buf, 0)
	if buf[0] != 0 {
		t.Errorf("empty-SPI notify encoded Protocol ID = %d, want 0", buf[0])
	}
	if buf[1] != 0 {
		t.Errorf("empty-SPI notify encoded SPI Size = %d, want 0", buf[1])
	}
	if n != 4 {
		t.Errorf("empty-SPI notify body = %d octets, want 4", n)
	}

	// A short SPI slice also leaves the field empty, so the same rule holds. WriteTo
	// already forces the SPI Size to zero here, and the Protocol ID follows it.
	under := &PayloadNotify{
		ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{1},
		NotifyMsgType: NotifyRekeySA,
	}
	ubuf := make([]byte, under.Len())
	un := under.WriteTo(ubuf, 0)
	if ubuf[0] != 0 {
		t.Errorf("short-SPI notify encoded Protocol ID = %d, want 0", ubuf[0])
	}
	if ubuf[1] != 0 {
		t.Errorf("short-SPI notify encoded SPI Size = %d, want 0", ubuf[1])
	}
	if un != under.Len() {
		t.Errorf("WriteTo wrote %d octets but Len reports %d", un, under.Len())
	}

	// Negative: a real SPI keeps its Protocol ID on the wire.
	full := &PayloadNotify{
		ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{1, 2, 3, 4},
		NotifyMsgType: NotifyRekeySA,
	}
	fbuf := make([]byte, full.Len())
	full.WriteTo(fbuf, 0)
	if fbuf[0] != ProtocolESP {
		t.Errorf("SPI-bearing notify encoded Protocol ID = %d, want %d", fbuf[0], ProtocolESP)
	}
}

// RFC requirement: RFC7296-3.10-5 positive -- a Protocol ID that arrives beside an empty SPI field is
// ignored. RFC 7296 Section 3.10 requires the receiver to ignore it, so PayloadNotify.ReadFrom
// discards the octet and reports zero. No consumer can branch on a value the RFC calls dead.
//
// RFC requirement: RFC7296-3.10-5 negative -- the discard is scoped to an empty SPI field. When the
// notification carries an SPI, the parsed Protocol ID is retained. There the octet names the
// protocol of the Child SA that the notification is about.
func TestNtfyEmptySPIProtocolIDIgnoredOnReceipt(t *testing.T) {
	// Octet 0 is ESP and octet 1 (SPI Size) is zero, so the SPI field is empty.
	body := []byte{ProtocolESP, 0, 0x40, 0x00}
	var got PayloadNotify
	if err := got.ReadFrom(body); err != nil {
		t.Fatalf("PayloadNotify.ReadFrom: %v", err)
	}
	if got.ProtocolID != 0 {
		t.Errorf("Protocol ID beside an empty SPI = %d, want 0 (ignored)", got.ProtocolID)
	}
	if got.NotifyMsgType != NotifyInitialContact {
		t.Errorf("NotifyMsgType = %d, want %d", got.NotifyMsgType, NotifyInitialContact)
	}

	// A non-zero Protocol ID with an empty SPI is ignored rather than refused. The
	// receive rule of Section 3.10 is to ignore the octet, not to drop the payload.
	for _, proto := range []uint8{ProtocolIKE, ProtocolAH, ProtocolESP, 200} {
		var p PayloadNotify
		if err := p.ReadFrom([]byte{proto, 0, 0x40, 0x00}); err != nil {
			t.Errorf("ReadFrom(Protocol ID %d with an empty SPI) = %v, want nil", proto, err)
		}
		if p.ProtocolID != 0 {
			t.Errorf("Protocol ID %d survived an empty SPI as %d, want 0", proto, p.ProtocolID)
		}
	}

	// Negative: with an SPI present the Protocol ID is kept.
	var kept PayloadNotify
	if err := kept.ReadFrom([]byte{ProtocolESP, 4, 0x40, 0x09, 1, 2, 3, 4}); err != nil {
		t.Fatalf("PayloadNotify.ReadFrom: %v", err)
	}
	if kept.ProtocolID != ProtocolESP {
		t.Errorf("Protocol ID beside a 4-octet SPI = %d, want %d", kept.ProtocolID, ProtocolESP)
	}
}

// VALIDATES: SET_WINDOW_SIZE notification data is read only when it is exactly 4
// octets, and the four octets are read in big-endian order.
// PREVENTS: a length check that accepts a longer or shorter body, and a decoder that
// reads the promised window in the wrong byte order.
//
// RFC requirement: RFC7296-2.3-7 positive -- RFC 7296 Section 2.3 states "The data associated with
// a SET_WINDOW_SIZE notification MUST be 4 octets long and contain the big endian
// representation of the number of messages the sender promises to keep"
// (rfc/full/rfc7296.txt:1447-1449). ParseSetWindowSize accepts exactly 4 octets and
// returns the big-endian value of those octets.
//
// RFC requirement: RFC7296-2.3-7 negative -- 3 octets and 5 octets are each refused with
// ErrSetWindowSizeLength, and so is an empty body. The 4-octet rule is a boundary and
// not a minimum, so a body one octet short and a body one octet long both fail.
func TestNtfySetWindowSizeDataIsFourOctets(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want uint32
	}{
		{"zero", []byte{0x00, 0x00, 0x00, 0x00}, 0},
		{"one", []byte{0x00, 0x00, 0x00, 0x01}, 1},
		{"big endian order", []byte{0x01, 0x02, 0x03, 0x04}, 0x01020304},
		{"maximum", []byte{0xFF, 0xFF, 0xFF, 0xFF}, 0xFFFFFFFF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSetWindowSize(tc.data)
			if err != nil {
				t.Fatalf("ParseSetWindowSize(%x): %v", tc.data, err)
			}
			if got != tc.want {
				t.Errorf("ParseSetWindowSize(%x) = %d, want %d", tc.data, got, tc.want)
			}
		})
	}

	// Negative: any other length is refused. Three octets and five octets bracket the
	// 4-octet rule, so neither a minimum nor a maximum check passes this set.
	for _, data := range [][]byte{
		nil,
		{},
		{0x01},
		{0x01, 0x02},
		{0x01, 0x02, 0x03},
		{0x01, 0x02, 0x03, 0x04, 0x05},
		{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	} {
		got, err := ParseSetWindowSize(data)
		if !errors.Is(err, ErrSetWindowSizeLength) {
			t.Errorf("ParseSetWindowSize(%d octets) = %v, want ErrSetWindowSizeLength", len(data), err)
		}
		if got != 0 {
			t.Errorf("ParseSetWindowSize(%d octets) returned window %d beside its error, want 0",
				len(data), got)
		}
	}
}

// TestNtfySetWindowSizeReadsThroughTheNotifyCodec proves the 4-octet body survives the
// PayloadNotify codec, so the parser above reads what a peer actually sends rather than
// a hand-built slice.
func TestNtfySetWindowSizeReadsThroughTheNotifyCodec(t *testing.T) {
	p := &PayloadNotify{
		NotifyMsgType:    NotifySetWindowSize,
		NotificationData: []byte{0x00, 0x00, 0x00, 0x05},
	}
	buf := make([]byte, p.Len())
	n := p.WriteTo(buf, 0)
	var got PayloadNotify
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("PayloadNotify.ReadFrom: %v", err)
	}
	if got.NotifyMsgType != NotifySetWindowSize {
		t.Fatalf("NotifyMsgType = %d, want %d", got.NotifyMsgType, NotifySetWindowSize)
	}
	window, err := ParseSetWindowSize(got.NotificationData)
	if err != nil {
		t.Fatalf("ParseSetWindowSize on a round-tripped notify: %v", err)
	}
	if window != 5 {
		t.Errorf("round-tripped window = %d, want 5", window)
	}
}

// TestNtfyRekeyNotifyMatchesProduction pins the one Child SA notification that ze sends. The
// rekey path builds a REKEY_SA notify with the ESP protocol and a 4-octet SPI
// (internal/component/ike/engine/rekey.go:67). That shape satisfies both Section 3.10 rules
// above, so ze never emits a notification that its own parser refuses.
func TestNtfyRekeyNotifyMatchesProduction(t *testing.T) {
	p := &PayloadNotify{
		ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{0, 0, 0, 1},
		NotifyMsgType: NotifyRekeySA,
	}
	buf := make([]byte, p.Len())
	n := p.WriteTo(buf, 0)
	var got PayloadNotify
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ze refuses its own REKEY_SA notify: %v", err)
	}
	if got.ProtocolID != ProtocolESP || got.SPISize != 4 {
		t.Errorf("round trip gave Protocol ID %d and SPI Size %d, want %d and 4",
			got.ProtocolID, got.SPISize, ProtocolESP)
	}
}
