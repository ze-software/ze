// VALIDATES: the RFC 7296 Configuration payload obligations that ze discharges by a choice
// rather than by a guard. Ze builds no CP payload at all today. The sender obligations of
// Section 2.19 and Section 3.15.1 therefore fall on a role ze does not take. The CFG_SET
// obligations are discharged by the MAY in Section 3.15.1, which permits an implementation
// to ignore CFG_SET.
//
// Each test asserts a property the code HAS. Each one also pairs that
// with a negative that shows the opposite is expressible. No assertion rests on an absent
// guard.
// PREVENTS: a CP producer that arrives without the obligations that come with it. Also a
// well-meaning change that answers CFG_SET with a CFG_ACK ze never decided to send.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// cpPayloadsIn returns every Configuration payload in a chain.
func cpPayloadsIn(entries []wire.PayloadEntry) []*wire.PayloadCP {
	var out []*wire.PayloadCP
	for _, pe := range entries {
		if cp, ok := pe.Payload.(*wire.PayloadCP); ok {
			out = append(out, cp)
		}
	}
	return out
}

// builtCPSweep walks every message the engine emits, at both nesting levels. It returns the
// Configuration payloads it found, and the counts that prove the walk was not vacuous.
//
// rfc-test-change-approved: 2026-07-31 owner standing approval for
// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only. The first draft counted inner
// chains that EXISTED rather than inner payloads the sweep INSPECTED. Removal of m.inner
// from the walk therefore left the guard green. A Configuration payload always travels
// inside the encrypted SK payload. An outer-only sweep finds none, and every positive here
// becomes vacuous. The counters now witness the walk itself.
func builtCPSweep(t *testing.T) (found []*wire.PayloadCP, outerPayloads, innerPayloads int) {
	t.Helper()
	for _, m := range engineBuiltChains(t) {
		for _, level := range []struct {
			isInner bool
			entries []wire.PayloadEntry
		}{{false, m.outer}, {true, m.inner}} {
			if level.isInner {
				innerPayloads += len(level.entries)
			} else {
				outerPayloads += len(level.entries)
			}
			found = append(found, cpPayloadsIn(level.entries)...)
		}
	}
	if outerPayloads == 0 || innerPayloads == 0 {
		t.Fatalf("vacuous sweep: %d outer and %d inner payloads inspected. A Configuration "+
			"payload travels inside the encrypted SK payload, so a sweep that inspects no "+
			"decrypted inner chain proves nothing", outerPayloads, innerPayloads)
	}
	return found, outerPayloads, innerPayloads
}

// RFC requirement: RFC7296-2.19-1 positive -- the obligation binds the IRAC, the client that
// wants an address. Ze is not one. RFC 7296 Section 4 makes the role optional:
// "Implementations are not required to support requesting temporary IP addresses or
// responding to such requests." Ze builds no Configuration payload in any message.
// buildAuthRequest (auth.go) assembles IDi, the optional CERTREQ or CERT with AUTH, the
// INITIAL_CONTACT notify, then SAi2, TSi and TSr. No builder anywhere constructs a
// wire.PayloadCP.
//
// The sweep below covers every message kind the engine emits, at both
// nesting levels. The claim is therefore proven over messages, not asserted over source.
//
// RFC requirement: RFC7296-2.19-1 negative -- the absence is a decision. It is not an encoder
// that cannot express a CP payload. buildEncryptedMessageEx carries a Configuration payload
// to the wire intact. The peer decrypts it and recovers the attribute. The empty sweep above
// therefore records what ze chose to send. It changes when an IRAC builder is added.
//
// RFC requirement: RFC7296-2.19-4 positive -- this is the SENDER obligation on a
// CP(CFG_REQUEST). It binds whoever emits one. Ze emits none. It therefore never sends a
// CFG_REQUEST that lacks an INTERNAL_ADDRESS attribute.
//
// Do not read this row as the
// receive-side duty to recognize the attribute. That is a separate Section 4 obligation on
// the responder. Ze does not owe that one today either, because it does not support
// responding to such requests.
//
// RFC requirement: RFC7296-2.19-4 negative -- the same encoder proof. A CFG_REQUEST is
// expressible. The row is therefore discharged by ze declining the requester role, and not
// by a limit of the codec.
func TestZeSendsNoConfigurationRequest(t *testing.T) {
	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only. Tracks the renamed
	// anti-vacuity counters, which now witness the inner walk instead of its existence.
	found, outerPayloads, innerPayloads := builtCPSweep(t)
	for _, cp := range found {
		t.Errorf("a Configuration payload (CFG type %d, %d attributes) was built; ze takes "+
			"no IRAC role and constructs no CP payload", cp.CFGType, len(cp.Attrs))
	}
	t.Logf("swept %d outer and %d decrypted inner payloads", outerPayloads, innerPayloads)

	// Negative: the encoder honors a Configuration payload when asked for one.
	ini, resp, _ := establishPSK(t)
	want := &wire.PayloadCP{CFGType: wire.CFGTypeRequest, Attrs: []wire.ConfigAttr{
		{Type: wire.CPAttrInternalIP4Address},
	}}
	raw, err := buildEncryptedMessageEx(ini, []wire.PayloadEntry{{Payload: want}},
		ini.NextMsgID, wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decryptAndParse: %v", err)
	}
	carried := cpPayloadsIn(inner)
	if len(carried) != 1 {
		t.Fatalf("the encoder carried %d Configuration payloads, want 1; the absence above "+
			"must be a decision rather than an encoder that drops the payload", len(carried))
	}
	if carried[0].CFGType != wire.CFGTypeRequest {
		t.Errorf("carried CFG type = %d, want %d", carried[0].CFGType, wire.CFGTypeRequest)
	}
	if len(carried[0].Attrs) != 1 || carried[0].Attrs[0].Type != wire.CPAttrInternalIP4Address {
		t.Errorf("carried attributes = %+v, want one INTERNAL_IP4_ADDRESS", carried[0].Attrs)
	}
}

// RFC requirement: RFC7296-2.20-1 positive -- RFC 7296 Section 2.20 lets an implementation
// decline to give out version information. It requires a decline to be an empty string, or
// no CP payload at all when CP is not supported. Ze declines, and it takes the second form.
// Ze constructs no Configuration payload in any message. It therefore emits no
// APPLICATION_VERSION attribute in any exchange. The sweep asserts that the attribute count
// is zero over every message kind, at both nesting levels.
//
// RFC requirement: RFC7296-2.20-1 negative -- the silence is a decline, not a missing code
// path. The codec carries a non-empty APPLICATION_VERSION and returns it unchanged. Ze CAN
// answer with a version string, and it does not. This half keeps the row honest when a CP
// producer is added. An unconfigured version must stay empty or absent. It must never become
// a default string, because a default answers where the RFC requires a decline.
func TestZeDeclinesApplicationVersion(t *testing.T) {
	found, _, _ := builtCPSweep(t)
	versions := 0
	for _, cp := range found {
		for _, a := range cp.Attrs {
			if a.Type == wire.CPAttrApplicationVersion {
				versions++
			}
		}
	}
	if versions != 0 {
		t.Errorf("%d APPLICATION_VERSION attributes were emitted; ze declines to give out "+
			"version information and must return an empty string or no CP payload", versions)
	}

	// Negative: the attribute is expressible and round-trips with its value intact.
	cp := &wire.PayloadCP{CFGType: wire.CFGTypeReply, Attrs: []wire.ConfigAttr{
		{Type: wire.CPAttrApplicationVersion, Value: []byte("ze")},
	}}
	buf := make([]byte, cp.Len())
	cp.WriteTo(buf, 0)
	var back wire.PayloadCP
	if err := back.ReadFrom(buf); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if len(back.Attrs) != 1 || string(back.Attrs[0].Value) != "ze" {
		t.Fatalf("APPLICATION_VERSION round trip gave %+v, want one attribute valued \"ze\"; "+
			"the decline above must be a choice rather than an unimplemented attribute",
			back.Attrs)
	}
}

// RFC requirement: RFC7296-3.15.1-2 positive -- the MUST NOT binds whoever sends a
// CFG_REQUEST. A non-empty INTERNAL_IP4_NETMASK does not make sense in a request, and must
// not be included. Ze sends no CFG_REQUEST at all. It therefore includes no netmask value in
// one. The sweep asserts that no message ze builds carries a Configuration payload of CFG
// type CFG_REQUEST.
//
// RFC requirement: RFC7296-3.15.1-2 negative -- the receive side stays tolerant. A peer can
// violate this row with a non-empty netmask in its CFG_REQUEST. Ze still parses that message
// and preserves the value. RFC 7296 Section 2.5 forbids rejection of a message over payload
// composition of this kind. Section 3.15.1 requires an implementation to ignore rather than
// refuse. Ze must not turn this sender-side rule into a receive-side check.
func TestZeSendsNoConfigRequestNetmask(t *testing.T) {
	found, _, _ := builtCPSweep(t)
	for _, cp := range found {
		if cp.CFGType == wire.CFGTypeRequest {
			t.Errorf("ze built a CP(CFG_REQUEST) with %d attributes; it takes no requester "+
				"role, so no netmask value can appear in one", len(cp.Attrs))
		}
	}

	// Negative: a peer's malformed CFG_REQUEST parses and is not rejected.
	offending := &wire.PayloadCP{CFGType: wire.CFGTypeRequest, Attrs: []wire.ConfigAttr{
		{Type: wire.CPAttrInternalIP4Address},
		{Type: wire.CPAttrInternalIP4Netmask, Value: []byte{255, 255, 255, 0}},
	}}
	ini, resp, _ := establishPSK(t)
	raw, err := buildEncryptedMessageEx(ini, []wire.PayloadEntry{{Payload: offending}},
		ini.NextMsgID, wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("a CFG_REQUEST carrying a non-empty netmask was rejected: %v; ze must "+
			"tolerate the peer's violation rather than refuse the message", err)
	}
	carried := cpPayloadsIn(inner)
	if len(carried) != 1 || len(carried[0].Attrs) != 2 {
		t.Fatalf("parsed %d CP payloads, want 1 holding 2 attributes", len(carried))
	}
	if len(carried[0].Attrs[1].Value) != 4 {
		t.Errorf("the offending netmask value was altered: %+v; the parser preserves what "+
			"arrived and leaves the decision to a consumer", carried[0].Attrs[1])
	}
}

// informationalOutcome is what one INFORMATIONAL request produced. It holds the payload
// types of the decrypted response, and the responder SA state afterwards. State is the
// observable that discriminates here. handleInformationalOwned always builds its response
// with a nil payload chain (inbound.go). Every response therefore carries zero payloads,
// whatever arrived.
type informationalOutcome struct {
	respTypes []uint8
	state     SAState
}

// driveInformational drives one INFORMATIONAL request carrying the given inner chain against
// a freshly established session. A fresh session per call keeps message-ID and SA state from
// leaking between variants.
func driveInformational(t *testing.T, inner []wire.PayloadEntry) informationalOutcome {
	t.Helper()
	ini, resp, ps := establishPSK(t)
	if resp.State != StateEstablished {
		t.Fatalf("fixture: responder SA is %v before the request, want established", resp.State)
	}
	probe := &wire.Message{Header: wire.Header{
		InitiatorSPI: resp.InitiatorSPI, ResponderSPI: resp.ResponderSPI,
		MajorVersion: 2, ExchangeType: wire.ExchangeInformational,
		Flags: wire.FlagInitiator, MessageID: resp.ExpectedMsgID,
	}}
	resp.lastResponse = nil
	resp.lastResponseSet = false
	ps.handleInformationalOwned(resp, probe, inner, false, nil, nil, slogutil.DiscardLogger())
	if resp.lastResponse == nil {
		t.Fatal("no INFORMATIONAL response was built, so the request went unanswered")
	}
	raw := resp.lastResponse
	decrypted, err := decryptAndParse(ini, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decryptAndParse of the response: %v", err)
	}
	types := make([]uint8, 0, len(decrypted))
	for _, pe := range decrypted {
		types = append(types, pe.Payload.Type())
	}
	return informationalOutcome{respTypes: types, state: resp.State}
}

// RFC requirement: RFC7296-3.15.1-5 positive -- the MUST is conditional on the responder having
// accepted configuration data pushed in a CFG_SET, and ze accepts none. RFC 7296 Section
// 3.15.1 ends the CFG_SET and CFG_ACK description with "An implementation of this
// specification MAY ignore CFG_SET payloads", and it records that "There are currently no
// defined uses for the CFG_SET/CFG_ACK exchange". Ze exercises that MAY: a CP(CFG_SET) in an
// INFORMATIONAL request changes no configuration and draws a response carrying no
// Configuration payload. The antecedent is therefore false and the obligation is discharged
// by the RFC's own permission rather than by an oversight.
// RFC requirement: RFC7296-3.15.1-5 negative -- an ignored payload is not a dropped message.
// The request IS answered. A response is built, and it is well formed. The observable is
// also sensitive. A payload ze does interpret changes the SA state. So "the CFG_SET changed
// nothing" is a fact about CFG_SET, not a comparison that can never fail.
//
// RFC requirement: RFC7296-3.15.1-6 positive -- the MUST NOT forbids an unaccepted attribute from
// appearing in a CFG_ACK. Ze builds no CFG_ACK in any exchange, so no attribute of any kind
// can appear in one. The assertion below covers every payload of the response, not only
// Configuration payloads of CFG type ACK.
// RFC requirement: RFC7296-3.15.1-6 negative -- a CFG_ACK is expressible. The codec builds one
// carrying attributes and parses it back, so the absence is ze's decision to ignore CFG_SET
// and not a payload it cannot construct.
//
// RFC requirement: RFC7296-3.15.1-7 positive -- when no attributes were accepted the responder must
// return either an empty CFG_ACK payload or a response message without a CFG_ACK payload. Ze
// accepts no attributes and takes the second option verbatim: the response exists and carries
// no CFG_ACK. This row is satisfied affirmatively by named behavior, not by silence.
// RFC requirement: RFC7296-3.15.1-7 negative -- the two options are distinguishable, and ze
// takes exactly one of them. The response is present. This is therefore not the degenerate
// case of no response at all. The response also holds no Configuration payload. It is
// therefore not the empty-CFG_ACK option either.
func TestCFGSetIsIgnoredAndDrawsNoCFGACK(t *testing.T) {
	cfgSet := []wire.PayloadEntry{{Payload: &wire.PayloadCP{
		CFGType: wire.CFGTypeSet,
		Attrs: []wire.ConfigAttr{
			{Type: wire.CPAttrInternalIP4DNS, Value: []byte{9, 9, 9, 9}},
			{Type: wire.CPAttrInternalIP4Netmask, Value: []byte{255, 255, 0, 0}},
		},
	}}}

	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only. The first draft compared
	// response PAYLOAD TYPES. Its own anti-vacuity guard proved that comparison dead.
	// handleInformationalOwned builds every response with a nil payload chain (inbound.go).
	//
	// All three variants therefore returned an empty list. An interpreted payload was
	// indistinguishable from an ignored one. The observable moves to SA state, which a
	// payload ze interprets does change. Assertions are added, none removed or downgraded.
	withSet := driveInformational(t, cfgSet)
	withoutSet := driveInformational(t, nil)

	// The request is answered, and the answer carries no Configuration payload of any type.
	for _, pt := range withSet.respTypes {
		if pt == wire.PayloadTypeCP {
			t.Error("the response to a CP(CFG_SET) carries a Configuration payload; ze " +
				"accepts no configuration data and must answer without a CFG_ACK")
		}
	}

	// The CFG_SET changed nothing: the SA is untouched.
	if withSet.state != StateEstablished {
		t.Errorf("responder SA is %v after a CP(CFG_SET), want established; ignoring a "+
			"CFG_SET must change no state", withSet.state)
	}
	if withSet.state != withoutSet.state {
		t.Errorf("responder SA state with CFG_SET = %v, without = %v", withSet.state,
			withoutSet.state)
	}

	// Negative: the observable is sensitive. A payload ze DOES interpret, reaching the same
	// handler by the same route, changes it. handleDeletePayload sets the SA dead on an IKE
	// Delete (inbound.go), so "CFG_SET changed nothing" is a fact about CFG_SET rather than
	// a comparison that can never fail.
	withDelete := driveInformational(t, []wire.PayloadEntry{
		{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}},
	})
	if withDelete.state == withoutSet.state {
		t.Errorf("an IKE Delete left the SA at %v, the same state as an empty request; the "+
			"observable cannot detect an interpreted payload, so it proves nothing about "+
			"CFG_SET", withDelete.state)
	}

	// No message ze builds carries a CFG_ACK, not only the response above.
	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only. Binds the sweep to this
	// test so the CFG_ACK assertion ranges over every message ze builds.
	built, _, _ := builtCPSweep(t)
	for _, cp := range built {
		if cp.CFGType == wire.CFGTypeACK {
			t.Errorf("ze built a CP(CFG_ACK) with %d attributes; it accepts no configuration "+
				"data, so no attribute can appear in one", len(cp.Attrs))
		}
	}

	// The response shape is unchanged by the CFG_SET.
	if len(withSet.respTypes) != len(withoutSet.respTypes) {
		t.Fatalf("response payload types with CFG_SET = %v, without = %v; ignoring a CFG_SET "+
			"must leave the response unchanged", withSet.respTypes, withoutSet.respTypes)
	}

	// Negative: a CFG_ACK is expressible, so its absence is a decision.
	ack := &wire.PayloadCP{CFGType: wire.CFGTypeACK, Attrs: []wire.ConfigAttr{
		{Type: wire.CPAttrInternalIP4DNS},
	}}
	buf := make([]byte, ack.Len())
	ack.WriteTo(buf, 0)
	var back wire.PayloadCP
	if err := back.ReadFrom(buf); err != nil {
		t.Fatalf("ReadFrom of a CFG_ACK: %v", err)
	}
	if back.CFGType != wire.CFGTypeACK || len(back.Attrs) != 1 {
		t.Fatalf("CFG_ACK round trip gave CFG type %d with %d attributes, want %d with 1; "+
			"the absence above must be a choice rather than a payload ze cannot build",
			back.CFGType, len(back.Attrs), wire.CFGTypeACK)
	}
}
