package filter_irr

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// TestIRRRowsFollowTheConfiguredNotation proves the AS number of an IRR row
// carries the notation bgp/as-notation selected. The method answers
// `show bgp irr` once per notation and compares the payload text.
//
// VALIDATES: showIRR writes the AS number through asn.JSONValue.
// PREVENTS: an operator reading 1.10 in the route table and 65546 in the IRR
// state that filtered it.
func TestIRRRowsFollowTheConfiguredNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })

	plug := &irrPlugin{
		config:      &irrConfig{Server: "whois.example.net"},
		lastRefresh: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		byASN: map[uint32]*asnState{
			65546: {asn: 65546, asSet: "AS-EXAMPLE", peerAddrs: []string{"192.0.2.1"}},
		},
	}

	configureNotation(t, asn.NotationPlain)
	if plain := irrPayload(t, plug); !strings.Contains(plain, `{"asn":65546`) {
		t.Errorf("asplain irr = %s, want the AS number as a JSON number", plain)
	}

	configureNotation(t, asn.NotationDot)
	dotted := irrPayload(t, plug)
	if !strings.Contains(dotted, `{"asn":"1.10"`) {
		t.Errorf("asdot irr = %s, want {\"asn\":\"1.10\"", dotted)
	}
	if strings.Contains(dotted, "65546") {
		t.Errorf("asdot irr = %s, want no decimal spelling of AS 65546", dotted)
	}
}

// irrPayload returns the JSON `show bgp irr` answered with.
func irrPayload(t *testing.T, plug *irrPlugin) string {
	t.Helper()
	status, data, err := plug.showIRR()
	if err != nil {
		t.Fatalf("showIRR: %v", err)
	}
	if status != statusDone {
		t.Fatalf("showIRR status = %s, want %s", status, statusDone)
	}
	raw, ok := data.(json.RawMessage)
	if !ok {
		t.Fatalf("showIRR answered %T, want json.RawMessage", data)
	}
	return string(raw)
}

// configureNotation records one notation for the rest of a test.
func configureNotation(t *testing.T, notation asn.Notation) {
	t.Helper()
	if err := asn.Configure(notation.String()); err != nil {
		t.Fatalf("asn.Configure(%s): %v", notation, err)
	}
}
