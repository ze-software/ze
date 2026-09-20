package cli

import (
	"net"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
)

// genericPrefixNLRI is four bytes parseGenericNLRI reads as one prefix: length
// 24, then three address bytes. Fed to a family a plugin claims, a plain string
// in the answer therefore says the decode never reached that plugin.
var genericPrefixNLRI = []byte{0x18, 0x0a, 0x00, 0x00}

// unclaimedFamily answers a family the family registry holds that no plugin
// claims in this binary, so the test can claim it with a synthetic plugin.
//
// It is derived rather than named: which NLRI plugins a test binary links
// depends on its build tags, so any family named here is claimed under one set
// of tags and free under another.
func unclaimedFamily(t *testing.T) string {
	t.Helper()

	claimed := registry.FamilyMap()
	names := family.RegisteredFamilyNames()
	slices.Sort(names)
	for _, name := range names {
		if claimed[name] == "" {
			return name
		}
	}
	t.Fatal("every registered family is claimed by a plugin, so no family is free to claim")
	return ""
}

// TestParseNLRIByFamilyReachesAPluginItDoesNotName is the discrimination test
// for the registry derivation. It registers a plugin whose name appears nowhere
// in decode_mp.go, claiming a family decode_mp.go also never names, and proves
// the decode reaches it.
//
// A name list restored in parseNLRIByFamily makes this RED: the synthetic name
// is in no list, so the family falls to parseGenericNLRI and the answer is a
// prefix string rather than the plugin's own JSON.
//
// VALIDATES: parseNLRIByFamily routes by what the plugin registry answers for
// the family, not by a plugin name written into internal/component/bgp/cli.
// PREVENTS: a plugin that registers an NLRI family decoding as a plain prefix
// list because nobody edited a name list in a central package
// (ai/rules/plugins.md, ai/rules/principles.md).
func TestParseNLRIByFamilyReachesAPluginItDoesNotName(t *testing.T) {
	claimable := unclaimedFamily(t)

	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })

	const marker = "reached-through-the-registry"
	err := registry.Register(registry.Registration{
		Name:        "test-nlri-derivation",
		Description: "synthetic NLRI plugin proving decode_mp.go names no plugin",
		Families:    []string{claimable},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessNLRIDecoder: func(string, string, bool) (any, error) {
			return map[string]any{"decoder": marker}, nil
		},
	})
	if err != nil {
		t.Fatalf("register synthetic plugin: %v", err)
	}

	fam, held := family.LookupFamily(claimable)
	if !held {
		t.Fatalf("family registry does not hold %s", claimable)
	}

	routes := parseNLRIByFamily(genericPrefixNLRI, fam.AFI, fam.SAFI, true)
	if len(routes) != 1 {
		t.Fatalf("want one decoded NLRI, got %d answers: %v", len(routes), routes)
	}
	decoded, isObject := routes[0].(map[string]any)
	if !isObject {
		t.Fatalf("want the plugin's decoded object, got %T (%v): the generic prefix reader answered instead", routes[0], routes[0])
	}
	if decoded["decoder"] != marker {
		t.Errorf("want decoder %q, got %v", marker, decoded["decoder"])
	}
}

// TestParseNLRIByFamilyCoversEveryRegisteredFamily widens the proof to the
// whole registry: every family registry.FamilyMap() holds reaches the plugin
// that claimed it, so a plugin the tree gains is decoded with no edit to
// decode_mp.go.
//
// The assertion runs over the registry, not over a fixture list, so a plugin
// added to the composition root widens this test on its own.
//
// VALIDATES: every family the plugin registry claims reaches its plugin.
// PREVENTS: a partial name list that decodes some registered families and
// silently mis-reads the rest as prefixes.
func TestParseNLRIByFamilyCoversEveryRegisteredFamily(t *testing.T) {
	claimed := registry.FamilyMap()
	if len(claimed) == 0 {
		t.Fatal("registry.FamilyMap() answered no family: no NLRI plugin is linked into this binary")
	}

	for name, plugin := range claimed {
		fam, held := family.LookupFamily(name)
		if !held {
			t.Errorf("family %q is claimed by plugin %q and the family registry does not hold it", name, plugin)
			continue
		}
		routes := parseNLRIByFamily(genericPrefixNLRI, fam.AFI, fam.SAFI, true)
		if len(routes) == 0 {
			t.Errorf("family %q: no answer at all", name)
			continue
		}
		for _, route := range routes {
			prefix, isPrefix := route.(string)
			if !isPrefix {
				continue
			}
			t.Errorf("family %q: answered prefix %q from the generic reader, so plugin %q was never reached",
				name, prefix, plugin)
		}
	}
}

// TestParseNLRIByFamilyReadsUnclaimedFamilyAsPrefixes proves the other half of
// the derivation: a family NO plugin claims keeps the plain prefix reader. The
// registry decides that, so the test asserts the family is unclaimed rather
// than assuming it.
//
// VALIDATES: a family no plugin claims still reads as a plain prefix list.
// PREVENTS: a derivation that sends every family to a plugin and loses the
// IPv4/IPv6 unicast and multicast answer an operator reads.
func TestParseNLRIByFamilyReadsUnclaimedFamilyAsPrefixes(t *testing.T) {
	const name = "ipv4/unicast"

	if plugin := registry.FamilyMap()[name]; plugin != "" {
		t.Fatalf("plugin %q now claims %s, so the generic reader is no longer its route", plugin, name)
	}

	fam, held := family.LookupFamily(name)
	if !held {
		t.Fatalf("family registry does not hold %s", name)
	}

	routes := parseNLRIByFamily(genericPrefixNLRI, fam.AFI, fam.SAFI, true)
	if len(routes) != 1 {
		t.Fatalf("want one prefix, got %d answers: %v", len(routes), routes)
	}
	prefix, isPrefix := routes[0].(string)
	if !isPrefix {
		t.Fatalf("want a prefix string, got %T", routes[0])
	}
	if prefix != "10.0.0.0/24" {
		t.Errorf("want prefix 10.0.0.0/24, got %q", prefix)
	}
}
