package attribute

import "testing"

// TestCommunityKindNames proves each kind's config spelling and field name, and
// that every function taking a kind refuses one outside the named set.
func TestCommunityKindNames(t *testing.T) {
	for _, tt := range []struct {
		kind  CommunityKind
		name  string
		field string
	}{
		{CommunityStandard, "standard", "community"},
		{CommunityLarge, "large", "large-community"},
		{CommunityExtended, "extended", "extended-community"},
	} {
		if got := tt.kind.String(); got != tt.name {
			t.Errorf("String() = %q, want %q", got, tt.name)
		}
		if got := tt.kind.FieldName(); got != tt.field {
			t.Errorf("FieldName() = %q, want %q", got, tt.field)
		}
	}

	// Config parsers select a named kind; an invented internal selector must
	// fail rather than look like an absent attribute or an unknown spelling.
	for _, check := range []struct {
		name string
		call func()
	}{
		{"String", func() { _ = CommunityKind(9).String() }},
		{"FieldName", func() { CommunityKind(9).FieldName() }},
		{"CommunityWire", func() { _, _, _ = CommunityWire("65001:100", CommunityKind(9)) }},
		{"CanonicalCommunity", func() { _, _ = CanonicalCommunity("65001:100", CommunityKind(9)) }},
	} {
		t.Run(check.name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != "BUG: invalid community kind" {
					t.Fatalf("panic = %v, want invalid community kind assertion", got)
				}
			}()
			check.call()
		})
	}
}
