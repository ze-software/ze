// Design: docs/architecture/wire/nlri.md -- NLRI receive validation, the per-family syntax registry.
// Related: ../plugins/nlri/srpolicy/split.go -- the registered RFC 9830 Section 5 rule under test.
// Related: session_validation_nlritype.go -- typedNLRIEdit, which removes the malformed NLRIs.
// RFC 9830 Sections 2.1, 4.2.1 and 5 -- see rfc/short/rfc9830.md.

package reactor

import (
	"bufio"
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// srNLRI builds one SR Policy NLRI section entry: an optional path identifier, the NLRI
// Length octet lenBits, then bodyOctets of distinguisher, color and endpoint (RFC 9830
// Section 2.1). seed varies the body so two NLRIs differ.
func srNLRI(lenBits, bodyOctets int, addPath bool, seed byte) []byte {
	var out []byte
	if addPath {
		out = append(out, 0, 0, 0, seed)
	}
	out = append(out, byte(lenBits))
	for i := range bodyOctets {
		out = append(out, seed+byte(i))
	}
	return out
}

// srReceiveBody builds an iBGP UPDATE body for fam. reach goes in MP_REACH_NLRI, with the
// mandatory attributes and one type-15 Tunnel Encapsulation TLV; unreach goes in
// MP_UNREACH_NLRI. withOthers adds ORIGIN, AS_PATH and LOCAL_PREF when there is no reach.
func srReceiveBody(fam family.Family, reach, unreach []byte, withOthers bool) []byte {
	var attrs []byte
	if len(unreach) > 0 {
		attrs = append(attrs, mpUnreachAttrs(fam, unreach)...)
	}
	if len(reach) > 0 {
		mp := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), 4, 192, 0, 2, 1, 0}
		mp = append(mp, reach...)
		attrs = append(attrs, 0x90, byte(attribute.AttrMPReachNLRI), byte(len(mp)>>8), byte(len(mp)))
		attrs = append(attrs, mp...)
		withOthers = true
		value := teSRPolicyValue(0, false)
		attrs = append(attrs, 0xD0, byte(attribute.AttrTunnelEncap), byte(len(value)>>8), byte(len(value)))
		attrs = append(attrs, value...)
	}
	if withOthers {
		attrs = append(attrs, 0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 5, 4, 0, 0, 0, 100)
	}
	return makeUpdateBody(nil, attrs, nil)
}

// srReceipt is what one UPDATE did to an Established session.
type srReceipt struct {
	err        error
	dispatched [][]byte
	written    []byte
	state      fsm.State
}

// srReceive delivers body through processMessage on an Established iBGP session that
// negotiated ipv4/sr-policy and ipv6/sr-policy, with ADD-PATH when addPath is set, and
// records what was dispatched and written.
func srReceive(t *testing.T, addPath bool, body []byte) srReceipt {
	t.Helper()
	v4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	v6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFISRPolicy}
	s := prefixReceiveSession(t, v4, PrefixCountInstalled, 1000, addPath, v6)
	conn := &recordingConn{}
	s.conn = conn
	s.bufWriter = bufio.NewWriterSize(conn, 4096)
	t.Cleanup(s.stopSendHoldTimer)
	t.Cleanup(s.closeConn)
	var got srReceipt
	s.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		if direction == rpc.DirectionReceived && wu != nil {
			got.dispatched = append(got.dispatched, bytes.Clone(wu.Payload()))
		}
		return false
	}
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))} //nolint:gosec // Fixed fixtures fit in a BGP message.
	got.err, _ = s.processMessage(&header, bytes.Clone(body), BufHandle{ID: noPoolBufID})
	got.written = conn.written()
	got.state = s.State()
	return got
}

// srReset is the NOTIFICATION every SR Policy session reset sends: UPDATE Message Error /
// Malformed Attribute List (3/1), like every other MP NLRI reset.
var srReset = message.PackTo(&message.Notification{
	ErrorCode: message.NotifyUpdateMessage, ErrorSubcode: message.NotifyUpdateMalformedAttr,
}, nil)

// srRequireKept asserts the UPDATE was accepted unchanged with no RFC 7606 action.
func srRequireKept(t *testing.T, name string, got srReceipt, body []byte) {
	t.Helper()
	if got.err != nil || got.state != fsm.StateEstablished || len(got.written) != 0 {
		t.Fatalf("%s: err=%v state=%v wrote %x, want accepted", name, got.err, got.state, got.written)
	}
	if len(got.dispatched) != 1 || !bytes.Equal(got.dispatched[0], body) {
		t.Fatalf("%s: dispatched %x, want the received body unchanged", name, got.dispatched)
	}
}

// srRequireWithdrawn asserts treat-as-withdraw: the session stays Established, nothing is
// written, and the received body itself is never dispatched.
func srRequireWithdrawn(t *testing.T, name string, got srReceipt, body []byte) {
	t.Helper()
	if got.err != nil || got.state != fsm.StateEstablished || len(got.written) != 0 {
		t.Fatalf("%s: err=%v state=%v wrote %x, want treat-as-withdraw", name, got.err, got.state, got.written)
	}
	for _, one := range got.dispatched {
		if bytes.Equal(one, body) {
			t.Fatalf("%s: the malformed UPDATE was dispatched as received", name)
		}
	}
}

// srRequireReset asserts session reset with NOTIFICATION 3/1.
func srRequireReset(t *testing.T, name string, got srReceipt) {
	t.Helper()
	if got.err == nil || got.state == fsm.StateEstablished {
		t.Fatalf("%s: err=%v state=%v, want session reset", name, got.err, got.state)
	}
	if !bytes.Equal(got.written, srReset) {
		t.Fatalf("%s: NOTIFICATION %x, want %x (3/1)", name, got.written, srReset)
	}
	if len(got.dispatched) != 0 {
		t.Fatalf("%s: dispatched %d UPDATEs before the reset", name, len(got.dispatched))
	}
}

// TestRFC9830NLRILengthReceiveVerdicts drives every SR Policy NLRI length case through
// processMessage, with and without ADD-PATH, and asserts the exact outcome.
//
// RFC 9830 Section 4.2.1: "The SR Policy NLRI MUST include a distinguisher, Color, and
// Endpoint field that implies that the length of the NLRI MUST be either 12 or 24 octets
// (depending on the address family of the Endpoint)." Section 2.1: "When AFI = 1, the value
// MUST be 96; when AFI = 2, the value MUST be 192." Section 5: a walkable error is
// "treat-as- withdraw"; an unwalkable one is "session reset" because ze has no AFI/SAFI
// disable.
func TestRFC9830NLRILengthReceiveVerdicts(t *testing.T) {
	v4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	v6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFISRPolicy}
	for _, addPath := range []bool{false, true} {
		good4 := srNLRI(96, 12, addPath, 0x10)
		good6 := srNLRI(192, 24, addPath, 0x20)

		// Well-formed: accepted unchanged, in each family.
		// RFC requirement: RFC9830-4.2.1-2 positive -- a received SR Policy NLRI holding a distinguisher, a Color and an Endpoint (12 octets under AFI 1, 24 under AFI 2), with and without ADD-PATH, is accepted through processMessage with no RFC 7606 action and delivered unchanged
		// RFC requirement: RFC9830-4.2.1-3 positive -- an NLRI of 12 octets under AFI 1 and of 24 octets under AFI 2 is accepted unchanged through processMessage, with and without ADD-PATH
		// RFC requirement: RFC9830-5-1 positive -- the syntactic validation of each NLRI length, of the MP attribute's NLRI section and of the length against the AFI passes a well-formed AFI 1 and AFI 2 NLRI unchanged
		for name, body := range map[string][]byte{
			"AFI 1 12 octets": srReceiveBody(v4, good4, nil, false),
			"AFI 2 24 octets": srReceiveBody(v6, good6, nil, false),
		} {
			srRequireKept(t, name, srReceive(t, addPath, body), body)
		}

		// Walkable, wrong length for the AFI: treat-as-withdraw, session kept.
		// RFC requirement: RFC9830-4.2.1-2 negative -- a received NLRI too long or too short to be exactly a distinguisher, a Color and an Endpoint (13 or 25 octets) is not accepted: processMessage treats the UPDATE as withdrawn with the session kept
		// RFC requirement: RFC9830-4.2.1-3 negative -- an NLRI of 13 or 24 octets under AFI 1, or of 12 or 25 octets under AFI 2, in MP_REACH_NLRI or in MP_UNREACH_NLRI beside reachable NLRI, makes processMessage treat the UPDATE as withdrawn with the session kept
		// RFC requirement: RFC9830-5-1 negative -- the length checked against the AFI catches 24 octets under AFI 1 and 12 octets under AFI 2, lengths a framing-only walk would accept
		// RFC requirement: RFC9830-5-4 negative -- a walkable malformed NLRI beside reachable NLRI does not reset the session: it stays Established, so the reset is kept for the errors that cannot be skipped
		// RFC requirement: RFC9830-5-2 positive -- a walkable malformed NLRI in an UPDATE that also carries reachable NLRI, in MP_REACH_NLRI or in MP_UNREACH_NLRI, is handled as treat-as-withdraw: the session stays Established and the UPDATE is withdrawn
		for name, body := range map[string][]byte{
			"AFI 1 13 octets":                srReceiveBody(v4, srNLRI(104, 13, addPath, 1), nil, false),
			"AFI 1 24 octets":                srReceiveBody(v4, srNLRI(192, 24, addPath, 1), nil, false),
			"AFI 2 12 octets":                srReceiveBody(v6, srNLRI(96, 12, addPath, 1), nil, false),
			"AFI 2 25 octets":                srReceiveBody(v6, srNLRI(200, 25, addPath, 1), nil, false),
			"AFI 1 12 beside 13":             srReceiveBody(v4, append(append([]byte{}, good4...), srNLRI(104, 13, addPath, 1)...), nil, false),
			"MP_UNREACH 13 beside reachable": srReceiveBody(v4, good4, srNLRI(104, 13, addPath, 1), false),
		} {
			srRequireWithdrawn(t, name, srReceive(t, addPath, body), body)
		}

		// Unwalkable: session reset with 3/1, in MP_REACH and in MP_UNREACH.
		// RFC requirement: RFC9830-5-4 positive -- an NLRI section that cannot be walked (zero length, a length of 100 bits, an NLRI overrunning the attribute), in MP_REACH_NLRI or MP_UNREACH_NLRI, resets the session with NOTIFICATION 3/1, since ze has no AFI/SAFI disable
		// RFC requirement: RFC9830-5-2 negative -- an error that does not let the rest of the UPDATE be processed is not handled as treat-as-withdraw: the unwalkable cases reset the session and deliver nothing
		for name, body := range map[string][]byte{
			"zero length":         srReceiveBody(v4, srNLRI(0, 0, addPath, 1), nil, false),
			"100 bits":            srReceiveBody(v4, srNLRI(100, 12, addPath, 1), nil, false),
			"overrun":             srReceiveBody(v4, srNLRI(96, 11, addPath, 1), nil, false),
			"MP_UNREACH 100 bits": srReceiveBody(v4, good4, srNLRI(100, 12, addPath, 1), false),
		} {
			srRequireReset(t, name, srReceive(t, addPath, body))
		}

		// A skippable-malformed NLRI in an UPDATE with no reachable NLRI is a session reset
		// through the RFC 7606 Section 5.2 escalation. For the MP_UNREACH-alone shape RFC
		// 7606 Section 5.2 exempts it ("other than MP_UNREACH_NLRI") and treat-as-withdraw
		// is owed: this asserts the current escalation, a recorded pre-existing deviation
		// (plan/journal/gate-fires-outside-its-population.md, 2026-10-14), not conformance.
		srRequireReset(t, "MP_UNREACH alone 13 octets",
			srReceive(t, addPath, srReceiveBody(v4, nil, srNLRI(104, 13, addPath, 1), false)))
		srRequireReset(t, "MP_UNREACH with other attributes 13 octets",
			srReceive(t, addPath, srReceiveBody(v4, nil, srNLRI(104, 13, addPath, 1), true)))
	}

	// ADD-PATH only: a path identifier cut short cannot be walked. This is the section
	// TestReceivePrefixLimitShortAddPathEntry sends.
	srRequireReset(t, "cut-short path identifier",
		srReceive(t, true, srReceiveBody(v4, nil, []byte{8, 0}, false)))
}

// TestSRPolicyMalformedWithdrawalSynthesis checks that treat-as-withdraw relays no NLRI Ze
// judged malformed: typedNLRIEdit removes them before synthesis reads the body, so the
// synthesized MP_UNREACH carries exactly the well-formed NLRIs, and an UPDATE whose every
// NLRI is malformed synthesizes and dispatches nothing.
// PREVENTS: a downstream peer receiving the 13-octet NLRI inside a withdrawal and resetting
// its own session over it (spec R-2, owner Q2 (a)).
func TestSRPolicyMalformedWithdrawalSynthesis(t *testing.T) {
	v4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	for _, addPath := range []bool{false, true} {
		good := srNLRI(96, 12, addPath, 0x10)
		bad := srNLRI(104, 13, addPath, 0x30)
		body := srReceiveBody(v4, append(append([]byte{}, good...), bad...), nil, false)
		got := srReceive(t, addPath, body)
		srRequireWithdrawn(t, "12 beside 13", got, body)
		if len(got.dispatched) != 1 {
			t.Fatalf("ADD-PATH=%v: dispatched %d UPDATEs, want the one synthesized withdrawal", addPath, len(got.dispatched))
		}
		attrs := rfc8669PathAttrs(t, got.dispatched[0])
		if _, _, _, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI); found {
			t.Fatalf("ADD-PATH=%v: the withdrawal still carries MP_REACH_NLRI", addPath)
		}
		_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPUnreachNLRI)
		if !found {
			t.Fatalf("ADD-PATH=%v: no MP_UNREACH_NLRI in %x", addPath, got.dispatched[0])
		}
		if !bytes.Equal(value[3:], good) {
			t.Fatalf("ADD-PATH=%v: withdrawn NLRIs %x, want only the well-formed %x", addPath, value[3:], good)
		}

		onlyBad := srReceiveBody(v4, bad, nil, false)
		got = srReceive(t, addPath, onlyBad)
		srRequireWithdrawn(t, "13 alone", got, onlyBad)
		if len(got.dispatched) != 0 {
			t.Fatalf("ADD-PATH=%v: an UPDATE of only malformed NLRIs dispatched %x", addPath, got.dispatched)
		}
	}
}
