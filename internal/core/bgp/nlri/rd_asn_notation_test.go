// Design: docs/architecture/wire/nlri.md -- AS-number notation in an RD string
package nlri

import "testing"

// TestParseRDStringReadsEveryNotation proves the administrator field of a
// route distinguisher takes any of the three RFC 5396 spellings. The method
// parses the same RD in each spelling and compares the type and the six value
// bytes.
//
// The configuration reader (bgpconfig.ParseRouteDistinguisher) and this one
// are two parsers of one syntax. They are reached by two entry points: the
// config file, and every command that takes an `rd` word. One answering
// "invalid IP in RD: 1.10" while the other answers 65546 is the defect this
// guards.
//
// VALIDATES: ParseRDString reads asplain, asdot and asdot+ (AC-1).
// PREVENTS: `update text ... rd 1.10:5` failing on a value the config file
// accepts.
func TestParseRDStringReadsEveryNotation(t *testing.T) {
	t.Parallel()
	want, err := ParseRDString("65546:5")
	if err != nil {
		t.Fatalf("ParseRDString(65546:5): %v", err)
	}
	if want.Type != RDType2 {
		t.Fatalf("RD type = %d, want %d for a four-byte AS number", want.Type, RDType2)
	}

	for _, spelling := range []string{"1.10:5", "2:1.10:5", "2:65546:5"} {
		got, err := ParseRDString(spelling)
		if err != nil {
			t.Fatalf("ParseRDString(%q): %v", spelling, err)
		}
		if got.Type != want.Type || got.Value != want.Value {
			t.Errorf("ParseRDString(%q) = %d/%v, want %d/%v",
				spelling, got.Type, got.Value, want.Type, want.Value)
		}
	}

	// An asdot+ spelling of a two-byte AS number still yields RFC 4364 type 0.
	small, err := ParseRDString("0.100:5")
	if err != nil {
		t.Fatalf("ParseRDString(0.100:5): %v", err)
	}
	plain, err := ParseRDString("100:5")
	if err != nil {
		t.Fatalf("ParseRDString(100:5): %v", err)
	}
	if small.Type != plain.Type || small.Value != plain.Value {
		t.Errorf("ParseRDString(0.100:5) = %d/%v, want %d/%v",
			small.Type, small.Value, plain.Type, plain.Value)
	}

	// The IPv4 form is decided by netip before the AS branch, so it keeps
	// type 1.
	ipForm, err := ParseRDString("192.0.2.1:5")
	if err != nil {
		t.Fatalf("ParseRDString(192.0.2.1:5): %v", err)
	}
	if ipForm.Type != RDType1 {
		t.Errorf("RD type for 192.0.2.1:5 = %d, want %d", ipForm.Type, RDType1)
	}

	// A dotted token that names no AS number is still refused.
	for _, bad := range []string{"1.99999:5", "65536.0:5", "1.2.3:5", "0:1.2.3.4:5"} {
		if _, err := ParseRDString(bad); err == nil {
			t.Errorf("ParseRDString(%q) was accepted", bad)
		}
	}
}

// TestParseRDStringAgreesWithTheConfigReader is the cross-check the defect
// needed: the two parsers must answer the same bytes for one spelling.
//
// It lives here rather than beside the config reader, because this package
// cannot import that one (the config package is a component and this is core).
// The config side asserts the same equality from its own entry point in
// internal/component/bgp/config/asn_notation_test.go.
func TestParseRDStringAgreesWithTheConfigReader(t *testing.T) {
	t.Parallel()
	dotted, err := ParseRDString("1.10:5")
	if err != nil {
		t.Fatalf("ParseRDString(1.10:5): %v", err)
	}
	// bgpconfig.ParseRouteDistinguisher writes the same eight bytes: two type
	// bytes then the six value bytes.
	wantWire := [8]byte{0x00, 0x02, 0x00, 0x01, 0x00, 0x0a, 0x00, 0x05}
	var gotWire [8]byte
	copy(gotWire[:], dotted.Bytes())
	if gotWire != wantWire {
		t.Errorf("rd 1.10:5 wire = %v, want %v", gotWire, wantWire)
	}
}

// TestParseRDStringDeclaredTypeWins proves a type prefix survives the round
// trip through String(), which is the only reason the prefix is written.
//
// RFC 4364 Section 4.2 gives Type 0 a 2-octet administrator and Type 2 a
// 4-octet one, so an AS number of 65535 or less fits both and the prefix is
// what tells them apart. Inferring the type from the magnitude threw that
// away: `2:65000:100` came back as Type 0.
//
// VALIDATES: ParseRDString honors a declared type, and refuses one the
// administrator cannot hold.
// PREVENTS: String() promising a disambiguation ParseRDString discards, so an
// RD read from a show output and pasted into a command names another VRF.
func TestParseRDStringDeclaredTypeWins(t *testing.T) {
	t.Parallel()
	// The round trip both ways: a small AS number in each type.
	for _, tc := range []struct {
		text string
		want RDType
		// String() writes the AS number in decimal, so only a decimal input
		// comes back as the text that was typed.
		roundTrips bool
	}{
		{"0:65000:100", RDType0, true},
		{"2:65000:100", RDType2, true},
		{"0:0.100:100", RDType0, false},
		{"2:0.100:100", RDType2, false},
		{"65000:100", RDType0, false}, // undeclared: the magnitude decides
		{"65546:100", RDType2, false}, // undeclared: four octets needed
		{"1.10:100", RDType2, false},  // undeclared, dotted: four octets needed
	} {
		rd, err := ParseRDString(tc.text)
		if err != nil {
			t.Fatalf("ParseRDString(%q): %v", tc.text, err)
		}
		if rd.Type != tc.want {
			t.Errorf("ParseRDString(%q) type = %d, want %d", tc.text, rd.Type, tc.want)
		}
		if tc.roundTrips && rd.String() != tc.text {
			t.Errorf("ParseRDString(%q).String() = %q, want the text back", tc.text, rd.String())
		}
	}

	// The dotted spellings still name the same route distinguisher as the
	// decimal one, type included.
	dotted, err := ParseRDString("2:0.100:100")
	if err != nil {
		t.Fatalf("ParseRDString(2:0.100:100): %v", err)
	}
	plain, err := ParseRDString("2:100:100")
	if err != nil {
		t.Fatalf("ParseRDString(2:100:100): %v", err)
	}
	if dotted != plain {
		t.Errorf("ParseRDString(2:0.100:100) = %v, want %v", dotted, plain)
	}

	// A declared type the administrator cannot hold is refused rather than
	// corrected, in either spelling.
	for _, bad := range []string{"0:65546:100", "0:1.10:100"} {
		if rd, err := ParseRDString(bad); err == nil {
			t.Errorf("ParseRDString(%q) = type %d, want an error", bad, rd.Type)
		}
	}
}
