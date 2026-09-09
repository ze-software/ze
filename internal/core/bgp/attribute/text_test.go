package attribute

import (
	"slices"
	"testing"
)

// TestParseASPathTextAsdot proves an AS path typed in a dotted notation is
// read into the same numbers the decimal spelling gives, so a path pasted from
// a router that displays asdot is accepted. The method parses each form and
// compares the ASN slice.
//
// VALIDATES: ParseASPathText reads asplain, asdot and asdot+ tokens.
// PREVENTS: "1.10" being refused as an invalid ASN in an as-path.
func TestParseASPathTextAsdot(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []uint32
	}{
		{"single-asplain", []string{"65546"}, []uint32{65546}},
		{"single-asdot", []string{"1.10"}, []uint32{65546}},
		{"single-asdot-plus", []string{"0.100"}, []uint32{100}},
		{"list-mixed", []string{"[", "1.10", "65001", "0.65526", "]"}, []uint32{65546, 65001, 65526}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := ParseASPathText(tt.args)
			if err != nil {
				t.Fatalf("ParseASPathText(%v): unexpected error %v", tt.args, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("ParseASPathText(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// TestParseASPathTextRejects proves a malformed token is still refused after
// the dotted spellings were accepted. The method feeds each bad token and
// requires an error.
//
// VALIDATES: an out-of-range field and a non-numeric token are errors.
// PREVENTS: asdot acceptance widening into "accept anything with a period".
func TestParseASPathTextRejects(t *testing.T) {
	for _, args := range [][]string{
		{"1.99999"},
		{"65536.0"},
		{"1.2.3"},
		{"4294967296"},
		{"peer"},
	} {
		if got, _, err := ParseASPathText(args); err == nil {
			t.Errorf("ParseASPathText(%v) = %v, want an error", args, got)
		}
	}
}

// TestParseASPathTextForms verifies every argument shape the `as-path`
// command word accepts. The method feeds each shape and compares the ASN
// slice and the number of arguments consumed.
//
// VALIDATES: ParseASPathText reads the bracketed, comma and single forms.
// PREVENTS: Malformed AS_PATH from text commands.
func TestParseASPathTextForms(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		want         []uint32
		wantConsumed int
		wantErr      bool
	}{
		{name: "bracketed_spaces", args: []string{"[65001", "65002]"}, want: []uint32{65001, 65002}, wantConsumed: 2},
		{name: "bracketed_commas", args: []string{"[65001,65002]"}, want: []uint32{65001, 65002}, wantConsumed: 1},
		{name: "single", args: []string{"65001"}, want: []uint32{65001}, wantConsumed: 1},
		{name: "comma_no_brackets", args: []string{"65001,65002,65003"}, want: []uint32{65001, 65002, 65003}, wantConsumed: 1},
		// An unbracketed list stops after one argument on purpose: the
		// arguments after it belong to the next command word, so the caller
		// is told only one was consumed.
		{name: "unbracketed_stops_at_one", args: []string{"65001", "65002"}, want: []uint32{65001}, wantConsumed: 1},
		{name: "empty_brackets", args: []string{"[]"}, want: []uint32{}, wantConsumed: 1},
		{name: "invalid_asn", args: []string{"[abc]"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, consumed, err := ParseASPathText(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseASPathText(%v) = %v, want an error", tt.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseASPathText(%v): unexpected error %v", tt.args, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ParseASPathText(%v) = %v, want %v", tt.args, got, tt.want)
			}
			if consumed != tt.wantConsumed {
				t.Errorf("ParseASPathText(%v) consumed %d arguments, want %d", tt.args, consumed, tt.wantConsumed)
			}
		})
	}
}
