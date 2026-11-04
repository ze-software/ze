// Design: docs/plugin-development/protocol.md -- The filter-update batch.
package rpc

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// TestFilterInputJSONSizeBoundCoversCanonicalEncoding compares the bound with
// the actual encoder for optional values, base64 remainders, HTML and control
// escaping, Unicode separators and invalid UTF-8 in ordinary string fields.
// It tests serialized behavior, not the struct's field or tag population.
func TestFilterInputJSONSizeBoundCoversCanonicalEncoding(t *testing.T) {
	for _, text := range []string{"", "ordinary", "<>&", "\x00\b\f\n\r\t", "\u2028\u2029", string([]byte{0xff, 0xfe})} {
		for _, fields := range []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage(` { "med" : 20 } `), json.RawMessage("\"<>&\u2028\u2029\"")} {
			for rawSize := range 7 {
				input := FilterUpdateInput{Filter: text, Direction: text, Peer: text, PeerAS: math.MaxUint32,
					Fields: fields, Raw: make([]byte, rawSize)}
				encoded, err := json.Marshal(&input)
				if err != nil {
					t.Fatal(err)
				}
				bound := input.JSONSizeBound()
				if bound < len(encoded) || bound > MaxMessageSize+1 {
					t.Fatalf("bound=%d encoded=%d text=%q fields=%q raw=%d", bound, len(encoded), text, fields, rawSize)
				}
			}
		}
	}
}

// TestFilterInputJSONSizeBoundSaturatesBeforeOverflow compares real oversized
// JSON encodings with the saturation marker, and proves a conservative bound
// may also saturate for a compactable input that still fits on the wire.
func TestFilterInputJSONSizeBoundSaturatesBeforeOverflow(t *testing.T) {
	for _, input := range []FilterUpdateInput{
		{Filter: strings.Repeat("\x00", MaxMessageSize/6+1)},
		{Raw: make([]byte, (MaxMessageSize/4)*3+1)},
	} {
		encoded, err := json.Marshal(&input)
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) <= MaxMessageSize || input.JSONSizeBound() != MaxMessageSize+1 {
			t.Fatalf("oversized encoding=%d bound=%d, want saturation", len(encoded), input.JSONSizeBound())
		}
	}
	input := FilterUpdateInput{Fields: json.RawMessage(strings.Repeat(" ", MaxMessageSize/6+1) + `{"med":20}`)}
	encoded, err := json.Marshal(&input)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= MaxMessageSize || input.JSONSizeBound() != MaxMessageSize+1 {
		t.Fatalf("compactable encoding=%d bound=%d, want fit with conservative saturation", len(encoded), input.JSONSizeBound())
	}
}

// TestFilterInputJSONSizeBoundAllocatesNothing keeps the ordinary-path proof
// from introducing another per-route encoding or allocation before CallRPC.
func TestFilterInputJSONSizeBoundAllocatesNothing(t *testing.T) {
	input := FilterUpdateInput{Filter: "judge", Direction: "import", Peer: "192.0.2.4", PeerAS: 65001,
		Fields: json.RawMessage(`{"med":20}`), Raw: []byte{0, 0, 0, 0}}
	var bound int
	allocs := testing.AllocsPerRun(100, func() { bound = input.JSONSizeBound() })
	if allocs != 0 || bound >= MaxMessageSize {
		t.Fatalf("ordinary bound=%d allocations=%.0f, want a small allocation-free proof", bound, allocs)
	}
}
