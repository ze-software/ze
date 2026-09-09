package bgpconfig

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// treeWithNotation returns a minimal BGP tree carrying one as-notation leaf.
func treeWithNotation(token string) *config.Tree {
	tree := config.NewTree()
	bgp := buildBGPBlock()
	if token != "" {
		bgp.Set(asn.LeafName, token)
	}
	bgp.AddListEntry("peer", "peer1", buildMinimalPeer("10.0.0.1", "65001", "auto"))
	tree.SetContainer("bgp", bgp)
	return tree
}

// configureNotation records one notation for the rest of a test.
func configureNotation(t *testing.T, notation asn.Notation) {
	t.Helper()
	if err := asn.Configure(notation.String()); err != nil {
		t.Fatalf("asn.Configure(%s): %v", notation, err)
	}
}

// TestResolvingACandidateLeavesTheRenderedNotation proves that resolving a
// candidate configuration does not change what this process renders. The
// method resolves a candidate that selects asdot+, twice over a clone, and
// reads the notation back.
//
// This covers ResolveBGPTree and nothing else. `ze config validate`, `ze
// doctor` and the web commit path each reach it through
// infra.ValidateBGPPeers, PeersFromConfigTree and peersAndDynamicGroups, over
// a CLONE of the tree. A write here would survive their refusal.
//
// It is NOT the only refusable path. The reload coordinator's verify phase
// reaches the whole loader through reactorAPIAdapter.VerifyConfig, and
// TestReloadVerifyLeavesTheRenderedNotation covers that one over the branch
// that re-reads the file. Neither test covers the other's path.
//
// VALIDATES: ResolveBGPTree does not write the notation.
// PREVENTS: the write returning to the resolver, where a refused commit would
// leave an operator reading a notation that never took effect.
func TestResolvingACandidateLeavesTheRenderedNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })
	configureNotation(t, asn.NotationPlain)

	candidate := treeWithNotation(asn.TokenDotPlus)

	for range 2 {
		if _, err := ResolveBGPTree(candidate.Clone()); err != nil {
			t.Fatalf("resolving the candidate failed for an unrelated reason: %v", err)
		}
		if got := asn.Configured(); got != asn.NotationPlain {
			t.Errorf("resolving a candidate changed the rendered notation to %v, want %v unchanged", got, asn.NotationPlain)
		}
	}
}

// TestApplyingAConfigurationRecordsTheNotation proves the value DOES change
// when a configuration is applied, so the fix above did not disconnect the
// leaf. The method calls the function the two apply paths call.
//
// VALIDATES: applyASNotation records the leaf, and refuses a token that names
// no notation.
// PREVENTS: a test that passes because nothing writes the value at all.
func TestApplyingAConfigurationRecordsTheNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })
	configureNotation(t, asn.NotationPlain)

	if err := applyASNotation(treeWithNotation(asn.TokenDotPlus)); err != nil {
		t.Fatalf("applyASNotation: %v", err)
	}
	if got := asn.Configured(); got != asn.NotationDotPlus {
		t.Errorf("applying a configuration recorded %v, want %v", got, asn.NotationDotPlus)
	}

	// An absent leaf is asplain, which is what every release before the leaf
	// existed rendered.
	if err := applyASNotation(treeWithNotation("")); err != nil {
		t.Fatalf("applyASNotation with no leaf: %v", err)
	}
	if got := asn.Configured(); got != asn.NotationPlain {
		t.Errorf("an absent leaf recorded %v, want %v", got, asn.NotationPlain)
	}

	// A token that names no notation refuses the configuration rather than
	// rendering a notation the operator did not ask for.
	if err := applyASNotation(treeWithNotation("dot")); err == nil {
		t.Error("applyASNotation accepted a token that names no notation")
	}
}

// TestRouteAttributesReadEveryNotation proves the two route attributes that
// carry an AS number take any of the three RFC 5396 spellings. The method
// parses an AS path and an aggregator in each spelling.
//
// The aggregator is the interesting one: its value is `AS:IP`, so the AS half
// goes through asn.Parse while the address half stays with netip.
//
// VALIDATES: ParseASPath and ParseAggregator (routeattr.go).
// PREVENTS: an operator who set `as-notation asdot` writing 1.10 into a static
// route's as-path or aggregator and having it refused. That is AC-1 for two
// leaves typed `string`, which no schema type can cover.
func TestRouteAttributesReadEveryNotation(t *testing.T) {
	for _, path := range []string{"1.10 100", "65546 100", "[ 1.10 0.100 ]"} {
		parsed, err := ParseASPath(path)
		if err != nil {
			t.Fatalf("ParseASPath(%q): %v", path, err)
		}
		if len(parsed.Values) != 2 || parsed.Values[0] != 65546 || parsed.Values[1] != 100 {
			t.Errorf("ParseASPath(%q) = %v, want [65546 100]", path, parsed.Values)
		}
	}
	if _, err := ParseASPath("1.99999"); err == nil {
		t.Error("ParseASPath accepted an out-of-range asdot AS number")
	}

	for _, aggregator := range []string{"1.10:192.0.2.1", "65546:192.0.2.1", "( 1.10:192.0.2.1 )"} {
		parsed, err := ParseAggregator(aggregator)
		if err != nil {
			t.Fatalf("ParseAggregator(%q): %v", aggregator, err)
		}
		if parsed.ASN != 65546 {
			t.Errorf("ParseAggregator(%q) AS = %d, want 65546", aggregator, parsed.ASN)
		}
		if parsed.IP != [4]byte{192, 0, 2, 1} {
			t.Errorf("ParseAggregator(%q) IP = %v, want 192.0.2.1: the address half must be untouched", aggregator, parsed.IP)
		}
	}
	if _, err := ParseAggregator("1.10:not-an-ip"); err == nil {
		t.Error("ParseAggregator accepted an address that is not one")
	}
}

// TestRouteDistinguisherReadsEveryNotation proves an RD written as ASN:NN
// takes any of the three RFC 5396 spellings, and that the IPv4 form is
// untouched by that. The method parses the same RD in each spelling and
// compares the eight wire bytes.
//
// The AS branch is reached only after netip refuses the first field, so a
// dotted token lands there rather than in the IP:NN branch.
//
// VALIDATES: ParseRouteDistinguisher (routeattr.go).
// PREVENTS: `rd 1.10:5` being refused while `rd 65546:5` is accepted, which is
// AC-1 for a leaf typed `string` three functions from the two already fixed.
func TestRouteDistinguisherReadsEveryNotation(t *testing.T) {
	want, err := ParseRouteDistinguisher("65546:5")
	if err != nil {
		t.Fatalf("ParseRouteDistinguisher(65546:5): %v", err)
	}
	// AS 65546 needs four octets, so RFC 4364 type 2 carries it.
	if want.Bytes[1] != 2 {
		t.Fatalf("RD type = %d, want 2 for a four-byte AS number", want.Bytes[1])
	}

	for _, spelling := range []string{"1.10:5", "65546:5"} {
		got, err := ParseRouteDistinguisher(spelling)
		if err != nil {
			t.Fatalf("ParseRouteDistinguisher(%q): %v", spelling, err)
		}
		if got.Bytes != want.Bytes {
			t.Errorf("ParseRouteDistinguisher(%q) = %v, want %v", spelling, got.Bytes, want.Bytes)
		}
	}

	// An asdot+ spelling of a two-byte AS number still yields RFC 4364 type 0.
	small, err := ParseRouteDistinguisher("0.100:5")
	if err != nil {
		t.Fatalf("ParseRouteDistinguisher(0.100:5): %v", err)
	}
	plain, err := ParseRouteDistinguisher("100:5")
	if err != nil {
		t.Fatalf("ParseRouteDistinguisher(100:5): %v", err)
	}
	if small.Bytes != plain.Bytes {
		t.Errorf("ParseRouteDistinguisher(0.100:5) = %v, want %v", small.Bytes, plain.Bytes)
	}

	// The IPv4 form is decided before the AS branch, so it keeps type 1.
	ipForm, err := ParseRouteDistinguisher("192.0.2.1:5")
	if err != nil {
		t.Fatalf("ParseRouteDistinguisher(192.0.2.1:5): %v", err)
	}
	if ipForm.Bytes[1] != 1 {
		t.Errorf("RD type for 192.0.2.1:5 = %d, want 1", ipForm.Bytes[1])
	}

	// A dotted token that names no AS number is still refused.
	for _, bad := range []string{"1.99999:5", "65536.0:5", "1.2.3:5"} {
		if _, err := ParseRouteDistinguisher(bad); err == nil {
			t.Errorf("ParseRouteDistinguisher(%q) was accepted", bad)
		}
	}
}

// TestExtendedCommunityReadsEveryNotation proves the configured
// `extended-community` leaf takes a dotted administrator, and that the config
// reader agrees with the wire encoder about the bytes it means.
//
// The two are separate parsers of one text form: this one is reached from a
// config file, and attribute.ParseSingleExtCommunity from a command. They both
// call attribute.ParseExtCommunityAdmin now, which is what keeps them equal.
//
// VALIDATES: parseExtCommunityASN reads asplain, asdot and asdot+ (AC-1).
// PREVENTS: `extended-community target:1.10:5` refused in a config file.
func TestExtendedCommunityReadsEveryNotation(t *testing.T) {
	want, err := ParseExtendedCommunity("target:65546:5")
	if err != nil {
		t.Fatalf("target:65546:5: %v", err)
	}
	for _, spelling := range []string{"target:1.10:5", "target:1.10L:5"} {
		got, err := ParseExtendedCommunity(spelling)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		if len(got.Bytes) != len(want.Bytes) {
			t.Fatalf("%s wrote %d bytes, want %d", spelling, len(got.Bytes), len(want.Bytes))
		}
		for i := range got.Bytes {
			if got.Bytes[i] != want.Bytes[i] {
				t.Errorf("%s = %v, want %v", spelling, got.Bytes, want.Bytes)
				break
			}
		}
	}

	// The suffix forces the four-octet form for a value that does not need it,
	// in the dotted spelling as in the decimal one.
	forced, err := ParseExtendedCommunity("target:100L:5")
	if err != nil {
		t.Fatalf("target:100L:5: %v", err)
	}
	dottedForced, err := ParseExtendedCommunity("target:0.100L:5")
	if err != nil {
		t.Fatalf("target:0.100L:5: %v", err)
	}
	if len(forced.Bytes) == 0 || forced.Bytes[0] != 0x02 {
		t.Errorf("target:100L:5 type = %v, want 0x02", forced.Bytes)
	}
	for i := range forced.Bytes {
		if dottedForced.Bytes[i] != forced.Bytes[i] {
			t.Errorf("target:0.100L:5 = %v, want %v", dottedForced.Bytes, forced.Bytes)
			break
		}
	}

	if _, err := ParseExtendedCommunity("target:1.99999:5"); err == nil {
		t.Error("target:1.99999:5 was accepted")
	}
}
