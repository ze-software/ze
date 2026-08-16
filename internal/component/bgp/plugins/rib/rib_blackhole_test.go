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

// TestCarriesBlackholeCommunity and its concat helper MOVED, they
// were not dropped. carriesBlackholeCommunity no longer exists in this package:
// the COMMUNITIES scan now lives in blackholecfg because origin validation and
// the origination gate read the same bytes, and every one of its 13 cases is
// carried over verbatim as TestCarries in
// internal/component/bgp/blackholecfg/blackholecfg_test.go. Keeping a copy here
// against a one-line delegate is the drift this extraction removes.
