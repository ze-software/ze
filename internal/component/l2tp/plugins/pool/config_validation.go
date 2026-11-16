// Design: docs/guide/l2tp.md -- bounded, disjoint subscriber pools
// Related: config.go -- parse descriptions before any bitmap is built

package l2tppool

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
)

// ipv4PoolSizeMax bounds one pool bitmap to 128 KiB. Parsing checks the range
// in uint64 before construction, so a uint32 word-count wrap cannot bypass it.
const ipv4PoolSizeMax uint64 = 1 << 20

// validateIPv4Description checks the constraints beyond address parsing. The
// parser MUST establish native IPv4, ordered endpoints before this call, and
// MUST call this before it constructs a bitmap. Invalid optional DNS addresses
// mean that leaf is absent; a present IPv6 address is refused.
func validateIPv4Description(pool ipv4PoolDescription, path string) error {
	start := uint64(addrToUint32(pool.start))
	end := uint64(addrToUint32(pool.end))
	size := end - start + 1
	if size > ipv4PoolSizeMax {
		return opererr.New(
			fmt.Sprintf("%s range %s-%s contains %d addresses; maximum is %d", path, pool.start, pool.end, size, ipv4PoolSizeMax),
			"The range exceeds the address pool's bounded bitmap size",
			fmt.Sprintf("set %s end %s", path, uint32ToAddr(uint32(start+ipv4PoolSizeMax-1))))
	}
	if start == 0 {
		fix := "delete " + path
		if end > 0 {
			fix = "set " + path + " start 0.0.0.1"
		}
		return opererr.New(path+" range contains 0.0.0.0",
			"The unspecified address cannot be assigned to a subscriber", fix)
	}
	if end == uint64(^uint32(0)) {
		fix := "delete " + path
		if start < end {
			fix = "set " + path + " end 255.255.255.254"
		}
		return opererr.New(path+" range contains 255.255.255.255",
			"The broadcast address cannot be assigned to a subscriber", fix)
	}
	if err := validatePoolDNSAddress(pool.dnsPrimary, path, "dns-primary"); err != nil {
		return err
	}
	return validatePoolDNSAddress(pool.dnsSecondary, path, "dns-secondary")
}

func validatePoolDNSAddress(address netip.Addr, path, leaf string) error {
	if !address.IsValid() {
		return nil
	}
	if address.Is4() {
		return nil
	}
	return opererr.New(fmt.Sprintf("%s %s is %s, which is not IPv4", path, leaf, address),
		"IPCP can offer only IPv4 DNS servers", "delete "+path+" "+leaf)
}

// The range collections below exist only during config validation. Their size
// is bounded by the number of pool entries in the parsed config; sorting makes
// the first reported overlap independent of Go map iteration.
type poolIPv4Range struct {
	path       string
	start, end netip.Addr
}

type poolIPv6Block struct {
	path  string
	block netip.Prefix
}

func validatePoolConfiguration(config poolConfiguration) error {
	if err := validatePoolIPv4Ranges(config); err != nil {
		return err
	}
	return validatePoolIPv6Blocks(config)
}

func validatePoolIPv4Ranges(config poolConfiguration) error {
	ranges := make([]poolIPv4Range, 0, len(config.namedIPv4)+1)
	if pool := config.defaultIPv4; pool != nil {
		ranges = append(ranges, poolIPv4Range{path: "l2tp pool ipv4", start: pool.start, end: pool.end})
	}
	for name := range config.namedIPv4 {
		path := "l2tp pool named-pool " + configtext.QuoteCommandWord(name)
		ranges = append(ranges, poolIPv4Range{path: path, start: config.namedIPv4[name].start, end: config.namedIPv4[name].end})
	}
	slices.SortFunc(ranges, func(first, second poolIPv4Range) int {
		if order := first.start.Compare(second.start); order != 0 {
			return order
		}
		return strings.Compare(first.path, second.path)
	})
	for index := 1; index < len(ranges); index++ {
		previous := ranges[index-1]
		current := ranges[index]
		if current.start.Compare(previous.end) > 0 {
			continue
		}
		end := previous.end
		if current.end.Compare(end) < 0 {
			end = current.end
		}
		return opererr.New(
			fmt.Sprintf("%s and %s overlap at %s-%s", previous.path, current.path, current.start, end),
			"Two pools could assign the same address to different subscribers",
			"delete "+current.path)
	}
	return nil
}

func validatePoolIPv6Blocks(config poolConfiguration) error {
	blocks := make([]poolIPv6Block, 0, len(config.namedIPv6)+1)
	if pool := config.defaultIPv6; pool != nil {
		blocks = append(blocks, poolIPv6Block{path: "l2tp pool ipv6-pd", block: pool.block})
	}
	for name, pool := range config.namedIPv6 {
		path := "l2tp pool named-ipv6-pool " + configtext.QuoteCommandWord(name)
		blocks = append(blocks, poolIPv6Block{path: path, block: pool.block})
	}
	slices.SortFunc(blocks, func(first, second poolIPv6Block) int {
		if order := first.block.Addr().Compare(second.block.Addr()); order != 0 {
			return order
		}
		return strings.Compare(first.path, second.path)
	})
	for index := 1; index < len(blocks); index++ {
		previous := blocks[index-1]
		current := blocks[index]
		if !previous.block.Overlaps(current.block) {
			continue
		}
		return opererr.New(fmt.Sprintf("%s block %s overlaps %s block %s", previous.path, previous.block, current.path, current.block),
			"Two pools could delegate overlapping prefixes to different subscribers", "delete "+current.path)
	}
	return nil
}
