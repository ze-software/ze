package filter_irr

import (
	"net/netip"
	"testing"
)

// TestMPMarkerBlocksAreNotTheLegacyBlock proves the plugin reads the mp marker
// of an MP_REACH_NLRI block, so IPv4 unicast carried in MP_REACH_NLRI is not
// mistaken for the legacy NLRI block it may rewrite.
//
// VALIDATES: the marker block parses (no parse error) under its own family and
// op; a rejected prefix in it is dropped from that block by the modify delta,
// never kept by echoing the block, and a rejected prefix in a block of a family
// the engine cannot rewrite makes the UPDATE not rewritable; the modify delta
// keeps the marker on the block it echoes.
// PREVENTS: an "nlri add" block from MP_REACH_NLRI judged rewritable as if it
// were the legacy block, which let a refused prefix reach the RIB through the
// MP attribute.
func TestMPMarkerBlocksAreNotTheLegacyBlock(t *testing.T) {
	list := &irrPrefixList{entries: []prefixEntry{{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 8, le: 24}}}

	p := list.partitionUpdate("nlri add 10.0.1.0/24 mp ipv4/unicast add 192.168.1.0/24 10.0.2.0/24")
	if p.hadParseError {
		t.Fatal("an mp block must parse")
	}
	if len(p.blocks) != 2 || p.blocks[0].mp || !p.blocks[1].mp || p.blocks[1].family != "ipv4/unicast" || p.blocks[1].op != "add" {
		t.Fatalf("blocks = %+v, want a legacy block then an mp ipv4/unicast add block", p.blocks)
	}
	if !p.rewritable() {
		t.Fatal("a refused prefix in an MP_REACH_NLRI block of a CIDR family is rewritable")
	}
	if got, want := buildModifyDelta(p), "nlri add 10.0.1.0/24 mp ipv4/unicast add 10.0.2.0/24"; got != want {
		t.Errorf("buildModifyDelta = %q, want %q", got, want)
	}

	for _, field := range []string{
		"nlri add 10.0.1.0/24 mp ipv4/flow add 192.168.1.0/24",
		"nlri add 10.0.1.0/24 mp ipv4/mpls-vpn add 192.168.1.0/24",
		"nlri add 10.0.1.0/24 mp ipv4/no-such-family add 192.168.1.0/24",
	} {
		refused := list.partitionUpdate(field)
		if refused.rewritable() {
			t.Errorf("%q: a refused prefix in a block the engine cannot rewrite must make the UPDATE not rewritable", field)
		}
	}

	p = list.partitionUpdate("nlri add 10.0.1.0/24 192.168.1.0/24 mp ipv6/unicast del 2001:db8::/32")
	if !p.rewritable() {
		t.Fatal("a refused prefix in the legacy block alone is rewritable")
	}
	want := "nlri add 10.0.1.0/24 mp ipv6/unicast del 2001:db8::/32"
	if got := buildModifyDelta(p); got != want {
		t.Errorf("buildModifyDelta = %q, want %q", got, want)
	}
}
