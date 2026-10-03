package l2tp

// Design: docs/architecture/wire/l2tp.md -- RFC 2661 header and message codec proofs
//
// Tagged proofs of RFC 2661 obligations the header codec and the
// per-message body writers and parsers already meet. Each tag names the
// exact assertion its body makes; a claim wider than the body is a lie the
// discrimination record exists to refuse.
// Related: header.go, avp.go, tunnel_fsm.go, tunnel_initiator.go,
// session_fsm.go, session_initiator.go.

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

// RFC requirement: RFC2661-3.1-1 positive -- every control header
// WriteControlHeader emits carries T=1.
// RFC requirement: RFC2661-3.1-2 positive -- every control header
// WriteControlHeader emits carries S=1.
// RFC requirement: RFC2661-3.1-3 positive -- every control header
// WriteControlHeader emits carries O=0.
// RFC requirement: RFC2661-3.1-4 positive -- every control header
// WriteControlHeader emits carries P=0.
// RFC requirement: RFC2661-3.1-5 positive -- every control header
// WriteControlHeader emits, and every data header WriteDataHeader emits under
// any flag combination, carries Ver=2.
// TestRFC2661HeaderFlagWord reads the leading 16-bit word of headers the two
// writers produce and checks each RFC 2661 Section 3.1 flag against its
// obliged value.
func TestRFC2661HeaderFlagWord(t *testing.T) {
	buf := make([]byte, 32)
	controlArgs := []struct{ length, tid, sid, ns, nr uint16 }{
		{12, 0, 0, 0, 0},
		{0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF},
		{40, 7, 9, 3, 4},
	}
	for _, a := range controlArgs {
		WriteControlHeader(buf, 0, a.length, a.tid, a.sid, a.ns, a.nr)
		word := binary.BigEndian.Uint16(buf)
		if word&flagT == 0 {
			t.Fatalf("control header %+v: T bit clear (word 0x%04x)", a, word)
		}
		if word&flagS == 0 {
			t.Fatalf("control header %+v: S bit clear (word 0x%04x)", a, word)
		}
		if word&flagO != 0 {
			t.Fatalf("control header %+v: O bit set (word 0x%04x)", a, word)
		}
		if word&flagP != 0 {
			t.Fatalf("control header %+v: P bit set (word 0x%04x)", a, word)
		}
		if word&verMask != 2 {
			t.Fatalf("control header %+v: Ver %d, want 2", a, word&verMask)
		}
	}
	// Data headers: every combination of the four flag inputs.
	for combo := range 16 {
		h := MessageHeader{
			HasLength:   combo&1 != 0,
			HasSequence: combo&2 != 0,
			HasOffset:   combo&4 != 0,
			Priority:    combo&8 != 0,
			TunnelID:    1,
			SessionID:   2,
		}
		WriteDataHeader(buf, 0, h)
		word := binary.BigEndian.Uint16(buf)
		if word&verMask != 2 {
			t.Fatalf("data header combo %d: Ver %d, want 2", combo, word&verMask)
		}
	}
}

// RFC requirement: RFC2661-3.1-2 negative -- a header with T=1 and S=0 is
// refused by ParseMessageHeader with ErrMalformedControl.
// TestRFC2661ControlHeaderWithoutSequenceRefused feeds a control word whose
// S bit is clear and expects the malformed-control error.
func TestRFC2661ControlHeaderWithoutSequenceRefused(t *testing.T) {
	// T=1 L=1 S=0 Ver=2, Length 12, TID 1, SID 0, then four pad bytes.
	b := []byte{0xC0, 0x02, 0x00, 0x0C, 0, 1, 0, 0, 0, 0, 0, 0}
	_, err := ParseMessageHeader(b)
	if !errors.Is(err, ErrMalformedControl) {
		t.Fatalf("T=1,S=0: got %v, want ErrMalformedControl", err)
	}
	// The same word with S=1 is accepted, so the refusal is the S bit's.
	b[0] = 0xC8
	if _, err := ParseMessageHeader(b); err != nil {
		t.Fatalf("T=1,S=1: unexpected %v", err)
	}
}

// RFC requirement: RFC2661-3.1-5 negative -- a header whose Ver field is 3
// is refused by ParseMessageHeader with ErrUnsupportedVersion.
// RFC requirement: RFC2661-3.1-6 positive -- a header whose Ver field is 0,
// 1, 3 or 15 is refused by ParseMessageHeader with ErrUnsupportedVersion,
// the error reactor.handle discards on.
// RFC requirement: RFC2661-8.1-3 positive -- a packet whose Ver field is 1
// (the L2F version) is refused by ParseMessageHeader with
// ErrUnsupportedVersion, the error reactor.handle discards on.
// TestRFC2661UnknownVersionRefused walks the Ver values Ze does not speak.
func TestRFC2661UnknownVersionRefused(t *testing.T) {
	for _, ver := range []byte{0, 1, 3, 15} {
		b := []byte{0xC8, ver, 0x00, 0x0C, 0, 1, 0, 0, 0, 0, 0, 0}
		_, err := ParseMessageHeader(b)
		if !errors.Is(err, ErrUnsupportedVersion) {
			t.Fatalf("Ver=%d: got %v, want ErrUnsupportedVersion", ver, err)
		}
	}
}

// RFC requirement: RFC2661-3.1-6 negative -- a header whose Ver field is 2
// is not discarded: ParseMessageHeader returns it with no error.
// RFC requirement: RFC2661-8.1-3 negative -- a packet whose Ver field is 2 is
// not an L2F packet and ParseMessageHeader accepts it.
// TestRFC2661KnownVersionAccepted is the control for the discard tests above.
func TestRFC2661KnownVersionAccepted(t *testing.T) {
	control := []byte{0xC8, 0x02, 0x00, 0x0C, 0, 1, 0, 0, 0, 0, 0, 0}
	h, err := ParseMessageHeader(control)
	if err != nil {
		t.Fatalf("control Ver=2: %v", err)
	}
	if h.Version != 2 || !h.IsControl {
		t.Fatalf("control Ver=2: %+v", h)
	}
	data := []byte{0x00, 0x02, 0, 1, 0, 2}
	h, err = ParseMessageHeader(data)
	if err != nil {
		t.Fatalf("data Ver=2: %v", err)
	}
	if h.Version != 2 || h.IsControl {
		t.Fatalf("data Ver=2: %+v", h)
	}
}

// writtenBody is one control-message body a Ze writer produced.
type writtenBody struct {
	name string
	body []byte
}

// writtenBodies builds every control-message body Ze emits, with every
// optional AVP present, so one loop can assert an invariant over all of them.
func writtenBodies() []writtenBody {
	defaults := TunnelDefaults{
		HostName:            "ze",
		FramingCapabilities: 3,
		BearerCapabilities:  3,
		RecvWindow:          4,
	}
	challenge := []byte{1, 2, 3, 4}
	response := make([]byte, 16)
	tieBreaker := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	var out []writtenBody
	add := func(name string, write func(buf []byte) int) {
		buf := make([]byte, 512)
		n := write(buf)
		out = append(out, writtenBody{name: name, body: buf[:n]})
	}
	add("SCCRQ", func(buf []byte) int { return writeSCCRQBody(buf, 5, defaults, challenge, tieBreaker) })
	add("SCCRP", func(buf []byte) int { return writeSCCRPBody(buf, 5, defaults, challenge, response) })
	add("SCCCN", func(buf []byte) int { return writeSCCCNBody(buf, response) })
	add("StopCCN", func(buf []byte) int {
		return writeStopCCNBody(buf, 5, ResultCodeValue{Result: 1, ErrorPresent: true, Error: 0, Message: "bye"})
	})
	add("ICRQ", func(buf []byte) int { return writeICRQBody(buf, 9, 77, 1, "5551000", "5552000") })
	add("ICRP", func(buf []byte) int { return writeICRPBody(buf, 9) })
	add("ICCN", func(buf []byte) int { return writeICCNBody(buf, 64000, 1) })
	add("OCRQ", func(buf []byte) int { return writeOCRQBody(buf, 9, 77, 9600, 64000, 1, 1, "5551000") })
	add("OCRP", func(buf []byte) int { return writeOCRPBody(buf, 9) })
	add("CDN", func(buf []byte) int { return writeCDNBody(buf, 9, 1) })
	return out
}

// rfc2661MandatoryBit is the M-bit value RFC 2661 Section 4.4 fixes for each
// AVP a Ze writer emits ("The M-bit for this AVP MUST be set to 1" or "0").
var rfc2661MandatoryBit = map[AVPType]bool{
	AVPMessageType:         true,
	AVPResultCode:          true,
	AVPProtocolVersion:     true,
	AVPFramingCapabilities: true,
	AVPBearerCapabilities:  true,
	AVPTieBreaker:          false,
	AVPHostName:            true,
	AVPAssignedTunnelID:    true,
	AVPReceiveWindowSize:   true,
	AVPChallenge:           true,
	AVPChallengeResponse:   true,
	AVPAssignedSessionID:   true,
	AVPCallSerialNumber:    true,
	AVPMinimumBPS:          true,
	AVPMaximumBPS:          true,
	AVPBearerType:          true,
	AVPFramingType:         true,
	AVPCalledNumber:        true,
	AVPCallingNumber:       true,
	AVPTxConnectSpeed:      true,
}

// RFC requirement: RFC2661-4.4.1-2 positive -- the first AVP of every body a
// Ze writer emits is a Message Type AVP with M=1.
// RFC requirement: RFC2661-4.4-1 positive -- every AVP a Ze writer emits whose
// Section 4.4 definition fixes M at 1 carries M=1, Bearer Type, Called Number
// and Calling Number in the ICRQ included.
// RFC requirement: RFC2661-4.4-2 positive -- the Tie Breaker AVP, the one
// M=0 AVP Ze emits, carries M=0.
// RFC requirement: RFC2661-4.4-3 positive -- no AVP a Ze writer emits carries
// H=1, so every AVP the RFC forbids hiding is sent with H=0.
// RFC requirement: RFC2661-4.3-3 positive -- no AVP a Ze writer emits carries
// H=1; Ze holds no wired shared-secret hiding, so H is never set.
// TestRFC2661WrittenAVPFlags walks every AVP of every body Ze writes and
// checks its M and H bits against Section 4.4.
func TestRFC2661WrittenAVPFlags(t *testing.T) {
	for _, wb := range writtenBodies() {
		it := NewAVPIterator(wb.body)
		first := true
		for {
			vendorID, attrType, flags, _, ok := it.Next()
			if !ok {
				break
			}
			if vendorID != 0 {
				t.Fatalf("%s: vendor AVP %d emitted", wb.name, vendorID)
			}
			if first {
				if attrType != AVPMessageType {
					t.Fatalf("%s: first AVP is %d, want Message Type", wb.name, attrType)
				}
				if flags&FlagMandatory == 0 {
					t.Fatalf("%s: Message Type AVP has M=0", wb.name)
				}
				first = false
			}
			if flags&FlagHidden != 0 {
				t.Fatalf("%s: AVP %d emitted with H=1", wb.name, attrType)
			}
			want, known := rfc2661MandatoryBit[attrType]
			if !known {
				t.Fatalf("%s: AVP %d has no expected M-bit in the test table", wb.name, attrType)
			}
			if got := flags&FlagMandatory != 0; got != want {
				t.Fatalf("%s: AVP %d M=%v, RFC 2661 Section 4.4 wants %v", wb.name, attrType, got, want)
			}
		}
		if err := it.Err(); err != nil {
			t.Fatalf("%s: iterate: %v", wb.name, err)
		}
		if first {
			t.Fatalf("%s: empty body", wb.name)
		}
	}
}

// avpTypesOf returns the set of attribute types a body carries.
func avpTypesOf(t *testing.T, body []byte) map[AVPType]bool {
	t.Helper()
	got := map[AVPType]bool{}
	it := NewAVPIterator(body)
	for {
		_, attrType, _, _, ok := it.Next()
		if !ok {
			break
		}
		got[attrType] = true
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	return got
}

// TestRFC2661MandatoryAVPSetsEmitted checks that every body writer emits the
// full AVP set its Section 6 message definition lists as MUST be present.
func TestRFC2661MandatoryAVPSetsEmitted(t *testing.T) {
	bodies := map[string][]byte{}
	for _, wb := range writtenBodies() {
		bodies[wb.name] = wb.body
	}
	cases := []struct {
		name string
		want []AVPType
	}{
		// RFC requirement: RFC2661-6.3-1 positive -- writeSCCCNBody emits the Message Type AVP.
		{"SCCCN", []AVPType{AVPMessageType}},
		// RFC requirement: RFC2661-6.4-1 positive -- writeStopCCNBody emits Message Type, Assigned Tunnel ID and Result Code.
		{"StopCCN", []AVPType{AVPMessageType, AVPAssignedTunnelID, AVPResultCode}},
		// RFC requirement: RFC2661-6.6-1 positive -- writeICRQBody emits Message Type, Assigned Session ID and Call Serial Number.
		{"ICRQ", []AVPType{AVPMessageType, AVPAssignedSessionID, AVPCallSerialNumber}},
		// RFC requirement: RFC2661-6.7-1 positive -- writeICRPBody emits Message Type and Assigned Session ID.
		{"ICRP", []AVPType{AVPMessageType, AVPAssignedSessionID}},
		// RFC requirement: RFC2661-6.8-1 positive -- writeICCNBody emits Message Type, Tx Connect Speed and Framing Type.
		{"ICCN", []AVPType{AVPMessageType, AVPTxConnectSpeed, AVPFramingType}},
		// RFC requirement: RFC2661-6.9-2 positive -- writeOCRQBody emits all eight AVPs Section 6.9 lists.
		{"OCRQ", []AVPType{AVPMessageType, AVPAssignedSessionID, AVPCallSerialNumber, AVPMinimumBPS, AVPMaximumBPS, AVPBearerType, AVPFramingType, AVPCalledNumber}},
		// RFC requirement: RFC2661-6.10-1 positive -- writeOCRPBody emits Message Type and Assigned Session ID.
		{"OCRP", []AVPType{AVPMessageType, AVPAssignedSessionID}},
		// RFC requirement: RFC2661-6.12-2 positive -- writeCDNBody emits Message Type, Result Code and Assigned Session ID.
		{"CDN", []AVPType{AVPMessageType, AVPResultCode, AVPAssignedSessionID}},
	}
	for _, c := range cases {
		body, ok := bodies[c.name]
		if !ok {
			t.Fatalf("%s: no writer in writtenBodies", c.name)
		}
		got := avpTypesOf(t, body)
		for _, want := range c.want {
			if !got[want] {
				t.Fatalf("%s: AVP %d absent from the emitted body", c.name, want)
			}
		}
	}
}

// bodyOf assembles a control-message body from a Message Type and a list of
// AVP writers, so a test can omit any one AVP.
func bodyOf(msg MessageType, avps ...func(buf []byte, off int) int) []byte {
	buf := make([]byte, 256)
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(msg))
	for _, w := range avps {
		off += w(buf, off)
	}
	return buf[:off]
}

func u16AVP(attr AVPType, v uint16) func(buf []byte, off int) int {
	return func(buf []byte, off int) int { return WriteAVPUint16(buf, off, true, attr, v) }
}

func u32AVP(attr AVPType, v uint32) func(buf []byte, off int) int {
	return func(buf []byte, off int) int { return WriteAVPUint32(buf, off, true, attr, v) }
}

func strAVP(attr AVPType, s string) func(buf []byte, off int) int {
	return func(buf []byte, off int) int { return WriteAVPString(buf, off, true, attr, s) }
}

func callErrorsAVP() func(buf []byte, off int) int {
	return func(buf []byte, off int) int {
		return writeAVPCallErrors(buf, off, CallErrorsValue{CRCErrors: 1})
	}
}

func accmAVP() func(buf []byte, off int) int {
	return func(buf []byte, off int) int { return writeAVPACCM(buf, off, ACCMValue{}) }
}

func resultCodeAVP() func(buf []byte, off int) int {
	return func(buf []byte, off int) int { return writeAVPResultCode(buf, off, ResultCodeValue{Result: 1}) }
}

// TestRFC2661MandatoryAVPSetsParsed checks the receive side of the Section 6
// AVP sets: a body carrying the set is accepted, a body missing one of the
// members the parser guards is refused.
func TestRFC2661MandatoryAVPSetsParsed(t *testing.T) {
	parseErr := func(parse func([]byte) error) func([]byte) error { return parse }
	scccn := parseErr(func(b []byte) error { _, err := parseSCCCN(b); return err })
	stopccn := parseErr(func(b []byte) error { _, err := parseStopCCN(b); return err })
	icrq := parseErr(func(b []byte) error { _, err := parseICRQ(b); return err })
	icrp := parseErr(func(b []byte) error { _, err := parseICRP(b); return err })
	iccn := parseErr(func(b []byte) error { _, err := parseICCN(b); return err })
	ocrq := parseErr(func(b []byte) error { _, err := parseOCRQ(b); return err })
	ocrp := parseErr(func(b []byte) error { _, err := parseOCRP(b); return err })
	occn := parseErr(func(b []byte) error { _, err := parseOCCN(b); return err })
	cdn := parseErr(func(b []byte) error { _, err := parseCDN(b); return err })
	wen := parseErr(func(b []byte) error { _, err := parseWEN(b); return err })
	sli := parseErr(func(b []byte) error { _, err := parseSLI(b); return err })

	cases := []struct {
		name   string
		body   []byte
		parse  func([]byte) error
		accept bool
	}{
		// RFC requirement: RFC2661-6.3-1 negative -- parseSCCCN refuses a body with no Message Type AVP.
		{"SCCCN empty", nil, scccn, false},
		{"SCCCN Message Type only", bodyOf(MsgSCCCN), scccn, true},
		// RFC requirement: RFC2661-6.4-1 negative -- parseStopCCN refuses a body with no Message Type AVP.
		{"StopCCN empty", nil, stopccn, false},
		{"StopCCN full", bodyOf(MsgStopCCN, u16AVP(AVPAssignedTunnelID, 5), resultCodeAVP()), stopccn, true},
		// RFC requirement: RFC2661-6.6-1 negative -- parseICRQ refuses an ICRQ missing the Assigned Session ID AVP, and one missing the Call Serial Number AVP.
		{"ICRQ no Assigned Session ID", bodyOf(MsgICRQ, u32AVP(AVPCallSerialNumber, 1)), icrq, false},
		{"ICRQ no Call Serial Number", bodyOf(MsgICRQ, u16AVP(AVPAssignedSessionID, 9)), icrq, false},
		{"ICRQ full", bodyOf(MsgICRQ, u16AVP(AVPAssignedSessionID, 9), u32AVP(AVPCallSerialNumber, 1)), icrq, true},
		// RFC requirement: RFC2661-6.7-1 negative -- parseICRP refuses an ICRP missing the Assigned Session ID AVP.
		{"ICRP no Assigned Session ID", bodyOf(MsgICRP), icrp, false},
		{"ICRP full", bodyOf(MsgICRP, u16AVP(AVPAssignedSessionID, 9)), icrp, true},
		// RFC requirement: RFC2661-6.8-1 negative -- parseICCN refuses an ICCN missing the Tx Connect Speed AVP, and one missing the Framing Type AVP.
		{"ICCN no Tx Connect Speed", bodyOf(MsgICCN, u32AVP(AVPFramingType, 1)), iccn, false},
		{"ICCN no Framing Type", bodyOf(MsgICCN, u32AVP(AVPTxConnectSpeed, 64000)), iccn, false},
		{"ICCN full", bodyOf(MsgICCN, u32AVP(AVPTxConnectSpeed, 64000), u32AVP(AVPFramingType, 1)), iccn, true},
		// RFC requirement: RFC2661-6.9-2 negative -- parseOCRQ refuses an OCRQ missing the Assigned Session ID AVP.
		{"OCRQ no Assigned Session ID", bodyOf(MsgOCRQ, u32AVP(AVPCallSerialNumber, 1), u32AVP(AVPMinimumBPS, 1), u32AVP(AVPMaximumBPS, 2), u32AVP(AVPBearerType, 1), u32AVP(AVPFramingType, 1), strAVP(AVPCalledNumber, "1")), ocrq, false},
		{"OCRQ full", bodyOf(MsgOCRQ, u16AVP(AVPAssignedSessionID, 9), u32AVP(AVPCallSerialNumber, 1), u32AVP(AVPMinimumBPS, 1), u32AVP(AVPMaximumBPS, 2), u32AVP(AVPBearerType, 1), u32AVP(AVPFramingType, 1), strAVP(AVPCalledNumber, "1")), ocrq, true},
		// RFC requirement: RFC2661-6.10-1 negative -- parseOCRP refuses an OCRP missing the Assigned Session ID AVP.
		{"OCRP no Assigned Session ID", bodyOf(MsgOCRP), ocrp, false},
		{"OCRP full", bodyOf(MsgOCRP, u16AVP(AVPAssignedSessionID, 9)), ocrp, true},
		// RFC requirement: RFC2661-6.11-1 negative -- parseOCCN refuses an OCCN missing the Tx Connect Speed AVP, and one missing the Framing Type AVP.
		{"OCCN no Tx Connect Speed", bodyOf(MsgOCCN, u32AVP(AVPFramingType, 1)), occn, false},
		{"OCCN no Framing Type", bodyOf(MsgOCCN, u32AVP(AVPTxConnectSpeed, 64000)), occn, false},
		// RFC requirement: RFC2661-6.11-1 positive -- parseOCCN accepts an OCCN carrying Message Type, Tx Connect Speed and Framing Type.
		{"OCCN full", bodyOf(MsgOCCN, u32AVP(AVPTxConnectSpeed, 64000), u32AVP(AVPFramingType, 1)), occn, true},
		// RFC requirement: RFC2661-6.12-2 negative -- parseCDN refuses a body with no Message Type AVP.
		{"CDN empty", nil, cdn, false},
		{"CDN full", bodyOf(MsgCDN, resultCodeAVP(), u16AVP(AVPAssignedSessionID, 9)), cdn, true},
		// RFC requirement: RFC2661-6.13-1 negative -- parseWEN refuses a WEN missing the Call Errors AVP.
		{"WEN no Call Errors", bodyOf(MsgWEN), wen, false},
		// RFC requirement: RFC2661-6.13-1 positive -- parseWEN accepts a WEN carrying Message Type and Call Errors.
		{"WEN full", bodyOf(MsgWEN, callErrorsAVP()), wen, true},
		// RFC requirement: RFC2661-6.14-2 negative -- parseSLI refuses an SLI missing the ACCM AVP.
		{"SLI no ACCM", bodyOf(MsgSLI), sli, false},
		// RFC requirement: RFC2661-6.14-2 positive -- parseSLI accepts an SLI carrying Message Type and ACCM.
		{"SLI full", bodyOf(MsgSLI, accmAVP()), sli, true},
	}
	for _, c := range cases {
		err := c.parse(c.body)
		if c.accept && err != nil {
			t.Fatalf("%s: refused: %v", c.name, err)
		}
		if !c.accept && err == nil {
			t.Fatalf("%s: accepted, want a refusal", c.name)
		}
	}
}

// sccrpBodyWithHostName builds an SCCRP carrying every Section 6.2 AVP and
// the given Host Name value.
func sccrpBodyWithHostName(host string) []byte {
	return bodyOf(MsgSCCRP,
		func(buf []byte, off int) int {
			return WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, protocolVersionValue[:])
		},
		u32AVP(AVPFramingCapabilities, 3),
		strAVP(AVPHostName, host),
		u16AVP(AVPAssignedTunnelID, 5),
	)
}

// RFC requirement: RFC2661-4.4.3-5 positive -- parseSCCRP accepts a Host Name
// AVP of exactly one octet.
// RFC requirement: RFC2661-4.4.3-5 negative -- parseSCCRP refuses a Host Name
// AVP of zero octets.
// TestRFC2661HostNameAtLeastOneOctet drives the SCCRP parser with the two
// lengths on either side of the Section 4.4.3 minimum.
func TestRFC2661HostNameAtLeastOneOctet(t *testing.T) {
	info, err := parseSCCRP(sccrpBodyWithHostName("z"))
	if err != nil {
		t.Fatalf("one-octet Host Name refused: %v", err)
	}
	if info.HostName != "z" {
		t.Fatalf("Host Name: got %q, want %q", info.HostName, "z")
	}
	if _, err := parseSCCRP(sccrpBodyWithHostName("")); err == nil {
		t.Fatalf("zero-octet Host Name accepted")
	}
}

// RFC requirement: RFC2661-4.4.6-1 positive -- writeAVPCallErrors emits the
// reserved leading 16 bits of the Call Errors value as zero.
// TestRFC2661CallErrorsReservedZero fills every counter with all-ones so a
// writer that placed a counter in the reserved slot would be caught.
func TestRFC2661CallErrorsReservedZero(t *testing.T) {
	buf := make([]byte, 64)
	full := CallErrorsValue{
		CRCErrors:        0xFFFFFFFF,
		FramingErrors:    0xFFFFFFFF,
		HardwareOverruns: 0xFFFFFFFF,
		BufferOverruns:   0xFFFFFFFF,
		TimeoutErrors:    0xFFFFFFFF,
		AlignmentErrors:  0xFFFFFFFF,
	}
	n := writeAVPCallErrors(buf, 0, full)
	if n != AVPHeaderLen+26 {
		t.Fatalf("length: got %d, want %d", n, AVPHeaderLen+26)
	}
	if reserved := binary.BigEndian.Uint16(buf[AVPHeaderLen:]); reserved != 0 {
		t.Fatalf("reserved field: got 0x%04x, want 0", reserved)
	}
	got, err := readCallErrors(buf[AVPHeaderLen:n])
	if err != nil {
		t.Fatalf("readCallErrors: %v", err)
	}
	if got != full {
		t.Fatalf("round trip: got %+v, want %+v", got, full)
	}
}

// RFC requirement: RFC2661-6.5-2 positive -- the HELLO handleHelloTimer emits
// carries Session ID 0 in its control header.
// RFC requirement: RFC2661-6.5-3 positive -- the HELLO handleHelloTimer emits
// carries the Message Type AVP, with value HELLO, as its first AVP.
// TestRFC2661HelloShape fires the HELLO timer on an established tunnel and
// reads the datagram it produces.
func TestRFC2661HelloShape(t *testing.T) {
	tun := newEstablishedTunnel(t, 1)
	out := tun.handleHelloTimer(time.Now())
	if len(out) != 1 {
		t.Fatalf("HELLO timer produced %d datagrams, want 1", len(out))
	}
	hdr, err := ParseMessageHeader(out[0].bytes)
	if err != nil {
		t.Fatalf("HELLO header: %v", err)
	}
	if !hdr.IsControl {
		t.Fatalf("HELLO is not a control message: %+v", hdr)
	}
	if hdr.SessionID != 0 {
		t.Fatalf("HELLO Session ID: got %d, want 0", hdr.SessionID)
	}
	body := out[0].bytes[hdr.PayloadOff:hdr.Length]
	it := NewAVPIterator(body)
	_, attrType, flags, value, ok := it.Next()
	if !ok {
		t.Fatalf("HELLO body carries no AVP: %v", it.Err())
	}
	if attrType != AVPMessageType || flags&FlagMandatory == 0 {
		t.Fatalf("HELLO first AVP: type %d flags %v, want Message Type with M=1", attrType, flags)
	}
	mt, err := readAVPUint16(value)
	if err != nil {
		t.Fatalf("HELLO Message Type value: %v", err)
	}
	if MessageType(mt) != MsgHello {
		t.Fatalf("HELLO Message Type: got %d, want %d", mt, MsgHello)
	}
}
