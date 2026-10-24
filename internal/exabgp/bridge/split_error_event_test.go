// Design: docs/architecture/exabgp-bridge.md -- an NLRI field that did not split
// Related: bridge_event.go -- convertUpdateIPC2, unsplitOperation
// Related: bridge_event_text.go -- Event.appendUpdateText

package bridge

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// splitErrorEvent is the UPDATE event the engine publishes when an ipv6
// unicast MP_REACH_NLRI or MP_UNREACH_NLRI does not split into routes: the
// operation carries the error member, `parsed` false and the field's bytes in
// place of its nlri list (docs/architecture/api/json-format.md).
func splitErrorEvent(t *testing.T, operation string) Event {
	t.Helper()
	var zebgp map[string]any
	document := `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},` +
		`"peer":{"remote":{"address":"192.0.2.1","as":65001}},` +
		`"update":{"attr":{"origin":"igp"},"nlri":{"ipv6/unicast":[` + operation + `]}}}}`
	if err := json.Unmarshal([]byte(document), &zebgp); err != nil {
		t.Fatalf("the event does not parse: %v", err)
	}
	return ReadEvent(zebgp)
}

// TestSplitErrorEventIsNotRendered feeds the engine's event for an NLRI field
// that did not split to both encoders.
//
// VALIDATES: neither the ExaBGP JSON (API 4 and 6) nor the ExaBGP text names
// the family, for an announce or a withdraw operation.
// PREVENTS: convertUpdateIPC2 creating the announce family before reading any
// route, so the operation reached a script as "ipv6 unicast":{}, an announce
// of nothing.
func TestSplitErrorEventIsNotRendered(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		operation string
	}{
		{"announce", `{"next-hop":"2001:db8::1","action":"add","error":"parsing NLRI at offset 0: nlri: short read","parsed":false,"raw":"40FDAB"}`},
		{"withdraw", `{"action":"del","error":"parsing NLRI at offset 0: nlri: short read","parsed":false,"raw":"40FDAB"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			event := splitErrorEvent(t, tc.operation)
			for _, format := range []Format{FormatV4JSON, FormatV6JSON} {
				encoded, err := json.Marshal(event.exabgpJSON(format))
				if err != nil {
					t.Fatalf("encode: %v", err)
				}
				if text := string(encoded); strings.Contains(text, "ipv6 unicast") {
					t.Errorf("format %d: a family whose NLRI did not split reached the script:\n%s", format, text)
				}
			}
			text, form := event.AppendText(nil)
			if form != TextWritten {
				t.Fatalf("text form %d, want TextWritten", form)
			}
			if strings.Contains(string(text), "announced") || strings.Contains(string(text), "withdrawn") {
				t.Errorf("a route line was written for an NLRI field that did not split:\n%s", text)
			}
		})
	}
}

// TestUnsplitOperationIsLogged checks the warning that replaces the operation
// the encoders leave out.
//
// VALIDATES: an operation carrying the error member is reported as skipped,
// with a warning naming the peer, the family, the action, the reason, the
// next step and the bytes, bounded with the field length stated; an operation
// with an nlri list is not.
// PREVENTS: the operation being dropped with no line in the log, which is how
// it was dropped before the error member existed.
func TestUnsplitOperationIsLogged(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))

	failed := map[string]any{"action": "del", "error": "nlri: short read", "parsed": false, "raw": "40FDAB"}
	if !unsplitOperation(log, "192.0.2.1", "ipv6/unicast", failed) {
		t.Fatal("an operation carrying the error member was not reported as skipped")
	}
	line := out.String()
	for _, want := range []string{"level=WARN", "peer=192.0.2.1", "family=ipv6/unicast", "action=del", `error="nlri: short read"`, "raw-octets=3", "raw=40FDAB", "show bgp peer <peer> capabilities", "ADD-PATH"} {
		if !strings.Contains(line, want) {
			t.Errorf("the warning lacks %s:\n%s", want, line)
		}
	}

	// A field longer than the bound logs its first unsplitRawHexMax hex
	// characters and its whole length, so a peer repeating it cannot flood
	// the log with its bytes.
	out.Reset()
	long := strings.Repeat("AB", 1000)
	flood := map[string]any{"action": "add", "error": "nlri: short read", "parsed": false, "raw": long}
	if !unsplitOperation(log, "192.0.2.1", "ipv6/unicast", flood) {
		t.Fatal("a long operation carrying the error member was not reported as skipped")
	}
	line = out.String()
	if !strings.Contains(line, "raw-octets=1000") {
		t.Errorf("the warning does not state the field's length:\n%s", line)
	}
	if strings.Contains(line, long[:unsplitRawHexMax+2]) {
		t.Errorf("the warning logs more than %d hex characters:\n%s", unsplitRawHexMax, line)
	}
	if !strings.Contains(line, "raw="+long[:unsplitRawHexMax]+"...") {
		t.Errorf("the warning lacks the bounded hex:\n%s", line)
	}

	out.Reset()
	parsed := map[string]any{"action": "add", "next-hop": "2001:db8::1", "nlri": []any{"2001:db8::/32"}}
	if unsplitOperation(log, "192.0.2.1", "ipv6/unicast", parsed) {
		t.Fatal("an operation with an nlri list was reported as skipped")
	}
	if out.Len() != 0 {
		t.Errorf("a warning was logged for an operation with routes:\n%s", out.String())
	}
}
