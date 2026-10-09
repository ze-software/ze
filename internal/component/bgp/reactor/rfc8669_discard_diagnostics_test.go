// Design: docs/architecture/wire/attributes.md -- BGP_PREFIX_SID receive validation.
// Related: rfc8669_duplicate_tlv_test.go -- labeled-unicast IBGP receive fixture.
// Related: rfc7606_session_diagnostics_test.go -- synchronized session log capture.

package reactor

import (
	"bytes"
	"encoding/hex"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC8669MalformedPrefixSIDDiscardDiagnostic observes the real receive
// diagnostic for two isolated framing errors on otherwise valid labeled unicast.
// Method: capture enabled session diagnostics, match the entire original UPDATE,
// and check the published route and surviving attributes after attribute discard.
// This proves logging for actual malformed discards, not completeness of the
// separate malformed/invalid-state detection requirements or peer advertisement.
// RFC 8669 Section 6: "When discarding an attribute, a BGP speaker SHOULD log an error for further analysis."
// MUTATION: Suppress both attribute-discard log calls for code40 in Session.applyRFC7606;
// the original-UPDATE diagnostic assertion must fail despite correct discard.
//
// RFC requirement: RFC8669-6-4 positive -- an actual malformed code40 discard for a TLV overrun or trailing bytes on IPv4 labeled unicast emits an enabled attribute40 discard diagnostic tied to the original UPDATE; the receive result is exactly AttributeDiscard, preserves the route and removes code40.
func TestRFC8669MalformedPrefixSIDDiscardDiagnostic(t *testing.T) {
	// RFC 8669 Section 3.1: one Label-Index TLV; its distinctive value scopes logs to this input.
	clean := labelIndexTLV("86690604")
	for _, tc := range []struct {
		name      string
		prefixSID string
	}{
		{
			name:      "TLV overrun",
			prefixSID: clean[:len(clean)-2],
		},
		{
			name:      "trailing bytes",
			prefixSID: clean + "aa",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// No parallel execution: captureSessionLog swaps a process-global logger.
			log := captureSessionLog(t, slog.LevelDebug)
			// RFC 8669 Sections 3.1 and 6: receive on IBGP, avoiding the EBGP boundary discard.
			received, wu, action, err := rfc8669DuplicateReceive(t, safiLabeled,
				rfc8669PrefixSIDAttr(tc.prefixSID, false), communityAttr, false)
			if err != nil {
				t.Fatalf("receive malformed Prefix-SID: %v", err)
			}
			if action != message.RFC7606ActionAttributeDiscard {
				t.Fatalf("action = %v, want exactly AttributeDiscard", action)
			}
			if wu == nil {
				t.Fatal("attribute discard lost the UPDATE")
			}
			// RFC 8669 Section 6: attribute discard leaves the labeled route intact.
			rfc8669DiscardRoutePreserved(t, received, wu.Payload())
			count, _ := countAttrCode(rfc8669PathAttrs(t, wu.Payload()), uint8(attribute.AttrPrefixSID))
			if count != 0 {
				t.Fatalf("published code40 count = %d, want 0", count)
			}

			// RFC 8669 Section 6: observe emitted text, not a logger callback or validator result.
			lines := rfc8669DiscardDiagnosticLines(log.String(), received)
			if len(lines) != 1 {
				t.Fatalf("original UPDATE has %d diagnostics, want 1; captured log:\n%s", len(lines), log.String())
			}
			for _, field := range []string{
				" event=attribute-discard ",
				" attr=40 ",
				" representation=original ",
			} {
				if !strings.Contains(lines[0], field) {
					t.Errorf("diagnostic lacks %q: %s", field, lines[0])
				}
			}
		})
	}
}

// TestRFC8669CleanPrefixSIDNoDiscardDiagnostic pairs the malformed inputs with
// the well-formed Label-Index under the same enabled diagnostic setup.
// Method: assert byte-for-byte publication, exact ActionNone, retained code40,
// and no code40 discard diagnostic for this original UPDATE. Invalid-state
// detection completeness is outside this test's claim.
// RFC 8669 Section 6: "When discarding an attribute, a BGP speaker SHOULD log an error for further analysis."
// MUTATION: Emit an attribute40 discard diagnostic from Session.applyRFC7606's
// ActionNone branch without changing the UPDATE; this clean control must fail.
//
// RFC requirement: RFC8669-6-4 negative -- the corresponding well-formed Label-Index on IPv4 labeled unicast under the same enabled diagnostics is retained byte-for-byte with ActionNone and emits no attribute40 discard diagnostic for its original UPDATE.
func TestRFC8669CleanPrefixSIDNoDiscardDiagnostic(t *testing.T) {
	log := captureSessionLog(t, slog.LevelDebug)
	// RFC 8669 Sections 3.1 and 6: the untruncated Label-Index is the clean control.
	clean := labelIndexTLV("86690604")
	// RFC 8669 Section 6: exercise the same receive entry with diagnostics still enabled.
	received, wu, action, err := rfc8669DuplicateReceive(t, safiLabeled,
		rfc8669PrefixSIDAttr(clean, false), communityAttr, false)
	if err != nil {
		t.Fatalf("receive clean Prefix-SID: %v", err)
	}
	if action != message.RFC7606ActionNone {
		t.Fatalf("action = %v, want exactly ActionNone", action)
	}
	if wu == nil {
		t.Fatal("clean receive lost the UPDATE")
	}
	if !bytes.Equal(received, wu.Payload()) {
		t.Fatalf("clean UPDATE changed: got %x, want %x", wu.Payload(), received)
	}
	count, value := countAttrCode(rfc8669PathAttrs(t, wu.Payload()), uint8(attribute.AttrPrefixSID))
	if count != 1 {
		t.Fatalf("published code40 count = %d, want 1", count)
	}
	if hex.EncodeToString(value) != clean {
		t.Fatalf("published Label-Index = %x, want %s", value, clean)
	}
	// RFC 8669 Section 6: unrelated live sessions' logs cannot satisfy or fail this assertion.
	for _, line := range rfc8669DiscardDiagnosticLines(log.String(), received) {
		if strings.Contains(line, " event=attribute-discard ") && strings.Contains(line, " attr=40 ") {
			t.Errorf("clean original UPDATE has an attribute40 discard diagnostic: %s", line)
		}
	}
}

// rfc8669DiscardDiagnosticLines selects actual diagnostic records by exact
// original UPDATE, including its header. The logger has no session-id field;
// body identity ties each record to the IBGP receive fixture, not other sessions
// writing concurrently through the process-global synchronized capture.
// RFC 8669 Section 6: "When discarding an attribute, a BGP speaker SHOULD log an error for further analysis."
func rfc8669DiscardDiagnosticLines(output string, received []byte) []string {
	// RFC 7606 Section 6: match the complete original UPDATE, not rewritten bytes.
	wire := "update-wire-hex=" + hex.EncodeToString(buildUpdateMsg(received))
	var matching []string
	for line := range strings.SplitSeq(output, "\n") {
		for field := range strings.FieldsSeq(line) {
			if field == wire {
				matching = append(matching, line)
				break
			}
		}
	}
	return matching
}

// rfc8669DiscardRoutePreserved checks the original labeled announcement and
// every non-discarded fixture attribute, without mistaking a withdrawal for
// route preservation. ATTR_TOMBSTONE bookkeeping is intentionally not constrained.
// RFC 7606 Section 2: "In this approach, the malformed attribute MUST be discarded and the UPDATE message continues to be processed."
func rfc8669DiscardRoutePreserved(t *testing.T, received, published []byte) {
	t.Helper()
	original := rfc8669PathAttrs(t, received)
	retained := rfc8669PathAttrs(t, published)
	if !bytes.Equal(published[:2], []byte{0, 0}) {
		t.Fatal("attribute discard introduced withdrawn routes")
	}
	for _, code := range []attribute.AttributeCode{
		attribute.AttrOrigin, attribute.AttrASPath, attribute.AttrMPReachNLRI, attribute.AttrCommunity,
	} {
		beforeCount, before := countAttrCode(original, uint8(code))
		afterCount, after := countAttrCode(retained, uint8(code))
		if beforeCount != 1 {
			t.Fatalf("fixture attribute %d count = %d, want 1", code, beforeCount)
		}
		if afterCount != 1 {
			t.Fatalf("published attribute %d count = %d, want 1", code, afterCount)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("attribute %d changed: got %x, want %x", code, after, before)
		}
	}
	count, _ := countAttrCode(retained, uint8(attribute.AttrMPUnreachNLRI))
	if count != 0 {
		t.Fatalf("attribute discard introduced %d MP_UNREACH attributes", count)
	}
}
