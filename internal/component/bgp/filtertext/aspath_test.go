// VALIDATES: one reading of the AS_PATH attribute out of the policy filter text
// format, against the writer that produces that format.
// PREVENTS: brackets left on a multi-ASN path; a single ASN read together with
// the attribute that follows it; padding a plugin delta writes inside the
// brackets read back as part of the ASN list; a reader that drifts from
// (*attribute.ASPath).AppendText when the writer changes shape.

package filtertext

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func TestASPath(t *testing.T) {
	tests := []struct {
		name       string
		updateText string
		want       string
	}{
		{
			name:       "single ASN",
			updateText: "origin igp as-path 65001 next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "65001",
		},
		{
			name:       "bracketed list",
			updateText: "origin igp as-path [65001 65002 65003] next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "65001 65002 65003",
		},
		{
			name:       "no as-path keyword",
			updateText: "origin igp next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "",
		},
		{
			name:       "empty text",
			updateText: "",
			want:       "",
		},
		{
			name:       "single ASN at the end",
			updateText: "origin igp as-path 65001",
			want:       "65001",
		},
		{
			name:       "bracketed list at the end",
			updateText: "origin igp as-path [65001 65002]",
			want:       "65001 65002",
		},
		{
			name:       "as-path first",
			updateText: "as-path 65001 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "65001",
		},
		{
			name:       "bracketed list before med",
			updateText: "as-path [65001 65002] med 100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "65001 65002",
		},
		{
			name:       "padded brackets",
			updateText: "origin igp as-path [ 65001 65002 ] next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "65001 65002",
		},
		{
			name:       "padded brackets around one ASN",
			updateText: "origin igp as-path [ 65001 ] next-hop 1.1.1.1",
			want:       "65001",
		},
		{
			name:       "empty brackets",
			updateText: "origin igp as-path [] next-hop 1.1.1.1",
			want:       "",
		},
		{
			name:       "brackets holding only spaces",
			updateText: "origin igp as-path [   ] next-hop 1.1.1.1",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ASPath(tt.updateText)
			if got != tt.want {
				t.Errorf("ASPath(%q) = %q, want %q", tt.updateText, got, tt.want)
			}
		})
	}
}

// TestASPathFieldMatchesAppendText renders each path with the writer that
// produces the format and reads it back with ASPath, so a change to either side
// that the other does not follow fails here. The expectation is the flattened
// ASN list, computed from the segments rather than written by hand: AppendText
// keeps no segment marker, so every segment type reads back the same way.
func TestASPathFieldMatchesAppendText(t *testing.T) {
	tests := []struct {
		name     string
		segments []attribute.ASPathSegment
	}{
		{
			name:     "no segments",
			segments: nil,
		},
		{
			name:     "one empty segment",
			segments: []attribute.ASPathSegment{{Type: attribute.ASSequence}},
		},
		{
			name:     "one ASN",
			segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{65001}}},
		},
		{
			name:     "several ASNs",
			segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002, 4200000000}}},
		},
		{
			name:     "one ASN in a set",
			segments: []attribute.ASPathSegment{{Type: attribute.ASSet, ASNs: []uint32{65001}}},
		},
		{
			name: "sequence then set",
			segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
				{Type: attribute.ASSet, ASNs: []uint32{65010, 65011}},
			},
		},
		{
			name: "confederation sequence then sequence",
			segments: []attribute.ASPathSegment{
				{Type: attribute.ASConfedSequence, ASNs: []uint32{64512, 64513}},
				{Type: attribute.ASSequence, ASNs: []uint32{65001}},
			},
		},
		{
			name: "confederation set alone",
			segments: []attribute.ASPathSegment{
				{Type: attribute.ASConfedSet, ASNs: []uint32{64512, 64513}},
			},
		},
	}

	// The three places the keyword sits in a rendered update: alone, between two
	// other attributes, and last.
	positions := []struct {
		name   string
		before string
		after  string
	}{
		{name: "alone", before: "", after: ""},
		{name: "between attributes", before: "origin igp ", after: " next-hop 10.0.0.1"},
		{name: "last attribute", before: "origin igp med 100 ", after: ""},
	}

	for _, tt := range tests {
		path := &attribute.ASPath{Segments: tt.segments}
		rendered := string(path.AppendText(nil))
		want := flattenASNs(tt.segments)

		for _, position := range positions {
			t.Run(tt.name+" "+position.name, func(t *testing.T) {
				updateText := position.before + rendered + position.after
				got := ASPath(updateText)
				if got != want {
					t.Errorf("ASPath(%q) = %q, want %q", updateText, got, want)
				}
			})
		}
	}
}

// flattenASNs writes the ASNs of every segment as one space-separated list, the
// shape (*attribute.ASPath).AppendText emits inside the brackets.
func flattenASNs(segments []attribute.ASPathSegment) string {
	var out strings.Builder
	for _, segment := range segments {
		for _, asn := range segment.ASNs {
			if out.Len() > 0 {
				out.WriteByte(' ')
			}
			out.WriteString(strconv.FormatUint(uint64(asn), 10))
		}
	}
	return out.String()
}
