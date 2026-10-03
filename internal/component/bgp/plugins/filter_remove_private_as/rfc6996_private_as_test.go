package filter_remove_private_as

import "testing"

// VALIDATES: RFC 6996 Private Use ASN ranges are exact and inclusive.
// PREVENTS: stripping public boundary ASNs adjacent to private ranges.
func TestIsPrivateASN(t *testing.T) {
	tests := []struct {
		asn  uint32
		want bool
	}{
		{64511, false},
		{64512, true},
		{65534, true},
		{65535, false},
		{4199999999, false},
		{4200000000, true},
		{4294967294, true},
		{4294967295, false},
	}
	for _, tt := range tests {
		if got := isPrivateASN(tt.asn); got != tt.want {
			t.Fatalf("isPrivateASN(%d) = %v, want %v", tt.asn, got, tt.want)
		}
	}
}

// VALIDATES: flat filter text is rewritten for downstream policy filters.
// PREVENTS: later filters in a chain seeing stale private ASNs.
func TestRewriteASPathText(t *testing.T) {
	got, changed := rewriteASPathText("64496 64512 64497", removeModeStrip, 65001)
	if !changed || got != "[64496 64497]" {
		t.Fatalf("strip rewrite = (%q,%v), want ([64496 64497], true)", got, changed)
	}

	got, changed = rewriteASPathText("64496 64512 64497", removeModePeerAS, 65001)
	if !changed || got != "[64496 65001 64497]" {
		t.Fatalf("peer-as rewrite = (%q,%v), want ([64496 65001 64497], true)", got, changed)
	}

	got, changed = rewriteASPathText("64496 64497", removeModeStrip, 65001)
	if changed || got != "64496 64497" {
		t.Fatalf("public rewrite = (%q,%v), want unchanged false", got, changed)
	}
}

// RFC requirement: RFC6996-4-1 positive -- a Private Use ASN present in the AS path
// is removed before advertisement: strip drops 64512 from [64496 64512 64497].
func TestRFC6996StripsPrivateUseASN(t *testing.T) {
	got, changed := rewriteASPathText("64496 64512 64497", removeModeStrip, 65001)
	if !changed || got != "[64496 64497]" {
		t.Fatalf("strip [64496 64512 64497] = (%q,%v), want ([64496 64497], true)", got, changed)
	}
}

// RFC requirement: RFC6996-4-1 negative -- removal is confined to the Private Use
// range: an AS path of only non-private ASNs is advertised unchanged, so a
// non-private ASN is never stripped as if it were private.
func TestRFC6996KeepsPublicASN(t *testing.T) {
	got, changed := rewriteASPathText("64496 64497", removeModeStrip, 65001)
	if changed || got != "64496 64497" {
		t.Fatalf("strip 64496 64497 = (%q,%v), want unchanged (false)", got, changed)
	}
}
