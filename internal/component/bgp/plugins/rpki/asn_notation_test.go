package rpki

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// TestROARowsFollowTheConfiguredNotation proves the AS number of a ROA row
// carries the notation bgp/as-notation selected, on the two commands an
// operator reads a ROA through. The method answers each command once per
// notation and compares the payload text.
//
// VALIDATES: roaCommand and roaLookupCommand write the AS number through
// asn.JSONValue.
// PREVENTS: an operator reading 1.10 in the route table and 65546 in the ROA
// that validates it.
func TestROARowsFollowTheConfiguredNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })

	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65546))

	configureNotation(t, asn.NotationPlain)
	plain := payloadOf(t, commandResult(rp.roaCommand(nil)))
	if !strings.Contains(plain, `"asn":65546`) {
		t.Errorf("asplain roa = %s, want the AS number as a JSON number", plain)
	}

	configureNotation(t, asn.NotationDot)
	dotted := payloadOf(t, commandResult(rp.roaCommand(nil)))
	if !strings.Contains(dotted, `"asn":"1.10"`) {
		t.Errorf("asdot roa = %s, want \"asn\":\"1.10\"", dotted)
	}

	lookup := payloadOf(t, commandResult(rp.roaCommand([]string{"10.0.0.0/24"})))
	if !strings.Contains(lookup, `"asn":"1.10"`) {
		t.Errorf("asdot roa lookup = %s, want \"asn\":\"1.10\"", lookup)
	}
}

// TestASPACustomerFollowsTheConfiguredNotation proves the ASPA rows an
// operator reads carry the notation, customer and providers alike, and that
// the customer argument is accepted in any notation. The method looks one
// customer up by its asdot spelling.
//
// VALIDATES: aspaCommand reads its argument through asn.Parse and writes both
// AS numbers through asn.JSONValue.
// PREVENTS: an operator having to type the decimal form of an AS number the
// same daemon just printed to them as 1.10.
func TestASPACustomerFollowsTheConfiguredNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })
	configureNotation(t, asn.NotationDot)

	rp := newTestPlugin()
	rp.aspaCache.Set(65546, []uint32{65547})

	found := payloadOf(t, commandResult(rp.aspaCommand([]string{"1.10"})))
	if !strings.Contains(found, `"customer-asn":"1.10"`) {
		t.Errorf("aspa lookup = %s, want \"customer-asn\":\"1.10\"", found)
	}
	if !strings.Contains(found, `"found":true`) {
		t.Errorf("aspa lookup = %s, want the record the asdot argument names", found)
	}
	if !strings.Contains(found, `"providers":["1.11"]`) {
		t.Errorf("aspa lookup = %s, want the providers in asdot too", found)
	}
}

// answer is what an rpki command handler returns, so a call can be passed to
// payloadOf in one expression.
type answer struct {
	status string
	data   any
	err    error
}

func commandResult(status string, data any, err error) answer {
	return answer{status: status, data: data, err: err}
}

// payloadOf returns the JSON a command answered with.
func payloadOf(t *testing.T, got answer) string {
	t.Helper()
	if got.err != nil {
		t.Fatalf("command failed: %v", got.err)
	}
	if got.status != statusDone {
		t.Fatalf("command status = %s, want %s", got.status, statusDone)
	}
	data := got.data
	raw, ok := data.(json.RawMessage)
	if !ok {
		t.Fatalf("command answered %T, want json.RawMessage", data)
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

// TestValidateCommandReadsEveryNotation proves `rpki validate <prefix> <asn>`
// takes the origin AS number in any of the three RFC 5396 spellings, and
// answers the same state for each.
//
// VALIDATES: validateCommand reads asn.Parse (AC-1).
// PREVENTS: an operator reading an origin as 1.10 on a show output and being
// told "invalid ASN: 1.10" when they paste it into the validate command.
func TestValidateCommandReadsEveryNotation(t *testing.T) {
	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65546))

	for _, spelling := range []string{"65546", "1.10"} {
		status, data, err := rp.validateCommand([]string{"10.1.0.0/24", spelling})
		if err != nil {
			t.Fatalf("validate 10.1.0.0/24 %s: %v", spelling, err)
		}
		if status != statusDone {
			t.Errorf("validate %s: status = %q, want %q", spelling, status, statusDone)
		}
		m := parseJSON(t, data)
		if m["state"] != "valid" {
			t.Errorf("validate %s: state = %v, want valid", spelling, m["state"])
		}
	}

	// A token that names no AS number is still refused.
	if _, _, err := rp.validateCommand([]string{"10.1.0.0/24", "1.99999"}); err == nil {
		t.Error("validate accepted an out-of-range asdot AS number")
	}
}
