package graph

// VALIDATES: the reference and header parser over each corpus trap (AC-2).
// PREVENTS: a missed edge shrinking a profile closure.

import (
	"slices"
	"strings"
	"testing"
)

// TestNormativeReferenceParse holds the reference parser to every trap the
// corpus research found (AC-2): a table of contents that repeats the headings
// indented, `RFC` wrapped onto the next line, `RFC-NNNN`, a combined
// References section, page furniture inside the section, a draft version
// suffix, and the informative section that must not be read.
// Method: one fixture per trap, each asserting the exact citation list.
func TestNormativeReferenceParse(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
		refs bool
	}{
		{
			name: "toc duplicates are not headings, informative is not read",
			text: "Table of Contents\n\n   9.  Normative References ........ 20\n   10. Informative References ...... 21\n\n" +
				"1.  Introduction\n\n   See [RFC1997].\n\n" +
				"9.  Normative References\n\n   [RFC4271]  Rekhter, Y., RFC 4271.\n\n" +
				"10.  Informative References\n\n   [RFC1771]  old, RFC 1771.\n",
			want: []string{"rfc4271"},
			refs: true,
		},
		{
			name: "RFC wrapped to the next line and RFC-NNNN",
			text: "Normative References\n\n   [1] Postel, J., RFC\n       791, 1981.\n   [2] Mills, RFC-1305.\n\nAuthors' Addresses\n\n   RFC 9999\n",
			want: []string{"rfc791", "rfc1305"},
			refs: true,
		},
		{
			name: "combined References runs through its subsections",
			text: "1.  Intro\n\n9.  References\n\n   [1] RFC 2453\n\n9.1  Additional References\n\n   [2] RFC 1058\n\nAuthor's Address\n\n   RFC 7777\n",
			want: []string{"rfc2453", "rfc1058"},
			refs: true,
		},
		{
			name: "page furniture does not end the section",
			text: "Normative References\n\n   [RFC4271] RFC 4271.\n\nRekhter, et al.         Standards Track                    [Page 98]\n\f\n" +
				"RFC 4271                         BGP-4                      January 2006\n\n   [RFC4760] RFC 4760.\n\nInformative References\n",
			want: []string{"rfc4271", "rfc4760"},
			refs: true,
		},
		{
			name: "draft citation loses its version, a six-digit number is no RFC",
			text: "Normative References\n\n   draft-ietf-idr-bgp-multisession-07\n   RFC 123456\n   RFC 0\n",
			want: []string{"draft-ietf-idr-bgp-multisession"},
			refs: true,
		},
		{
			name: "no references section at all",
			text: "1.  Intro\n\n   This text cites RFC 4271 in prose only.\n",
			want: nil,
			refs: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.text)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !slices.Equal(got.Cites, tc.want) {
				t.Errorf("Cites = %v, want %v", got.Cites, tc.want)
			}
			if got.HasReferences != tc.refs {
				t.Errorf("HasReferences = %v, want %v", got.HasReferences, tc.refs)
			}
		})
	}
	if _, err := Parse("  \n"); err == nil {
		t.Error("an empty text parsed; it must be refused")
	}
}

// TestHeaderLinksParse holds the header reader to the lineage forms the corpus
// uses (AC-2): the author column on the same line, a continuation line, `RFC`
// and `RFCs` prefixes, an IEN list that is not RFCs, an `(if approved)` note,
// and the prose sentence old RFCs carry instead of a header line.
// Method: one fixture per form, asserting both lists exactly.
func TestHeaderLinksParse(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		updates   []string
		obsoletes []string
	}{
		{
			name:      "author column and continuation line",
			text:      "Network Working Group                                         Y. Rekhter\nUpdates: 1997, 4271, 4360,                                    A. Author\n         4456, 6368                                           Org\nObsoletes: 1771                                            S. Hares, Ed.\n\n",
			updates:   []string{"rfc1997", "rfc4271", "rfc4360", "rfc4456", "rfc6368"},
			obsoletes: []string{"rfc1771"},
		},
		{
			name:      "RFC and RFCs prefixes, IENs dropped, note cut",
			text:      "Obsoletes: RFCs 760, 777, IENs 128, 129\nUpdates: 4271 (if approved)\n\n",
			updates:   []string{"rfc4271"},
			obsoletes: []string{"rfc760", "rfc777"},
		},
		{
			name:      "prose lineage with RFC wrapped and an anchor",
			text:      "Network Working Group\n\nAbstract\n\n   This memo obsoletes RFC\n   2463 [RFC-2463] and RFC 2464. This document updates RFC 2780.\n\n1.  Introduction\n\n   This document updates RFC 9999 in the body, which is not front matter.\n",
			updates:   []string{"rfc2780"},
			obsoletes: []string{"rfc2463", "rfc2464"},
		},
		{
			name: "no lineage",
			text: "Network Working Group\n\nAbstract\n\n   This document updates the definition of options.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.text)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !slices.Equal(got.Updates, tc.updates) {
				t.Errorf("Updates = %v, want %v", got.Updates, tc.updates)
			}
			if !slices.Equal(got.Obsoletes, tc.obsoletes) {
				t.Errorf("Obsoletes = %v, want %v", got.Obsoletes, tc.obsoletes)
			}
		})
	}
}

// TestRFCNumberBounds holds the five-digit bound of the boundary table: 1 and
// 99999 are RFC numbers, 0 and a six-digit run are not.
func TestRFCNumberBounds(t *testing.T) {
	for digits, want := range map[string]bool{"1": true, "99999": true, "0": false, "100000": false} {
		if _, ok := rfcStem(digits); ok != want {
			t.Errorf("rfcStem(%q) = %v, want %v", digits, ok, want)
		}
	}
	if !strings.HasPrefix(DraftStem("draft-a-b-16"), "draft-a-b") || DraftStem("draft-a-b-16") != "draft-a-b" {
		t.Errorf("DraftStem kept the version: %q", DraftStem("draft-a-b-16"))
	}
}
