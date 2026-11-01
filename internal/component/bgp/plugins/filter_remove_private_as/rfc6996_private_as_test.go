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

// VALIDATES: the as-path field scan finds a Private Use ASN in a sequence
// and in a nested AS_SET, and finds none in a path of public ASNs or in an
// object with no as-path.
// PREVENTS: a private ASN missed in a set, or a public one taken as private.
func TestASPathFieldHasPrivateASN(t *testing.T) {
	cases := []struct {
		fields string
		want   bool
	}{
		{`{"as-path":[64496,64512,64497]}`, true},
		{`{"as-path":[64496,[64497,4200000000]]}`, true},
		{`{"as-path":[64496,64497]}`, false},
		{`{}`, false},
		{``, false},
	}
	for _, c := range cases {
		if got := asPathFieldHasPrivateASN([]byte(c.fields)); got != c.want {
			t.Errorf("asPathFieldHasPrivateASN(%s) = %v, want %v", c.fields, got, c.want)
		}
	}
}

// RFC requirement: RFC6996-4-1 positive -- a Private Use ASN present in the AS path
// is removed before advertisement: 64512 in [64496 64512 64497] draws the
// remove-private intent.
func TestRFC6996StripsPrivateUseASN(t *testing.T) {
	if !asPathFieldHasPrivateASN([]byte(`{"as-path":[64496,64512,64497]}`)) {
		t.Fatal("[64496 64512 64497] carries no Private Use ASN, want 64512 found")
	}
}

// RFC requirement: RFC6996-4-1 negative -- removal is confined to the Private Use
// range: an AS path of only non-private ASNs is advertised unchanged, so a
// non-private ASN is never stripped as if it were private.
func TestRFC6996KeepsPublicASN(t *testing.T) {
	if asPathFieldHasPrivateASN([]byte(`{"as-path":[64496,64497]}`)) {
		t.Fatal("[64496 64497] carries a Private Use ASN, want none")
	}
}
