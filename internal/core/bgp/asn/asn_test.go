package asn

import (
	"encoding/json"
	"testing"
)

// TestParseAsdot proves Parse reads the asdot and asdot+ forms RFC 5396
// Section 2 defines, and the asplain form beside them. The method is a table
// of tokens an operator can type against the number each one names.
func TestParseAsdot(t *testing.T) {
	cases := []struct {
		text string
		want uint32
	}{
		{"1.10", 65546},
		{"0.65526", 65526},
		{"65526", 65526},
		{"65546", 65546},
		{"0.0", 0},
		{"0.1", 1},
		{"1.0", 65536},
		{"65535.65535", 4294967295},
		{"65535.0", 4294901760},
		{"4294967295", 4294967295},
		{"065546", 65546},
	}
	for _, tc := range cases {
		got, err := Parse(tc.text)
		if err != nil {
			t.Fatalf("Parse(%q): unexpected error %v", tc.text, err)
		}
		if got != tc.want {
			t.Errorf("Parse(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

// TestParseRejects proves Parse refuses a malformed or out-of-range token
// rather than answering with a number nobody typed. The method walks the
// boundary of each 16-bit field and of the 32-bit whole.
func TestParseRejects(t *testing.T) {
	cases := []string{
		"",
		".",
		"1.",
		".10",
		"1.99999",
		"65536.0",
		"0.65536",
		"1.2.3",
		"4294967296",
		"-1",
		"+1",
		"1 .10",
		"1.10 ",
		"one.ten",
		"0x10",
		"1,10",
	}
	for _, text := range cases {
		if got, err := Parse(text); err == nil {
			t.Errorf("Parse(%q) = %d, want an error", text, got)
		}
	}
}

// TestAppendNotation proves each notation renders the number the way RFC 5396
// Section 2 describes, including the 65535/65536 boundary that separates asdot
// from asdot+. The method renders one number per notation and compares bytes.
func TestAppendNotation(t *testing.T) {
	cases := []struct {
		number   uint32
		notation Notation
		want     string
	}{
		{100, NotationPlain, "100"},
		{65546, NotationPlain, "65546"},
		{4294967295, NotationPlain, "4294967295"},
		{100, NotationDot, "100"},
		{65535, NotationDot, "65535"},
		{65536, NotationDot, "1.0"},
		{65546, NotationDot, "1.10"},
		{65526, NotationDot, "65526"},
		{4294967295, NotationDot, "65535.65535"},
		{0, NotationDotPlus, "0.0"},
		{100, NotationDotPlus, "0.100"},
		{65535, NotationDotPlus, "0.65535"},
		{65546, NotationDotPlus, "1.10"},
		{65526, NotationDotPlus, "0.65526"},
	}
	for _, tc := range cases {
		got := string(Append(nil, tc.number, tc.notation))
		if got != tc.want {
			t.Errorf("Append(%d, %s) = %q, want %q", tc.number, tc.notation, got, tc.want)
		}
		if text := Text(tc.number, tc.notation); text != tc.want {
			t.Errorf("Text(%d, %s) = %q, want %q", tc.number, tc.notation, text, tc.want)
		}
	}
}

// TestAppendRoundTrips proves every rendering Append produces is a token Parse
// reads back to the same number. An operator can therefore paste show output
// into a config. The method walks the two field boundaries in all three
// notations.
func TestAppendRoundTrips(t *testing.T) {
	numbers := []uint32{0, 1, 100, 65535, 65536, 65546, 131071, 4294901760, 4294967295}
	notations := []Notation{NotationPlain, NotationDot, NotationDotPlus}
	for _, notation := range notations {
		for _, number := range numbers {
			text := Text(number, notation)
			got, err := Parse(text)
			if err != nil {
				t.Fatalf("Parse(%q) from the %s rendering of %d: %v", text, notation, number, err)
			}
			if got != number {
				t.Errorf("round trip of %d through %s gave %q then %d", number, notation, text, got)
			}
		}
	}
}

// TestParseNotation proves the config tokens map to the notations they name
// and that anything else is refused. The method covers each token and five
// near misses.
func TestParseNotation(t *testing.T) {
	cases := []struct {
		token string
		want  Notation
	}{
		{TokenPlain, NotationPlain},
		{TokenDot, NotationDot},
		{TokenDotPlus, NotationDotPlus},
	}
	for _, tc := range cases {
		got, err := parseNotation(tc.token)
		if err != nil {
			t.Fatalf("parseNotation(%q): unexpected error %v", tc.token, err)
		}
		if got != tc.want {
			t.Errorf("parseNotation(%q) = %v, want %v", tc.token, got, tc.want)
		}
		if got.String() != tc.token {
			t.Errorf("%v.String() = %q, want %q", got, got.String(), tc.token)
		}
	}
	for _, token := range []string{"", "asdot++", "dot", "ASDOT", "plain"} {
		if _, err := parseNotation(token); err == nil {
			t.Errorf("parseNotation(%q) accepted an unknown token", token)
		}
	}
}

// TestAppendAllocatesNothing proves Append writes into a caller-owned buffer,
// which is what lets the text renderers stay allocation free. The method runs
// Append over a stack buffer and counts allocations.
func TestAppendAllocatesNothing(t *testing.T) {
	var scratch [16]byte
	allocations := testing.AllocsPerRun(100, func() {
		buf := Append(scratch[:0], 65546, NotationDot)
		if len(buf) != 4 {
			t.Fatalf("Append wrote %d bytes, want 4", len(buf))
		}
	})
	if allocations != 0 {
		t.Errorf("Append allocated %.1f times per run, want 0", allocations)
	}
}

// TestNumberJSONRoundTrip proves an AS number written into a payload comes
// back as the same number in every notation. It also proves asplain still
// writes a JSON number. The method marshals one number per notation, then
// reads it back.
//
// VALIDATES: Number.MarshalJSON, Number.UnmarshalJSON, AppendJSON, JSONValue.
// PREVENTS: a consumer that decodes a payload written under asdot answering
// nothing, which is what a plain uint32 field does with "1.10".
func TestNumberJSONRoundTrip(t *testing.T) {
	restore := Configured()
	t.Cleanup(func() { configure(t, restore) })

	cases := []struct {
		notation Notation
		number   uint32
		want     string
	}{
		{NotationPlain, 65546, `65546`},
		{NotationPlain, 100, `100`},
		{NotationDot, 65546, `"1.10"`},
		{NotationDot, 100, `"100"`},
		{NotationDotPlus, 100, `"0.100"`},
	}
	for _, tc := range cases {
		configure(t, tc.notation)

		encoded, err := json.Marshal(Of(tc.number))
		if err != nil {
			t.Fatalf("marshal %d in %s: %v", tc.number, tc.notation, err)
		}
		if string(encoded) != tc.want {
			t.Errorf("marshal %d in %s = %s, want %s", tc.number, tc.notation, encoded, tc.want)
		}
		if got := JSONValue(tc.number); got != tc.want {
			t.Errorf("JSONValue(%d) in %s = %s, want %s", tc.number, tc.notation, got, tc.want)
		}

		var back Number
		if err := json.Unmarshal(encoded, &back); err != nil {
			t.Fatalf("unmarshal %s: %v", encoded, err)
		}
		if back.Value() != tc.number {
			t.Errorf("unmarshal %s = %d, want %d", encoded, back.Value(), tc.number)
		}
		// The decoded Number answers the spelling the producer wrote. A client
		// renders it and derives no notation of its own.
		want := tc.want
		if len(want) >= 2 && want[0] == '"' {
			want = want[1 : len(want)-1]
		}
		if got := back.String(); got != want {
			t.Errorf("decoded %s renders %q, want %q", encoded, got, want)
		}
	}
}

// TestNumberUnmarshalsEveryNotation proves a consumer reads a payload whatever
// notation wrote it. A client and a producer that disagree about the
// configuration still understand each other. The method decodes each
// spelling.
//
// VALIDATES: Number.UnmarshalJSON over a JSON number and all three notations.
// PREVENTS: a decode that depends on the reader's own configuration.
func TestNumberUnmarshalsEveryNotation(t *testing.T) {
	for _, encoded := range []string{`65546`, `"65546"`, `"1.10"`} {
		var number Number
		if err := json.Unmarshal([]byte(encoded), &number); err != nil {
			t.Fatalf("unmarshal %s: %v", encoded, err)
		}
		if number.Value() != 65546 {
			t.Errorf("unmarshal %s = %d, want 65546", encoded, number.Value())
		}
	}
	var number Number
	if err := json.Unmarshal([]byte(`"1.99999"`), &number); err == nil {
		t.Error("unmarshal of an out-of-range asdot value was accepted")
	}
}

// TestFromJSON proves every shape an AS number arrives in is read, and that a
// value naming no AS number reports absence rather than AS 0. The method walks
// the four decoded shapes and four refusals.
//
// VALIDATES: FromJSON over string, float64, int, int64 and uint32.
// PREVENTS: a walk that skips a string, which under asdot drops every 4-byte
// AS number from the value it was building.
func TestFromJSON(t *testing.T) {
	accepted := []struct {
		value any
		want  uint32
	}{
		{"1.10", 65546},
		{"65546", 65546},
		{float64(65546), 65546},
		{int(65546), 65546},
		{int64(65546), 65546},
		{uint32(65546), 65546},
	}
	for _, tc := range accepted {
		got, ok := FromJSON(tc.value)
		if !ok || got != tc.want {
			t.Errorf("FromJSON(%#v) = %d, %v; want %d, true", tc.value, got, ok, tc.want)
		}
	}
	for _, value := range []any{nil, "peer", float64(-1), float64(4294967296), true} {
		if got, ok := FromJSON(value); ok {
			t.Errorf("FromJSON(%#v) = %d, true; want a refusal", value, got)
		}
	}
}

// TestConfigureFromBGP proves the as-notation leaf of a config subtree selects
// the notation. An absent leaf selects asplain, and a leaf naming nothing is
// refused. The method runs one subtree per case.
//
// VALIDATES: ConfigureFromBGP and Configured.
// PREVENTS: a typo being accepted and rendering asplain, which tells the
// operator their edit took effect when it did not.
func TestConfigureFromBGP(t *testing.T) {
	restore := Configured()
	t.Cleanup(func() { configure(t, restore) })

	accepted := []struct {
		config map[string]any
		want   Notation
	}{
		{map[string]any{}, NotationPlain},
		{map[string]any{LeafName: TokenPlain}, NotationPlain},
		{map[string]any{LeafName: TokenDot}, NotationDot},
		{map[string]any{LeafName: TokenDotPlus}, NotationDotPlus},
	}
	for _, tc := range accepted {
		configure(t, NotationDotPlus) // so an absent leaf has something to overwrite
		if err := ConfigureFromBGP(tc.config); err != nil {
			t.Fatalf("ConfigureFromBGP(%v): unexpected error %v", tc.config, err)
		}
		if got := Configured(); got != tc.want {
			t.Errorf("ConfigureFromBGP(%v) recorded %v, want %v", tc.config, got, tc.want)
		}
	}
	for _, config := range []map[string]any{
		{LeafName: "dot"},
		{LeafName: ""},
		{LeafName: 3},
	} {
		if err := ConfigureFromBGP(config); err == nil {
			t.Errorf("ConfigureFromBGP(%v) accepted a value that names no notation", config)
		}
	}
}

// configure records one notation for the rest of a test.
func configure(t *testing.T, notation Notation) {
	t.Helper()
	if err := Configure(notation.String()); err != nil {
		t.Fatalf("Configure(%s): %v", notation, err)
	}
}

// TestNumberTolerantOfNull proves a null AS field leaves the Number at its
// zero value instead of failing the decode. The method decodes null on its own
// and inside a struct beside a field that must survive it.
//
// VALIDATES: Number.UnmarshalJSON accepts JSON null.
// PREVENTS: one null field failing an entire dashboard or completion decode,
// which is the failure this type was introduced to remove. The plain uint32
// field it replaced tolerated null.
func TestNumberTolerantOfNull(t *testing.T) {
	var number Number
	if err := json.Unmarshal([]byte(`null`), &number); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if number.Value() != 0 {
		t.Errorf("null decoded to %d, want 0", number.Value())
	}

	var row struct {
		RemoteAS Number `json:"remote-as"`
		State    string `json:"state"`
	}
	if err := json.Unmarshal([]byte(`{"remote-as":null,"state":"established"}`), &row); err != nil {
		t.Fatalf("unmarshal a row with a null AS number: %v", err)
	}
	if row.State != "established" {
		t.Errorf("state = %q, want the rest of the row to survive a null AS number", row.State)
	}
}
