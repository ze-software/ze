package bridge

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestTextExtensivePerClass proves each NLRI class is written as its ExaBGP
// extensive() (5.0.1), from the ExaBGP document entry of the route.
//
// VALIDATES: the prefix order cidr, path-information, label, next-hop, rd;
// a flow's `string`; VPLS; and Family.extensive for every other SAFI, an
// undecodable route included.
// PREVENTS: a text line built from ze's members, which put the path
// information after the next hop and wrote a flow as JSON.
func TestTextExtensivePerClass(t *testing.T) {
	cases := []struct {
		name, family, nextHop string
		announced             bool
		entry                 map[string]any
		want                  string
	}{
		{"vpn with path id", "ipv4 mpls-vpn", "192.168.0.1", true,
			map[string]any{"nlri": "10.0.0.0/24", "path-information": "0.0.0.10", "label": []any{[]any{float64(20012)}}, "rd": "100:100"},
			"10.0.0.0/24 path-information 0.0.0.10 label 20012 next-hop 192.168.0.1 rd 100:100"},
		{"label stack", "ipv4 nlri-mpls", "1.1.1.1", true,
			map[string]any{"nlri": "10.0.0.0/24", "label": []any{[]any{float64(1)}, []any{float64(2)}}},
			"10.0.0.0/24 label [ 1 2 ] next-hop 1.1.1.1"},
		{"vpn withdrawn", "ipv4 mpls-vpn", "", false,
			map[string]any{"nlri": "10.0.0.0/24", "label": []any{[]any{float64(524288)}}, "rd": "100:100"},
			"10.0.0.0/24 rd 100:100"},
		{"flow", "ipv4 flow", "1.2.3.4", true,
			map[string]any{"string": "flow destination-ipv4 192.168.0.1/32 next-hop 1.2.3.4"},
			"flow destination-ipv4 192.168.0.1/32 next-hop 1.2.3.4"},
		{"vpls", "l2vpn vpls", "192.0.2.2", true,
			map[string]any{"rd": "65001:100", "endpoint": float64(10), "base": float64(10000), "offset": float64(0), "size": float64(10)},
			"vpls rd 65001:100 endpoint 10 base 10000 offset 0 size 10 next-hop 192.0.2.2"},
		{"vpls without next hop", "l2vpn vpls", "", false,
			map[string]any{"rd": "65001:100", "endpoint": float64(1), "base": float64(2), "offset": float64(3), "size": float64(4)},
			"vpls rd 65001:100 endpoint 1 base 2 offset 3 size 4 "},
		{"mup", "ipv4 mup", "10.0.0.2", true, map[string]any{"name": "Type2SessionTransformedRoute"}, "afi ipv4 safi mup"},
		{"undecoded mup", "ipv6 mup", "::1", true, map[string]any{"nlri": "wire[ipv6/mup](51 bytes)"}, "afi ipv6 safi mup"},
		{"mvpn", "ipv4 mcast-vpn", "10.10.6.4", true, map[string]any{"code": float64(5)}, "afi ipv4 safi mcast-vpn"},
	}
	for _, tc := range cases {
		got := string(appendExtensive(nil, tc.family, tc.entry, tc.nextHop, tc.announced))
		if got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestTextCommunitiesSortByOctets proves a community list is written sorted by
// its octets and named as ExaBGP names it, and a list of one bare.
//
// VALIDATES: Communities.__repr__, with well-known names and large communities.
// PREVENTS: the peer's order, and `[ x ]` around a single member.
func TestTextCommunitiesSortByOctets(t *testing.T) {
	section := []byte{
		0xC0, 0x08, 12, 0x78, 0x14, 0x78, 0x14, 0xFF, 0xFF, 0xFF, 0x01, 0x78, 0x14, 0x00, 0x00, // COMMUNITY
		0xC0, 0x20, 12, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, // LARGE_COMMUNITY
	}
	wire := textAttributeWire{attrs: attribute.NewAttributesWire(section, bgpctx.APIContextID)}

	members, read := wire.communities(textCommunityKinds[zeAttrCommunities])
	if !read {
		t.Fatal("communities not read from the octets")
	}
	if got := string(appendCommunityListText(nil, members)); got != "[ 30740:0 30740:30740 no-export ]" {
		t.Errorf("communities = %q", got)
	}
	large, read := wire.communities(textCommunityKinds[zeAttrLargeCommunities])
	if !read {
		t.Fatal("large communities not read from the octets")
	}
	if got := string(appendCommunityListText(nil, large)); got != "1:2:3" {
		t.Errorf("large communities = %q", got)
	}
	if _, read := (textAttributeWire{}).communities(textCommunityKinds[zeAttrCommunities]); read {
		t.Error("an event without octets answered communities read from them")
	}
}

// TestTextPrefixSIDAndSimpson proves the SR-MPLS Prefix-SID text and the
// 0x0800 next-hop community words.
//
// VALIDATES: PrefixSid.__str__ and TrafficNextHopSimpson.__repr__ (5.0.1).
// PREVENTS: ` attribute [ 0xFF {...} ]` and `0x0800:000000000000` on a text line.
func TestTextPrefixSIDAndSimpson(t *testing.T) {
	if got := string(appendPrefixSIDText(nil, map[string]any{"sr-label-index": float64(777)})); got != "[ 777 ]" {
		t.Errorf("label index = %q", got)
	}
	both := map[string]any{"sr-label-index": float64(300), "sr-srgbs": []any{
		[]any{float64(800000), float64(4096)}, []any{float64(1000000), float64(5000)},
	}}
	if got := string(appendPrefixSIDText(nil, both)); got != "[ 300, [ ( 800000,4096 ), ( 1000000,5000 ) ] ]" {
		t.Errorf("label index and SRGB = %q", got)
	}
	for text, want := range map[string]string{
		"0x0800:000000000000": "redirect-to-nexthop",
		"0x0800:000000000001": "copy-to-nexthop",
		"target:1:1":          "target:1:1",
	} {
		if got := exabgpCommunityText(text); got != want {
			t.Errorf("%s = %q, want %q", text, got, want)
		}
	}
}
