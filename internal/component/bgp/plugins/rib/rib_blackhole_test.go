// VALIDATES: RFC 7999 Section 3.3, both Rx conditions, at the point where a
// best path becomes a FIB candidate. Condition 2 is the per-session agreement,
// and condition 1 is coverage by an equal or shorter authorized prefix.
// PREVENTS: an unconfigured peer discarding traffic, which RFC 7999 Section 4
// forbids by default. Also prevents an opted-in peer blackholing a prefix it
// was never authorized for, the denial-of-reachability vector of Section 6.

package rib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

func mustPrefixes(t *testing.T, ss ...string) []netip.Prefix {
	t.Helper()
	out := make([]netip.Prefix, 0, len(ss))
	for _, s := range ss {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			t.Fatalf("bad test prefix %q: %v", s, err)
		}
		out = append(out, p)
	}
	return out
}

// RFC 7999 Section 3.3 condition 1. Coverage is not prefix-list membership: a
// /32 inside an authorized /24 is covered, and the authorized entry carries no
// ge or le bound for it to fall outside of.
func TestCoveredByAuthorizedPrefix(t *testing.T) {
	authorized := mustPrefixes(t, "192.0.2.0/24", "2001:db8::/32")

	covered := []string{
		"192.0.2.1/32",   // the blackhole case: longer, inside
		"192.0.2.0/24",   // equal length, the authorized prefix itself
		"192.0.2.128/25", // longer, inside
		"2001:db8::1/128",
		"2001:db8:1::/48",
	}
	for _, s := range covered {
		if !coveredByAuthorized(authorized, mustPrefixes(t, s)[0]) {
			t.Errorf("%s: not covered, want covered", s)
		}
	}

	notCovered := []string{
		"198.51.100.1/32", // outside every authorized block
		"192.0.0.0/16",    // SHORTER than the authorized /24: covers it rather than being covered
		"192.0.3.1/32",    // adjacent block, one bit out
		"2001:db9::1/128", // outside the authorized v6 block
		"0.0.0.0/0",
	}
	for _, s := range notCovered {
		if coveredByAuthorized(authorized, mustPrefixes(t, s)[0]) {
			t.Errorf("%s: covered, want not covered", s)
		}
	}
}

// An empty authorization list is the closed state. RFC 7999 Section 6 names
// unauthorized BLACKHOLE as a denial-of-reachability vector, so an unstated
// authorization must authorize nothing rather than everything.
func TestCoveredByAuthorizedEmptyListAuthorizesNothing(t *testing.T) {
	if coveredByAuthorized(nil, mustPrefixes(t, "192.0.2.1/32")[0]) {
		t.Error("an empty authorization list covered a prefix")
	}
}

// A v4 blackhole must not be authorized by a v6 entry, or the reverse. Both
// netip.Prefix.Contains and the length test would otherwise be asked to compare
// across families.
func TestCoveredByAuthorizedIsFamilyScoped(t *testing.T) {
	if coveredByAuthorized(mustPrefixes(t, "2001:db8::/32"), mustPrefixes(t, "192.0.2.1/32")[0]) {
		t.Error("a v6 authorization covered a v4 prefix")
	}
	if coveredByAuthorized(mustPrefixes(t, "192.0.2.0/24"), mustPrefixes(t, "2001:db8::1/128")[0]) {
		t.Error("a v4 authorization covered a v6 prefix")
	}
}

// The decision as a whole, over the two RFC conditions plus the community
// itself. Every row states what a real operator config produces.
func TestBlackholeRouteTypeDecision(t *testing.T) {
	authorized := mustPrefixes(t, "192.0.2.0/24")
	agreed := []attribute.Community{attribute.CommunityBlackhole}
	pfx := mustPrefixes(t, "192.0.2.1/32")[0]

	cases := []struct {
		name         string
		cfg          blackholeConfig
		hasCommunity bool
		prefix       netip.Prefix
		want         routetype.Type
	}{
		{
			name:         "unconfigured peer never discards",
			cfg:          blackholeConfig{},
			hasCommunity: true,
			prefix:       pfx,
			want:         0,
		},
		{
			name:         "no community agreed, an authorization listed",
			cfg:          blackholeConfig{authorized: authorized},
			hasCommunity: true,
			prefix:       pfx,
			want:         0,
		},
		{
			name:         "a community agreed, no authorization listed",
			cfg:          blackholeConfig{communities: agreed},
			hasCommunity: true,
			prefix:       pfx,
			want:         0,
		},
		{
			name:         "both conditions met",
			cfg:          blackholeConfig{communities: agreed, authorized: authorized},
			hasCommunity: true,
			prefix:       pfx,
			want:         routetype.Blackhole,
		},
		{
			name:         "both conditions met but the route carries no agreed community",
			cfg:          blackholeConfig{communities: agreed, authorized: authorized},
			hasCommunity: false,
			prefix:       pfx,
			want:         0,
		},
		{
			name:         "community agreed and present, prefix outside the authorization",
			cfg:          blackholeConfig{communities: agreed, authorized: authorized},
			hasCommunity: true,
			prefix:       mustPrefixes(t, "198.51.100.1/32")[0],
			want:         0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// scanned records whether the expensive community test was reached.
			// It must not be, for a route the two config tests already refuse.
			scanned := false
			got := blackholeRouteType(c.cfg, c.prefix, func() bool {
				scanned = true
				return c.hasCommunity
			})
			if got != c.want {
				t.Errorf("blackholeRouteType = %v, want %v", got, c.want)
			}
			wantScan := len(c.cfg.communities) > 0 && coveredByAuthorized(c.cfg.authorized, c.prefix)
			if scanned != wantScan {
				t.Errorf("community scan reached = %v, want %v: the cheap config tests must gate the wire scan", scanned, wantScan)
			}
		})
	}
}

// The wire scan that answers "does this route carry a community this session
// agreed to honor". The COMMUNITIES attribute is a set of 4-octet values, so the
// scan must find a value at any position and must not match a value that merely
// shares two octets with one.
func TestCarriesBlackholeCommunity(t *testing.T) {
	blackhole := []byte{0xFF, 0xFF, 0x02, 0x9A}
	noExport := []byte{0xFF, 0xFF, 0xFF, 0x01}
	other := []byte{0x00, 0x64, 0x00, 0x01}

	wellKnown := []attribute.Community{attribute.CommunityBlackhole}
	// 65001:666, an operator's own RTBH community. Operators use one of these
	// far more often than the well-known value, which is why the scan takes the
	// set from configuration instead of holding one constant.
	ownValue := []byte{0xFD, 0xE9, 0x02, 0x9A}
	own := []attribute.Community{attribute.Community(65001<<16 | 666)}

	cases := []struct {
		name string
		data []byte
		want []attribute.Community
		ok   bool
	}{
		{"only value", blackhole, wellKnown, true},
		{"first of three", concat(blackhole, noExport, other), wellKnown, true},
		{"last of three", concat(other, noExport, blackhole), wellKnown, true},
		{"middle of three", concat(other, blackhole, noExport), wellKnown, true},
		{"absent", concat(other, noExport), wellKnown, false},
		{"empty", nil, wellKnown, false},
		{"straddles a boundary, must not match", concat([]byte{0x00, 0xFF, 0xFF, 0x02}, []byte{0x9A, 0x00, 0x00, 0x00}), wellKnown, false},
		{"truncated attribute", []byte{0xFF, 0xFF, 0x02}, wellKnown, false},
		{"no community agreed matches nothing", blackhole, nil, false},

		// The case the hardcoded value could not express at all.
		{"an operator's own community", concat(other, ownValue), own, true},
		{"the well-known value does not match an operator's own agreement", blackhole, own, false},
		{"an operator's own value does not match the well-known agreement", ownValue, wellKnown, false},
		{"either of two agreed values matches", concat(other, ownValue), append(append([]attribute.Community{}, wellKnown...), own...), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := carriesBlackholeCommunity(c.data, c.want); got != c.ok {
				t.Errorf("carriesBlackholeCommunity = %v, want %v", got, c.ok)
			}
		})
	}
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
