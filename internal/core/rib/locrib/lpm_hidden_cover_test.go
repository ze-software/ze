// Design: docs/architecture/rib/unified-locrib.md -- selectable covering prefixes.
// Related: manager.go -- LPM must skip hidden groups without losing their covers.

package locrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// TestLocRIB_LPMHiddenCover queries through policy-hidden prefixes and checks
// that the longest selectable cover wins while rejected candidates stay stored.
func TestLocRIB_LPMHiddenCover(t *testing.T) {
	previous := ribpolicy.Current()
	t.Cleanup(func() { ribpolicy.Set(previous) })
	snapshot, err := ribpolicy.NewSnapshot(nil, chainFunc(func(route ribpolicy.Route) ribpolicy.Verdict {
		if route.HasTag {
			return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
		}
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}))
	if err != nil {
		t.Fatal(err)
	}
	ribpolicy.Set(snapshot)

	cases := []struct {
		name     string
		family   family.Family
		visible  []string
		hidden   []string
		query    string
		selected string
	}{
		{
			name: "ipv4-nested-hidden", family: family.IPv4Unicast,
			visible: []string{"10.0.0.0/8", "10.1.0.0/16"},
			hidden:  []string{"10.1.2.0/24", "10.1.2.0/28"},
			query:   "10.1.2.5", selected: "10.1.0.0/16",
		},
		{
			name: "ipv6-nested-hidden", family: family.IPv6Unicast,
			visible: []string{"2001:db8::/32", "2001:db8:1::/48"},
			hidden:  []string{"2001:db8:1:2::/64", "2001:db8:1:2::/96"},
			query:   "2001:db8:1:2::5", selected: "2001:db8:1::/48",
		},
		{
			name: "ipv4-visible-control", family: family.IPv4Unicast,
			visible: []string{"10.0.0.0/8", "10.1.2.0/28"},
			hidden:  []string{"10.1.2.0/24"},
			query:   "10.1.2.5", selected: "10.1.2.0/28",
		},
		{
			name: "ipv6-visible-control", family: family.IPv6Unicast,
			visible: []string{"2001:db8::/32", "2001:db8:1:2::/96"},
			hidden:  []string{"2001:db8:1:2::/64"},
			query:   "2001:db8:1:2::5", selected: "2001:db8:1:2::/96",
		},
		{
			name: "ipv4-default-under-hidden-host", family: family.IPv4Unicast,
			visible: []string{"0.0.0.0/0"}, hidden: []string{"192.0.2.5/32"},
			query: "192.0.2.5", selected: "0.0.0.0/0",
		},
		{
			name: "ipv6-default-under-hidden-host", family: family.IPv6Unicast,
			visible: []string{"::/0"}, hidden: []string{"2001:db8::5/128"},
			query: "2001:db8::5", selected: "::/0",
		},
		{
			name: "ipv4-hidden-only", family: family.IPv4Unicast,
			hidden: []string{"0.0.0.0/0", "192.0.2.5/32"}, query: "192.0.2.5",
		},
		{
			name: "ipv6-hidden-only", family: family.IPv6Unicast,
			hidden: []string{"::/0", "2001:db8::5/128"}, query: "2001:db8::5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rib := NewRIB()
			// Set before the first family is created; this changes no global
			// setting and deterministically puts every cover in the same shard.
			rib.nShards = 1
			for _, prefix := range tc.visible {
				rib.Insert(tc.family, netip.MustParsePrefix(prefix), pathStatic())
			}
			for _, prefix := range tc.hidden {
				path := pathStatic()
				path.Tag, path.HasTag = 4242, true
				rib.Insert(tc.family, netip.MustParsePrefix(prefix), path)
				inspected := false
				rib.Inspect(tc.family, netip.MustParsePrefix(prefix), func(group PathGroup) {
					inspected = true
					if len(group.Paths) != 1 {
						t.Fatalf("%s: stored candidates = %d, want one hidden path", prefix, len(group.Paths))
					}
					if !group.Paths[0].Hidden {
						t.Fatalf("%s: rejected candidate is selectable", prefix)
					}
				})
				if !inspected {
					t.Fatalf("%s: rejected candidate disappeared", prefix)
				}
			}
			path, prefix, found := rib.LPM(tc.family, netip.MustParseAddr(tc.query))
			if tc.selected == "" {
				if found {
					t.Fatalf("LPM selected %s with only hidden covers", prefix)
				}
				return
			}
			if !found {
				t.Fatalf("LPM found no selectable cover, want %s", tc.selected)
			}
			if prefix != netip.MustParsePrefix(tc.selected) {
				t.Fatalf("LPM selected %s, want %s", prefix, tc.selected)
			}
			if path.Hidden {
				t.Fatal("LPM returned a hidden path")
			}
		})
	}
}
