package config

import (
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// lgRangesTree builds a tree whose environment.looking-glass block holds the
// given next-hop-range entries, in order, as key then name. enabled is written
// as the `enabled` leaf when it is not empty.
func lgRangesTree(enabled string, keyThenName ...string) *Tree {
	tree := NewTree()
	lg := tree.GetOrCreateContainer("environment").GetOrCreateContainer("looking-glass")
	if enabled != "" {
		lg.Set("enabled", enabled)
	}
	for i := 0; i+1 < len(keyThenName); i += 2 {
		entry := NewTree()
		if keyThenName[i+1] != "" {
			entry.Set("name", keyThenName[i+1])
		}
		lg.AddListEntry("next-hop-range", keyThenName[i], entry)
	}
	return tree
}

// lgRangesFix answers the fix line of the operator error err holds.
func lgRangesFix(t *testing.T, err error) string {
	t.Helper()
	var operator *opererr.Error
	if !errors.As(err, &operator) {
		t.Fatalf("error is %T (%v), want an operator error with a fix line", err, err)
	}
	return operator.Fix()
}

// TestExtractLGNextHopRangesIgnoresEnabled asks whether the named ranges are
// read when the block does not say `enabled true`, which is the config of a
// looking glass started from ze.looking-glass.enabled or ze.looking-glass.listen
// (AC-15). It reads the same list under each value of the leaf.
//
// PREVENTS: an `enabled` gate on the extraction, which would start that
// looking glass with no names.
func TestExtractLGNextHopRangesIgnoresEnabled(t *testing.T) {
	want := map[netip.Prefix]string{
		netip.MustParsePrefix("198.51.100.0/24"):    "ixp-lan-a",
		netip.MustParsePrefix("2001:db8:100::/64"): "ixp-lan-a",
	}
	for _, enabled := range []string{"", "false", "true"} {
		t.Run("enabled="+enabled, func(t *testing.T) {
			tree := lgRangesTree(enabled, "198.51.100.0/24", "ixp-lan-a", "2001:db8:100::/64", "ixp-lan-a")
			got, err := ExtractLGNextHopRanges(tree)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(got) != len(want) {
				t.Fatalf("ranges = %v, want %v", got, want)
			}
			for prefix, name := range want {
				if got[prefix] != name {
					t.Errorf("range %s is named %q, want %q", prefix, got[prefix], name)
				}
			}
		})
	}
}

// TestExtractLGNextHopRangesNoList asks what a config that names no range
// answers: no range and no error, for a nil tree, a tree with no block and a
// block with no entry.
func TestExtractLGNextHopRangesNoList(t *testing.T) {
	for name, tree := range map[string]*Tree{
		"nil tree":      nil,
		"no block":      NewTree(),
		"block, no entry": lgRangesTree("true"),
	} {
		got, err := ExtractLGNextHopRanges(tree)
		if err != nil {
			t.Errorf("%s: error %v, want none", name, err)
		}
		if len(got) != 0 {
			t.Errorf("%s: ranges %v, want none", name, got)
		}
	}
}

// TestExtractLGNextHopRangesRefusesOneRangeTwice asks whether two entries that
// name one range are refused with both entries and the line to remove (AC-12).
// 198.51.100.1/24 and 198.51.100.0/24 are two list keys, which the parser
// accepts, for one range.
//
// PREVENTS: a name that depends on the order of the entries.
func TestExtractLGNextHopRangesRefusesOneRangeTwice(t *testing.T) {
	tree := lgRangesTree("", "198.51.100.0/24", "ixp-lan-a", "198.51.100.1/24", "ixp-lan-b")
	_, err := ExtractLGNextHopRanges(tree)
	if err == nil {
		t.Fatal("two entries for 198.51.100.0/24 were accepted")
	}
	for _, entry := range []string{"next-hop-range 198.51.100.0/24", "next-hop-range 198.51.100.1/24"} {
		if !strings.Contains(err.Error(), entry) {
			t.Errorf("refusal does not name the entry %q: %v", entry, err)
		}
	}
	if fix, want := lgRangesFix(t, err), "delete environment looking-glass next-hop-range 198.51.100.1/24"; fix != want {
		t.Errorf("fix = %q, want %q", fix, want)
	}
}

// TestExtractLGNextHopRangesAcceptsNestedRanges asks whether distinct prefixes
// commit, nested ones included (AC-12, permitted survivors), and whether a key
// written with host bits names the range it is in.
func TestExtractLGNextHopRangesAcceptsNestedRanges(t *testing.T) {
	tree := lgRangesTree("", "198.51.100.0/24", "ixp-lan", "198.51.100.0/25", "ixp-lan-low", "198.51.100.200/32", "rs1",
		"203.0.113.9/24", "transit")
	got, err := ExtractLGNextHopRanges(tree)
	if err != nil {
		t.Fatalf("nested ranges were refused: %v", err)
	}
	for prefix, name := range map[string]string{
		"198.51.100.0/24": "ixp-lan", "198.51.100.0/25": "ixp-lan-low", "198.51.100.200/32": "rs1",
		"203.0.113.0/24": "transit",
	} {
		if got[netip.MustParsePrefix(prefix)] != name {
			t.Errorf("range %s is named %q, want %q", prefix, got[netip.MustParsePrefix(prefix)], name)
		}
	}
}

// TestExtractLGNextHopRangesCountBoundary asks where the list ends: 256
// entries are read and 257 are refused, with a fix that removes the entry
// past the bound. This reader is the layer that enforces the bound, because
// the schema walk does not reach the environment section.
func TestExtractLGNextHopRangesCountBoundary(t *testing.T) {
	build := func(count int) *Tree {
		entries := make([]string, 0, 2*count)
		for i := range count {
			entries = append(entries, "10."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256)+".0/24", "range-"+strconv.Itoa(i))
		}
		return lgRangesTree("", entries...)
	}

	got, err := ExtractLGNextHopRanges(build(LGNextHopRangesMax))
	if err != nil {
		t.Fatalf("%d entries were refused: %v", LGNextHopRangesMax, err)
	}
	if len(got) != LGNextHopRangesMax {
		t.Fatalf("read %d ranges, want %d", len(got), LGNextHopRangesMax)
	}

	_, err = ExtractLGNextHopRanges(build(LGNextHopRangesMax + 1))
	if err == nil {
		t.Fatalf("%d entries were accepted", LGNextHopRangesMax+1)
	}
	// Entry 257 is 10.1.0.0/24: index 256.
	if fix, want := lgRangesFix(t, err), "delete environment looking-glass next-hop-range 10.1.0.0/24"; fix != want {
		t.Errorf("fix = %q, want %q", fix, want)
	}
}

// TestExtractLGNextHopRangesPrefixLengthBoundary asks which prefix lengths the
// reader takes: 32 and 128 are the last valid ones, 33 and 129 are refused.
func TestExtractLGNextHopRangesPrefixLengthBoundary(t *testing.T) {
	for key, valid := range map[string]bool{
		"198.51.100.7/32":      true,
		"198.51.100.7/33":      false,
		"2001:db8:100::7/128": true,
		"2001:db8:100::7/129": false,
		"198.51.100.0":         false,
	} {
		_, err := ExtractLGNextHopRanges(lgRangesTree("", key, "rs1"))
		if valid && err != nil {
			t.Errorf("%s was refused: %v", key, err)
		}
		if !valid && err == nil {
			t.Errorf("%s was accepted", key)
		}
	}
}

// TestExtractLGNextHopRangesRefusesEntryWithoutName asks whether an entry with
// no name (name length 0) is refused, with the line that names it.
func TestExtractLGNextHopRangesRefusesEntryWithoutName(t *testing.T) {
	_, err := ExtractLGNextHopRanges(lgRangesTree("", "198.51.100.0/24", ""))
	if err == nil {
		t.Fatal("an entry with no name was accepted")
	}
	if fix, want := lgRangesFix(t, err), "set environment looking-glass next-hop-range 198.51.100.0/24 name <name>"; fix != want {
		t.Errorf("fix = %q, want %q", fix, want)
	}
}

// TestExtractLGNextHopRangesRefusesMappedRange asks whether a range written
// IPv4-mapped is refused, naming the IPv4 range to write, alone and beside the
// IPv4 range of the same addresses.
//
// PREVENTS: ::ffff:198.51.100.0/120 stored as an IPv6 range, which names no
// next hop (the looking glass unmaps the address it looks up) and is not seen
// as a second entry for 198.51.100.0/24.
func TestExtractLGNextHopRangesRefusesMappedRange(t *testing.T) {
	const mapped = "::ffff:198.51.100.0/120"
	const fix = "delete environment looking-glass next-hop-range " + mapped

	for what, tree := range map[string]*Tree{
		"alone":                 lgRangesTree("", mapped, "ixp-lan-a"),
		"beside the IPv4 range": lgRangesTree("", "198.51.100.0/24", "ixp-lan-a", mapped, "ixp-lan-b"),
	} {
		_, err := ExtractLGNextHopRanges(tree)
		if err == nil {
			t.Fatalf("%s: the mapped range was accepted", what)
		}
		if !strings.Contains(err.Error(), "198.51.100.0/24") {
			t.Errorf("%s: the refusal does not name the IPv4 range to write: %v", what, err)
		}
		if got := lgRangesFix(t, err); got != fix {
			t.Errorf("%s: fix = %q, want %q", what, got, fix)
		}
	}

	// Shorter than the 96 bits every mapped address shares: refused, and no
	// IPv4 range stands for it.
	_, err := ExtractLGNextHopRanges(lgRangesTree("", "::ffff:0:0/90", "all-mapped"))
	if err == nil {
		t.Fatal("::ffff:0:0/90 was accepted")
	}
	if got, want := lgRangesFix(t, err), "delete environment looking-glass next-hop-range ::ffff:0:0/90"; got != want {
		t.Errorf("fix = %q, want %q", got, want)
	}

	// The IPv6 range beside the mapped block is no mapped range.
	if _, err := ExtractLGNextHopRanges(lgRangesTree("", "::fffe:0:0/96", "not-mapped")); err != nil {
		t.Errorf("::fffe:0:0/96 was refused: %v", err)
	}
}

// TestLGNextHopRangeSchemaBoundaries asks what the schema takes when the list
// is written in a config file: the parser is the layer that enforces the
// prefix type and the length of the name (1 to 64 characters). Each config
// holds one entry and differs from the others in one value.
func TestLGNextHopRangeSchemaBoundaries(t *testing.T) {
	config := func(prefix, name string) string {
		return "environment {\n\tlooking-glass {\n\t\tnext-hop-range " + prefix + " {\n\t\t\tname " + name + "\n\t\t}\n\t}\n}\n"
	}
	name64 := strings.Repeat("a", 64)
	for _, tt := range []struct {
		what, prefix, name string
		valid              bool
	}{
		{"IPv4 length 32", "198.51.100.7/32", "rs1", true},
		{"IPv4 length 33", "198.51.100.7/33", "rs1", false},
		{"IPv6 length 128", "2001:db8:100::7/128", "rs1", true},
		{"IPv6 length 129", "2001:db8:100::7/129", "rs1", false},
		{"name of 64 characters", "198.51.100.0/24", name64, true},
		{"name of 65 characters", "198.51.100.0/24", name64 + "a", false},
		{"name starting with a dash", "198.51.100.0/24", "-lan", false},
		// The looking glass is a public page and shows the name as written,
		// so the pattern is what keeps markup and a space out of it.
		{"name holding a less-than sign", "198.51.100.0/24", "lan<b", false},
		{"name holding a space", "198.51.100.0/24", `"ixp lan"`, false},
		{"name holding a slash", "198.51.100.0/24", "ixp/lan", false},
		{"name of letters, digits, dot, underscore and dash", "198.51.100.0/24", "ixp_lan-1.a", true},
	} {
		t.Run(tt.what, func(t *testing.T) {
			tree, err := ParseTreeForValidation(config(tt.prefix, tt.name))
			if !tt.valid {
				if err == nil {
					t.Fatalf("the parser accepted next-hop-range %s name %s", tt.prefix, tt.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("the parser refused next-hop-range %s name %s: %v", tt.prefix, tt.name, err)
			}
			ranges, err := ExtractLGNextHopRanges(tree)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if got := ranges[netip.MustParsePrefix(tt.prefix).Masked()]; got != tt.name {
				t.Errorf("range %s is named %q, want %q", tt.prefix, got, tt.name)
			}
		})
	}
}
