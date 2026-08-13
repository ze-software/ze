// VALIDATES: one reading of the COMMUNITIES attribute out of the policy filter
// text format, shared by the two plugins that decide on a community.
// PREVENTS: "community " matching inside "extended-community " or
// "large-community "; a bracketed list read as one value; a value bleeding into
// the next attribute.

package filtertext

import (
	"slices"
	"testing"
)

func TestCommunityValues(t *testing.T) {
	tests := []struct {
		name       string
		updateText string
		kind       CommunityKind
		want       []string
	}{
		{
			name:       "single standard value",
			updateText: "origin igp community 65001:100 next-hop 10.0.0.1",
			kind:       CommunityStandard,
			want:       []string{"65001:100"},
		},
		{
			name:       "bracketed standard list",
			updateText: "community [65001:100 no-export] med 10",
			kind:       CommunityStandard,
			want:       []string{"65001:100", "no-export"},
		},
		{
			name:       "well-known name as the formatter emits it",
			updateText: "community [blackhole no-export] nlri ipv4 unicast add 10.0.0.1/32",
			kind:       CommunityStandard,
			want:       []string{"blackhole", "no-export"},
		},
		{
			name:       "standard does not match inside large-community",
			updateText: "large-community 65000:1:2 med 10",
			kind:       CommunityStandard,
			want:       nil,
		},
		{
			name:       "standard does not match inside extended-community",
			updateText: "extended-community 000200010000000a med 10",
			kind:       CommunityStandard,
			want:       nil,
		},
		{
			name:       "large value",
			updateText: "community 65001:100 large-community 65000:1:2",
			kind:       CommunityLarge,
			want:       []string{"65000:1:2"},
		},
		{
			name:       "extended value",
			updateText: "extended-community 000200010000000a",
			kind:       CommunityExtended,
			want:       []string{"000200010000000a"},
		},
		{
			name:       "three values in one bracketed list",
			updateText: "origin igp community [65001:100 no-export 65002:200] nlri ipv4/unicast add 10.0.0.0/24",
			kind:       CommunityStandard,
			want:       []string{"65001:100", "no-export", "65002:200"},
		},
		{
			name:       "bracketed list at end of text",
			updateText: "origin igp community [65001:100 65001:200]",
			kind:       CommunityStandard,
			want:       []string{"65001:100", "65001:200"},
		},
		{
			name:       "standard read while extended is also present",
			updateText: "origin igp community 65001:100 extended-community 000200010000000a nlri ipv4/unicast add 10.0.0.0/24",
			kind:       CommunityStandard,
			want:       []string{"65001:100"},
		},
		{
			name:       "standard read while large is also present",
			updateText: "origin igp community 65001:100 large-community 65001:1:2 nlri ipv4/unicast add 10.0.0.0/24",
			kind:       CommunityStandard,
			want:       []string{"65001:100"},
		},
		{
			name:       "attribute absent",
			updateText: "origin igp med 10",
			kind:       CommunityStandard,
			want:       nil,
		},
		{
			name:       "empty text",
			updateText: "",
			kind:       CommunityStandard,
			want:       nil,
		},
		{
			name:       "value at end of text",
			updateText: "med 10 community 65001:100",
			kind:       CommunityStandard,
			want:       []string{"65001:100"},
		},
		{
			// The formatter never emits this. Reading it as one unsplit value
			// means no configured value matches it, which is the safe answer for
			// a text a filter cannot trust.
			name:       "unterminated bracket reads as one value",
			updateText: "community [65001:100 no-export",
			kind:       CommunityStandard,
			want:       []string{"[65001:100 no-export"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommunityValues(tt.updateText, tt.kind)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("CommunityValues(%q, %v) = %v, want %v", tt.updateText, tt.kind, got, tt.want)
			}
		})
	}
}

func TestHasCommunity(t *testing.T) {
	const text = "community [blackhole 65001:666] large-community 65000:1:2"

	if !HasCommunity(text, CommunityStandard, "blackhole") {
		t.Error("the well-known name is present and was not found")
	}
	if !HasCommunity(text, CommunityStandard, "65001:666") {
		t.Error("the operator value is present and was not found")
	}
	if HasCommunity(text, CommunityStandard, "65535:666") {
		t.Error("the numeric spelling of a well-known value is not what the formatter emits, so it must not match")
	}
	if HasCommunity(text, CommunityLarge, "blackhole") {
		t.Error("a standard value must not be found in the large attribute")
	}
	if !HasCommunity(text, CommunityLarge, "65000:1:2") {
		t.Error("the large value is present and was not found")
	}
}

func TestCommunityKindNames(t *testing.T) {
	for _, tt := range []struct {
		kind  CommunityKind
		name  string
		field string
	}{
		{CommunityStandard, "standard", "community"},
		{CommunityLarge, "large", "large-community"},
		{CommunityExtended, "extended", "extended-community"},
		{CommunityKind(9), "unknown", ""},
	} {
		if got := tt.kind.String(); got != tt.name {
			t.Errorf("String() = %q, want %q", got, tt.name)
		}
		if got := tt.kind.FieldName(); got != tt.field {
			t.Errorf("FieldName() = %q, want %q", got, tt.field)
		}
	}

	// An unrecognized kind reads no field at all. Without the guard it would cut
	// on the empty needle and return the first token of the update as a
	// community value, which every caller would then match against.
	if got := CommunityValues("origin igp community 65001:100", CommunityKind(9)); got != nil {
		t.Errorf("CommunityValues on an unknown kind = %v, want nil", got)
	}
}
