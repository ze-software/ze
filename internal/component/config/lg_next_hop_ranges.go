// Design: docs/architecture/web-interface.md -- the named next-hop ranges of the looking glass
// Related: loader_extract.go -- ExtractLGSettings and ExtractLGConfig, the other readers of the block

package config

import (
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
)

// LGNextHopRangesMax is how many named next-hop ranges the looking glass
// takes. It restates `max-elements 256` of the list `next-hop-range` in
// internal/component/lg/yang/ze-lg-conf.yang, because the schema walk does not
// reach the `environment` section (validate_sections.go, validatedSections),
// so this reader is where the bound is enforced. The looking glass scans the
// whole list for each next hop it shows, which is why the list has a bound.
const LGNextHopRangesMax = 256

// lgNextHopRangePath is the config path of the list, as an operator writes it
// in a `set` or `delete` line.
const lgNextHopRangePath = "environment looking-glass next-hop-range "

// ExtractLGNextHopRanges reads environment.looking-glass.next-hop-range: the
// names an operator gave to ranges of next-hop addresses. The answer is keyed
// by the range with its host bits cleared, so one range holds one name.
//
// It reads the list whether or not the block says `enabled true`. The looking
// glass also starts from ze.looking-glass.enabled and ze.looking-glass.listen,
// with no `enabled` leaf in the block, and that looking glass shows these
// names too. The function takes no `enabled` answer at all, so no caller can
// gate the names on it (plan/learned/RECURRING-PATTERNS.md, "An `enabled` gate
// on config extraction discards the service's security settings").
//
// A nil answer with a nil error means the config names no range. An error
// means the list cannot be used as written, and each one is an operator error
// whose last line is the statement that repairs the config:
//
//   - two entries that name the same range, such as 198.51.100.1/24 beside
//     198.51.100.0/24, which are two list keys for one range
//   - more than LGNextHopRangesMax entries
//   - an entry whose key is not a prefix, or that has no name
//   - an entry whose key is an IPv4-mapped IPv6 range, such as
//     ::ffff:198.51.100.0/120, which is written 198.51.100.0/24
//
// The looking glass registers this function as a tree validator
// (internal/component/lg/register.go), so a commit, `ze config validate`, a
// daemon start and a reload all refuse the same lists. The hub calls it again
// at start and on reload to hand the names to the server.
func ExtractLGNextHopRanges(tree *Tree) (map[netip.Prefix]string, error) {
	if tree == nil {
		return nil, nil //nolint:nilnil // no tree names no range, which is an answer
	}
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return nil, nil //nolint:nilnil // no environment block names no range
	}
	lg := envBlock.GetContainer("looking-glass")
	if lg == nil {
		return nil, nil //nolint:nilnil // no looking-glass block names no range
	}
	entries := lg.GetListOrdered("next-hop-range")
	if len(entries) == 0 {
		return nil, nil //nolint:nilnil // a block with no entry names no range
	}
	if len(entries) > LGNextHopRangesMax {
		return nil, opererr.New(
			"environment looking-glass holds "+strconv.Itoa(len(entries))+" next-hop-range entries, and it takes "+
				strconv.Itoa(LGNextHopRangesMax)+" at most",
			"the looking glass reads the whole list for each next hop it shows, so the list has a fixed size",
			"delete "+lgNextHopRangePath+entries[LGNextHopRangesMax].Key)
	}

	names := make(map[netip.Prefix]string, len(entries))
	// writtenKeys holds the key each range was first written under, so the
	// refusal of a second entry names both entries as the operator wrote them.
	writtenKeys := make(map[netip.Prefix]string, len(entries))
	for _, entry := range entries {
		prefix, err := netip.ParsePrefix(entry.Key)
		if err != nil {
			return nil, opererr.Wrap(err,
				lgNextHopRangePath+entry.Key+" is not an address range",
				"a next-hop range is an IPv4 or IPv6 prefix, such as 198.51.100.0/24 or 2001:db8:100::/64",
				"delete "+lgNextHopRangePath+entry.Key)
		}
		// An IPv4-mapped IPv6 range names nothing: the looking glass reads a
		// mapped next hop as the IPv4 address it holds, so only an IPv4 range
		// contains it. Stored as written it would also sit beside the IPv4
		// range of the same addresses without being refused as its duplicate.
		if prefix.Addr().Is4In6() {
			return nil, opererr.New(
				lgNextHopRangePath+entry.Key+" is an IPv4-mapped IPv6 range, which names no next hop",
				"the looking glass reads an IPv4-mapped next hop as its IPv4 address, so the range that names it "+
					"is the IPv4 one"+lgMappedRangeAsIPv4(prefix),
				"delete "+lgNextHopRangePath+entry.Key)
		}
		prefix = prefix.Masked()
		if first, taken := writtenKeys[prefix]; taken {
			return nil, opererr.New(
				lgNextHopRangePath+first+" and "+lgNextHopRangePath+entry.Key+
					" name the same range, "+prefix.String(),
				"a range has one name, so the looking glass cannot choose which of the two to show beside a next hop",
				"delete "+lgNextHopRangePath+entry.Key)
		}
		name, _ := entry.Value.Get("name")
		if name == "" {
			return nil, opererr.New(
				lgNextHopRangePath+entry.Key+" has no name",
				"an entry with no name labels nothing, and the looking glass reads an empty name as no name",
				"set "+lgNextHopRangePath+entry.Key+" name <name>")
		}
		writtenKeys[prefix] = entry.Key
		names[prefix] = name
	}
	return names, nil
}

// lgMappedRangeAsIPv4 ends the reason an IPv4-mapped range is refused with the
// IPv4 range to write in its place, such as ", 198.51.100.0/24". mapped MUST
// be a prefix whose address is IPv4-mapped. It answers "" for a range shorter
// than the 96 bits every mapped address shares: that range is wider than the
// whole of IPv4, so no IPv4 range stands for it.
func lgMappedRangeAsIPv4(mapped netip.Prefix) string {
	const mappedBits = 96
	if mapped.Bits() < mappedBits {
		return ""
	}
	return ", " + netip.PrefixFrom(mapped.Addr().Unmap(), mapped.Bits()-mappedBits).Masked().String()
}
