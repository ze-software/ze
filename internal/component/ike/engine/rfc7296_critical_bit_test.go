// VALIDATES: RFC 7296 critical-bit obligations on the send side. Every payload ze emits
// carries a clear critical bit, at the outer chain, at the SK payload's own generic header,
// and inside the encrypted inner chain. A Vendor ID payload received from a peer changes no
// interpretation of the payloads this specification defines.
// PREVENTS: a response builder that echoes a parsed payload entry back to the peer. Such an
// echo carries the peer's critical bit straight out again. It also prevents a new builder
// that sets the field directly.
package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// builtChain is one message the engine emits, with both payload chains resolved. outer is
// the chain in the message itself. inner is the chain inside the SK payload, and it is nil
// for a plaintext message.
type builtChain struct {
	name       string
	raw        []byte
	isResponse bool
	outer      []wire.PayloadEntry
	inner      []wire.PayloadEntry
}

// engineBuiltChains returns one message of every kind the engine emits, with the encrypted
// inner chain decrypted. The peer SA decrypts, because skRecvEncKey selects SK_er for an
// initiator and SK_ei for a responder. So an initiator SA reads a responder-built message.
func engineBuiltChains(t *testing.T) []builtChain {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)

	// Capture the handshake messages before any later builder runs.
	saInitReq := ini.InitiatorSAInitMsg
	saInitResp := resp.ResponderSAInitMsg
	authReq := ini.LastSentMsg
	authResp := resp.LastSentMsg

	probe := &wire.Message{Header: wire.Header{
		InitiatorSPI: resp.InitiatorSPI, ResponderSPI: resp.ResponderSPI,
		MajorVersion: 2, ExchangeType: wire.ExchangeInformational,
		Flags: wire.FlagInitiator, MessageID: resp.ExpectedMsgID,
	}}
	resp.lastResponse = nil
	resp.lastResponseSet = false
	ps.handleInformationalOwned(resp, probe, nil, false, nil, nil, log)
	if resp.lastResponse == nil {
		t.Fatal("no INFORMATIONAL response was built")
	}
	informationalResp := resp.lastResponse

	delReq, err := buildEncryptedMessageEx(ini,
		[]wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}},
		ini.NextMsgID, wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}

	rekeyReq, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	t.Cleanup(pending.clear)

	sources := []struct {
		name       string
		raw        []byte
		isResponse bool
		peer       *SA
	}{
		{"IKE_SA_INIT request", saInitReq, false, nil},
		{"IKE_SA_INIT response", saInitResp, true, nil},
		{"IKE_AUTH request", authReq, false, resp},
		{"IKE_AUTH response", authResp, true, ini},
		{"INFORMATIONAL response", informationalResp, true, ini},
		{"INFORMATIONAL Delete request", delReq, false, resp},
		{"CREATE_CHILD_SA rekey request", rekeyReq, false, resp},
	}

	out := make([]builtChain, 0, len(sources))
	for _, s := range sources {
		msg := parseMsg(t, s.raw)
		bc := builtChain{
			name: s.name, raw: s.raw, isResponse: s.isResponse, outer: msg.Payloads,
		}
		if s.peer != nil {
			inner, err := decryptAndParse(s.peer, msg, s.raw)
			if err != nil {
				t.Fatalf("%s: decryptAndParse: %v", s.name, err)
			}
			bc.inner = inner
		}
		out = append(out, bc)
	}
	return out
}

// definedPayloadType reports whether a payload type is one RFC 7296 Section 3.2 defines.
// The document assigns 33 through 48.
func definedPayloadType(pt uint8) bool { return pt >= 33 && pt <= 48 }

// RFC requirement: RFC7296-3.2-4 positive -- every payload ze sends whose type RFC 7296 defines
// carries a clear critical bit. The rule is SEND SIDE ONLY. RFC7296-3.2-2 governs the recipient
// and requires the opposite, so ze adds no receive-side rejection of a defined type carrying a
// set bit. The sweep covers both nesting levels of every message the engine builds. Ze has three
// generic-header producers: Message.WriteTo (wire/message.go) for the outer chain,
// buildEncryptedMessageEx (auth.go) for the inner chain, and writeAuthHeaderWithMsgID (auth.go)
// for the SK payload's own header. The third names no Critical field at all.
// RFC requirement: RFC7296-3.2-4 negative -- the encoder honors the field. A defined type asked
// to be critical reaches the wire with octet 1 of its generic header at 0x80, and
// Message.ReadFrom recovers the value. The clear bit above is a builder decision rather than an
// encoder that cannot express anything else.
func TestDefinedPayloadTypesAreSentUncritical(t *testing.T) {
	seen := make(map[uint8]bool)
	for _, m := range engineBuiltChains(t) {
		for _, level := range []struct {
			where   string
			entries []wire.PayloadEntry
		}{{"outer", m.outer}, {"inner", m.inner}} {
			for _, pe := range level.entries {
				pt := pe.Payload.Type()
				if !definedPayloadType(pt) {
					continue
				}
				seen[pt] = true
				if pe.Critical {
					t.Errorf("%s %s chain: payload type %d is defined in RFC 7296 and "+
						"carries a set critical bit", m.name, level.where, pt)
				}
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no payload of a defined type was observed, so the assertion is vacuous")
	}
	t.Logf("observed %d distinct defined payload types: %v", len(seen), sortedTypes(seen))

	// The encoder honors the field. A defined type asked to be critical reaches the wire
	// with octet 1 of its generic header at 0x80, and the parser recovers the value. The
	// clear bit on every built message is therefore a decision of the builders.
	msg := wire.Message{
		Header: wire.Header{MajorVersion: 2, ExchangeType: wire.ExchangeInformational},
		Payloads: []wire.PayloadEntry{
			{Payload: &wire.PayloadNonce{NonceData: make([]byte, 32)}, Critical: true},
		},
	}
	buf := make([]byte, 256)
	n := msg.WriteTo(buf, 0)
	if got := buf[wire.HeaderLen+1]; got != 0x80 {
		t.Fatalf("the encoder wrote octet 1 of the generic header as %#02x for an explicit "+
			"critical payload, want 0x80", got)
	}
	back := parseMsg(t, buf[:n])
	if len(back.Payloads) != 1 || !back.Payloads[0].Critical {
		t.Error("the parser did not recover a set critical bit, so a clear bit proves nothing")
	}
}

// sortedTypes returns the observed payload types in ascending order for a stable log line.
func sortedTypes(seen map[uint8]bool) []int {
	out := make([]int, 0, len(seen))
	for pt := range seen {
		out = append(out, int(pt))
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// RFC requirement: RFC7296-2.5-17 positive -- no payload in any response ze sends has the
// critical flag set, whatever its type, at either nesting level. The scope is wider than
// RFC7296-3.2-4, which covers defined types only. All ten response builders construct fresh
// wire.PayloadEntry values from negotiated state, and none echoes a parsed entry back. The
// counts named in the failure message cover all three generic-header producers: the outer chain,
// the SK payload's own header, and the decrypted inner chain.
// RFC requirement: RFC7296-2.5-17 negative -- the rule is scoped to responses, and the sample
// distinguishes them, so "every response" is not "every message that exists". Separately,
// buildEncryptedMessageEx DOES propagate a set bit when a caller asks for one. The clear bit in
// the response builders is therefore their choice.
func TestResponsePayloadsAreNeverCritical(t *testing.T) {
	var outerCount, innerCount, multiOuter, skHeaders, requests, responses int

	for _, m := range engineBuiltChains(t) {
		if !m.isResponse {
			requests++
			continue
		}
		responses++
		if len(m.outer) > 1 {
			multiOuter++
		}
		for _, pe := range m.outer {
			outerCount++
			if pe.Payload.Type() == wire.PayloadTypeSK {
				skHeaders++
			}
			if pe.Critical {
				t.Errorf("%s: an outer payload of type %d in a response carries a set "+
					"critical flag", m.name, pe.Payload.Type())
			}
		}
		for _, pe := range m.inner {
			innerCount++
			if pe.Critical {
				t.Errorf("%s: an inner payload of type %d in a response carries a set "+
					"critical flag", m.name, pe.Payload.Type())
			}
		}
	}

	// Anti-vacuity. Each count names one of the three generic-header producers. A zero in
	// any of them means the loop above asserted nothing about that producer.
	if requests == 0 || responses == 0 {
		t.Fatalf("the sample holds %d requests and %d responses, and the rule is scoped to "+
			"responses. Both must appear", requests, responses)
	}
	if outerCount == 0 || innerCount == 0 || multiOuter == 0 || skHeaders == 0 {
		t.Fatalf("response payloads examined: outer=%d inner=%d, responses with a "+
			"multi-payload outer chain=%d, SK generic headers=%d. Every count must be "+
			"positive or the assertion passes over an empty set",
			outerCount, innerCount, multiOuter, skHeaders)
	}

	// buildEncryptedMessageEx propagates a set bit when a caller asks for one. The clear bit
	// in every response builder is therefore the builder's choice.
	ini, resp, _ := establishPSK(t)
	raw, err := buildEncryptedMessageEx(ini,
		[]wire.PayloadEntry{{Payload: &wire.PayloadNonce{NonceData: make([]byte, 32)}, Critical: true}},
		ini.NextMsgID, wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decryptAndParse: %v", err)
	}
	if len(inner) != 1 || !inner[0].Critical {
		t.Error("buildEncryptedMessageEx dropped an explicitly critical inner payload, so a " +
			"clear bit in the response builders proves nothing")
	}
}

// RFC requirement: RFC7296-3.12-4 positive -- a Vendor ID payload changes no interpretation of
// the information this specification defines. The differential runs handleSAInitRequest twice
// over one initiator SA, and only one arm appends a Vendor ID. It compares the responder
// state, the chosen proposal, and the response payload type sequence. Raw bytes cannot be
// compared, because the responder nonce and DH public value are fresh per run. Ze constructs
// no Vendor ID payload today, and RFC7296-3.2-4 and RFC7296-2.5-17 cover the clause's
// critical-bit half over every payload type. This is the ENGINE-layer view over the
// negotiation outcome, and RFC7296-3.12-2 is the WIRE-layer view over the payload octets.
//
// RFC requirement: RFC7296-3.12-4 negative -- the differential is sensitive. A payload this
// specification DOES interpret changes the outcome, and the same mechanism appends it. A KE
// payload names a group ze did not select. The responder answers INVALID_KE_PAYLOAD and the SA
// dies. So "no change" is a fact about the Vendor ID, not a comparison that can never fail.
func TestVendorIDDoesNotChangeInterpretation(t *testing.T) {
	log := slogutil.DiscardLogger()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "vendorid-psk")

	// One initiator SA for every arm, so the request differs only by the appended payload.
	ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	base := buildSAInitRequest(ini, testIKEGroup())

	withExtra := func(extra ...wire.PayloadEntry) []byte {
		msg := parseMsg(t, base)
		msg.Payloads = append(msg.Payloads, extra...)
		buf := make([]byte, 4096)
		n, err := msg.CheckedWriteTo(buf, 0)
		if err != nil {
			t.Fatalf("CheckedWriteTo: %v", err)
		}
		return buf[:n]
	}

	// outcome captures everything the responder decided, minus the values that are fresh on
	// every run. The nonce and the DH public value differ per run, so raw bytes cannot be
	// compared.
	type outcome struct {
		state     SAState
		number    uint16
		encID     uint16
		prfID     uint16
		integID   uint16
		dhID      int
		respTypes []uint8
	}
	run := func(req []byte) outcome {
		sa, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
		if err != nil {
			t.Fatalf("newResponderSA: %v", err)
		}
		handleSAInitRequest(sa, parseMsg(t, req), req, nil, nil, log)
		o := outcome{
			state:  sa.State,
			number: sa.Proposal.Number,
			encID:  uint16(sa.Proposal.Encryption.ID),
			prfID:  uint16(sa.Proposal.PRF.ID),
			//nolint:gosec // G115: transform IDs are IANA 16-bit registry values.
			integID: uint16(sa.Proposal.Integrity.ID),
			dhID:    int(sa.Proposal.DHGroup.ID),
		}
		if len(sa.LastSentMsg) > 0 {
			for _, pe := range parseMsg(t, sa.LastSentMsg).Payloads {
				o.respTypes = append(o.respTypes, pe.Payload.Type())
			}
		}
		return o
	}

	vendorReq := withExtra(wire.PayloadEntry{
		Payload: &wire.PayloadVendorID{VendorIDData: []byte("ze-wp5-vendor-id-probe")},
	})

	// Anti-vacuity: the Vendor ID really reached the parser as a Vendor ID payload.
	sawVendorID := false
	for _, pe := range parseMsg(t, vendorReq).Payloads {
		if _, ok := pe.Payload.(*wire.PayloadVendorID); ok {
			sawVendorID = true
		}
	}
	if !sawVendorID {
		t.Fatal("the modified request carries no Vendor ID payload, so the differential " +
			"compares two identical inputs")
	}

	plain := run(base)
	vendor := run(vendorReq)

	if plain.state != StateSAInitReceived {
		t.Fatalf("the baseline responder state is %v, want sa-init-received", plain.state)
	}
	if vendor.state != plain.state {
		t.Errorf("the Vendor ID changed the responder state to %v, and the baseline is %v",
			vendor.state, plain.state)
	}
	if vendor.number != plain.number || vendor.encID != plain.encID ||
		vendor.prfID != plain.prfID || vendor.integID != plain.integID ||
		vendor.dhID != plain.dhID {
		t.Errorf("the Vendor ID changed the chosen proposal: %+v, and the baseline is %+v",
			vendor, plain)
	}
	if len(plain.respTypes) == 0 {
		t.Fatal("the baseline built no response, so the payload-sequence comparison is vacuous")
	}
	if len(vendor.respTypes) != len(plain.respTypes) {
		t.Fatalf("the response holds %d payloads with the Vendor ID and %d without",
			len(vendor.respTypes), len(plain.respTypes))
	}
	for i := range plain.respTypes {
		if vendor.respTypes[i] != plain.respTypes[i] {
			t.Errorf("response payload %d is type %d with the Vendor ID and type %d without",
				i, vendor.respTypes[i], plain.respTypes[i])
		}
	}

	// The differential is sensitive. A payload this specification DOES interpret changes the
	// outcome through the same insertion mechanism. A KE payload naming a group ze did not
	// select is answered with INVALID_KE_PAYLOAD, and the SA dies.
	otherGroup := uint16(testIKEGroup().Proposals[0].DHGroup) + 1
	interpreted := run(withExtra(wire.PayloadEntry{
		Payload: &wire.PayloadKE{DHGroup: otherGroup, KeyExchangeData: make([]byte, 256)},
	}))
	if interpreted.state == plain.state {
		t.Errorf("appending a KE payload for group %d left the responder state at %v. The "+
			"comparison cannot detect an interpretation change", otherGroup, plain.state)
	}
}

// RFC requirement: RFC7296-3.2-4 positive -- this covers the producer the message harness cannot
// enumerate. The behavioral tests range over the messages ze builds today. A future builder
// that sets the critical bit, and that engineBuiltChains does not hold, would pass them. This
// walks the engine package source instead and refuses the field outright.
//
// TestEngineSourceNeverSetsTheCriticalField is a source-level assertion beside the
// behavioral pair above. Those tests cover the messages the harness enumerates. A future
// builder outside the harness would pass them. This walks the engine package instead.
//
// Two rules. No composite literal names a Critical field. No assignment writes to a
// .Critical selector unless the value comes from another .Critical selector. The second
// rule admits the one legitimate writer, buildEncryptedMessageEx, which copies the caller's
// value. It refuses a constant, a call result, or any other expression.
//
// The scope is the engine package. Both parse-path writers live in the wire package, and
// they read the bit off the wire.
func TestEngineSourceNeverSetsTheCriticalField(t *testing.T) {
	// The real package holds no violation, and it holds the one admitted propagation.
	violations, propagations, scanned := scanCriticalWrites(t, ".")
	for _, v := range violations {
		t.Error(v)
	}
	if scanned == 0 {
		t.Fatal("no engine source file was scanned, so the walk asserts nothing")
	}
	if propagations == 0 {
		t.Errorf("the walk over %d files found no Critical propagation. "+
			"buildEncryptedMessageEx copies the caller's value, so the rule that admits it "+
			"is untested", scanned)
	}

	// The walk is mutation-proof by construction. A fixture directory carries one file per
	// rule, and the detector must flag the two violations and admit the propagation.
	// go -overlay cannot reach this walk, because it reads source at run time rather than
	// at build time. The fixture is the mutation.
	dir := t.TempDir()
	fixtures := map[string]string{
		"literal.go":   "package p\n\nfunc f() any { return T{Critical: true} }\n",
		"assign.go":    "package p\n\nfunc g(x T) { x.Critical = true }\n",
		"propagate.go": "package p\n\nfunc h(dst T, src T) { dst.Critical = src.Critical }\n",
		"clean.go":     "package p\n\nfunc i() any { return T{Payload: nil} }\n",
	}
	for name, body := range fixtures {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	got, gotProp, gotScanned := scanCriticalWrites(t, dir)
	if gotScanned != len(fixtures) {
		t.Fatalf("the fixture walk scanned %d files, want %d", gotScanned, len(fixtures))
	}
	if len(got) != 2 {
		t.Errorf("the fixture walk reported %d violations, want 2 (one literal, one "+
			"assignment). Reported: %v", len(got), got)
	}
	if gotProp != 1 {
		t.Errorf("the fixture walk counted %d propagations, want 1. A legitimate copy of "+
			"another Critical field must be admitted", gotProp)
	}
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "literal.go") || !strings.Contains(joined, "assign.go") {
		t.Errorf("the fixture walk missed a violating file. Reported: %v", got)
	}
	if strings.Contains(joined, "propagate.go") || strings.Contains(joined, "clean.go") {
		t.Errorf("the fixture walk flagged a clean file. Reported: %v", got)
	}
}

// scanCriticalWrites parses every non-test Go file in dir and applies two rules. No
// composite literal names a Critical field. No assignment writes a .Critical selector
// unless the value comes from another .Critical selector. It returns the violation
// messages, the count of admitted propagations, and the count of files parsed.
func scanCriticalWrites(t *testing.T, dir string) (violations []string, propagations, scanned int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		scanned++

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CompositeLit:
				for _, elt := range node.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Critical" {
						violations = append(violations, fset.Position(kv.Pos()).String()+
							": a composite literal names a Critical field. The engine must "+
							"leave every payload uncritical")
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range node.Lhs {
					if !isCriticalSelector(lhs) {
						continue
					}
					if i < len(node.Rhs) && isCriticalSelector(node.Rhs[i]) {
						propagations++
						continue
					}
					violations = append(violations, fset.Position(lhs.Pos()).String()+
						": an assignment writes a Critical field from something other than "+
						"another Critical field")
				}
			}
			return true
		})
	}
	return violations, propagations, scanned
}

// isCriticalSelector reports whether an expression is a selector ending in .Critical.
func isCriticalSelector(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel != nil && sel.Sel.Name == "Critical"
}
