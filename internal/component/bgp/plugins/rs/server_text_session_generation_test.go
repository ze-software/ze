// Design: docs/architecture/api/text-parser.md -- State Parser: parseTextState
// Related: server_text.go -- parseTextState

package rs

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestParseTextStateSessionGeneration proves the route server reads the
// session a text down names, and refuses a line whose generation it cannot
// read rather than departing with a guessed one. Method: parseTextState over
// a well-formed down, a down whose token has no value, and one whose value
// is not a number.
func TestParseTextStateSessionGeneration(t *testing.T) {
	event := parseTextState("peer 192.0.2.1 remote as 65001 state down reason tcp-failure session-generation 9")
	if event == nil {
		t.Fatal("a well-formed down was refused")
	}
	if event.SessionGeneration != 9 {
		t.Fatalf("SessionGeneration = %d, want 9", event.SessionGeneration)
	}
	if event.State != "down" || event.Reason != "tcp-failure" {
		t.Fatalf("state %q reason %q, want down tcp-failure", event.State, event.Reason)
	}
	for _, line := range []string{
		"peer 192.0.2.1 remote as 65001 state down session-generation",
		"peer 192.0.2.1 remote as 65001 state down session-generation nine",
	} {
		if got := parseTextState(line); got != nil {
			t.Fatalf("parseTextState(%q) = %+v, want nil: a wrong generation bounds the departure wrongly", line, got)
		}
	}
}

// TestParseStructuredStateSessionGeneration proves the structured (DirectBridge)
// down carries the same generation into the route server's event.
func TestParseStructuredStateSessionGeneration(t *testing.T) {
	event := parseStructuredState(&rpc.StructuredEvent{PeerAddress: "192.0.2.1",
		State: rpc.SessionStateDown, SessionGeneration: 9})
	if event == nil {
		t.Fatal("a structured down was refused")
	}
	if event.SessionGeneration != 9 {
		t.Fatalf("SessionGeneration = %d, want 9", event.SessionGeneration)
	}
}
