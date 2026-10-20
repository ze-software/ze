package attribute

import (
	"encoding/hex"
	"strings"
	"testing"
)

// TestInterfaceSetEncode pins the eight octets each interface-set text form
// produces, on every parser that consults the keyword table, and the name
// AppendDecoded renders them back to.
//
// VALIDATES: both ExaBGP forms, both transitivity types, the three directions,
// and both bounds of the 14-bit group identifier, against the octets ExaBGP
// sends for the same text (qa/api/api-flow-merge.ci).
// PREVENTS: an interface set reaching the wire with the wrong type octet, or a
// group identifier that overflowed into a direction flag.
func TestInterfaceSetEncode(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		octet string
		named string
	}{
		{"transitive input", "interface-set:transitive:input:1234:10", "0702 000004d2 400a", "interface-set:transitive:input:1234:10"},
		{"transitive output", "interface-set:transitive:output:254:254", "0702 000000fe 80fe", "interface-set:transitive:output:254:254"},
		{"transitive both", "interface-set:transitive:input-output:1234:10", "0702 000004d2 c00a", "interface-set:transitive:input-output:1234:10"},
		{"non transitive", "interface-set:non-transitive:input:3405770241:1", "4702 caffee01 4001", "interface-set:non-transitive:input:3405770241:1"},
		{"no transitivity given", "interface-set:input:1234:10", "0702 000004d2 400a", "interface-set:transitive:input:1234:10"},
		{"group id zero", "interface-set:transitive:output:0:0", "0702 00000000 8000", "interface-set:transitive:output:0:0"},
		{"group id maximum", "interface-set:transitive:input:1:16383", "0702 00000001 7fff", "interface-set:transitive:input:1:16383"},
		{"keyword case", "Interface-Set:transitive:input:1234:10", "0702 000004d2 400a", "interface-set:transitive:input:1234:10"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := strings.ReplaceAll(tc.octet, " ", "")

			ec, matched, err := ParseExtendedCommunityKeyword(tc.text)
			if !matched {
				t.Fatalf("ParseExtendedCommunityKeyword(%q) did not match", tc.text)
			}
			if err != nil {
				t.Fatalf("ParseExtendedCommunityKeyword(%q): %v", tc.text, err)
			}
			if got := hex.EncodeToString(ec[:]); got != want {
				t.Errorf("ParseExtendedCommunityKeyword(%q) = %s, want %s", tc.text, got, want)
			}

			single, err := ParseSingleExtCommunity(tc.text)
			if err != nil {
				t.Fatalf("ParseSingleExtCommunity(%q): %v", tc.text, err)
			}
			if got := hex.EncodeToString(single[:]); got != want {
				t.Errorf("ParseSingleExtCommunity(%q) = %s, want %s", tc.text, got, want)
			}

			if got := string(ec.AppendDecoded(nil)); got != tc.named {
				t.Errorf("AppendDecoded = %q, want %q", got, tc.named)
			}
		})
	}
}

// TestInterfaceSetRefusals proves each malformed value is reported rather than
// encoded to something else.
//
// VALIDATES: transitivity, direction, AS number and group identifier outside
// what the draft and ExaBGP accept are refused, including the direction with
// both flags clear, which draft-ietf-idr-flowspec-interfaceset Section 5 calls
// an error.
// PREVENTS: a truncated group identifier, a zero AS number, or a community
// with no direction reaching the wire in place of the value the operator wrote.
func TestInterfaceSetRefusals(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"unknown transitivity", "interface-set:maybe:input:1234:10"},
		{"unknown direction", "interface-set:transitive:sideways:1234:10"},
		{"no direction flag", "interface-set:transitive:none:1234:10"},
		{"no direction", "interface-set:1234:10"},
		{"dotted as number", "interface-set:transitive:input:1.234:10"},
		{"as number too large", "interface-set:transitive:input:4294967296:10"},
		{"group id above 14 bits", "interface-set:transitive:input:1234:16384"},
		{"group id not a number", "interface-set:transitive:input:1234:ten"},
		{"empty value", "interface-set:"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ec, matched, err := ParseExtendedCommunityKeyword(tc.text)
			if !matched {
				t.Fatalf("ParseExtendedCommunityKeyword(%q) did not match the keyword", tc.text)
			}
			if err == nil {
				t.Fatalf("ParseExtendedCommunityKeyword(%q) = %x, want an error", tc.text, ec[:])
			}
		})
	}
}

// TestExtendedCommunityKeywordNoMatch proves text naming no registered keyword
// is left to the caller's own forms rather than refused.
//
// VALIDATES: a known non-registered prefix and a colon-less word are unmatched.
// PREVENTS: the table swallowing `target:65000:1` and refusing it.
func TestExtendedCommunityKeywordNoMatch(t *testing.T) {
	for _, text := range []string{"target:65000:1", "rate-limit:0", "copy-to-nexthop", "interface-set"} {
		if _, matched, err := ParseExtendedCommunityKeyword(text); matched {
			t.Errorf("ParseExtendedCommunityKeyword(%q) matched (err %v), want unmatched", text, err)
		}
	}
}
