// Design: docs/architecture/testing/interop.md -- selected-network wire inputs.
// Related: prepare.go -- renderScenario preserves the injector's non-address bytes.
package bgp

import (
	"encoding/binary"
	"encoding/hex"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// renderInjectedNextHops changes only IPv4 next-hop address fields of complete
// literal UPDATE sends whose addresses belong to the default lab /24. Other
// attributes, RDs, IPv6 addresses and NLRI remain byte-for-byte intact.
func renderInjectedNextHops(input string, network [4]byte) string {
	if network[0] == 172 && network[1] == 30 && network[2] == 0 {
		return input
	}
	var rendered textbuf.Buffer
	position, copied := 0, 0
	for line := range strings.SplitAfterSeq(input, "\n") {
		start := position
		position += len(line)
		if !strings.HasPrefix(strings.TrimLeft(line, " \t"), "action=send:") {
			continue
		}
		header, encoded, found := strings.Cut(line, ":hex=")
		if !found {
			continue
		}
		leading := len(encoded) - len(strings.TrimLeft(encoded, " \t"))
		end := len(strings.TrimRight(encoded, " \t\r\n"))
		if leading >= end {
			continue
		}
		frame, err := hex.DecodeString(encoded[leading:end])
		if err != nil {
			// Negative protocol fixtures are inputs, not renderer errors.
			continue
		}
		if !renderInjectedNextHop(frame, network) {
			continue
		}
		rendered.Str(input[copied:start]).Str(header).Str(":hex=").Str(encoded[:leading]).
			HexUpper(frame).Str(encoded[end:])
		copied = position
	}
	if copied == 0 {
		return input
	}
	return rendered.Str(input[copied:]).String()
}

// renderInjectedNextHop validates the complete envelope and attribute sequence
// before changing the owned frame. Malformed framing or duplicate attribute
// codes remain the negative input the scenario deliberately supplied.
func renderInjectedNextHop(frame []byte, network [4]byte) bool {
	if len(frame) < bgpHeaderLength {
		return false
	}
	if frame[18] != bgpUpdate {
		return false
	}
	if int(binary.BigEndian.Uint16(frame[16:18])) != len(frame) {
		return false
	}
	for _, octet := range frame[:16] {
		if octet != 0xff {
			return false
		}
	}
	body := frame[bgpHeaderLength:]
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return false
	}
	attributes := sections.Attrs(body)
	iterator := attribute.NewAttrIterator(attributes)
	offsets := [2]int{-1, -1}
	var seen [4]uint64
	for code, flags, value, ok := iterator.Next(); ok; code, flags, value, ok = iterator.Next() {
		word, bit := code>>6, uint64(1)<<(code&63)
		if seen[word]&bit != 0 {
			return false
		}
		seen[word] |= bit
		switch code {
		case attribute.AttrNextHop:
			if len(value) != 4 {
				return false
			}
			if flags & ^attribute.FlagExtLength != attribute.FlagTransitive {
				return false
			}
			offsets[0] = iterator.Offset() - len(value)
		case attribute.AttrMPReachNLRI:
			if flags & ^attribute.FlagExtLength != attribute.FlagOptional {
				return false
			}
			if len(value) < 5 || len(value) < 5+int(value[3]) {
				return false
			}
			// RFC 4760 Section 3: the existing wire view locates the address,
			// including the RD prefix of a VPN next hop, without copying NLRI.
			mp := wireu.MPReachWire(value)
			address := mp.NextHop()
			hopBytes := mp.NextHopBytes()
			if len(hopBytes) != 0 && !address.IsValid() {
				return false
			}
			if address.Is4() {
				offsets[1] = iterator.Offset() - len(value) + 4 + len(hopBytes) - 4
			}
		default:
			// Other attribute codes are an open wire population.
		}
	}
	if iterator.Remaining() != 0 {
		return false
	}
	changed := false
	for _, offset := range offsets {
		if offset < 0 {
			continue
		}
		nextHop := attributes[offset : offset+4]
		if nextHop[0] != 172 || nextHop[1] != 30 || nextHop[2] != 0 {
			continue
		}
		copy(nextHop[:3], network[:3])
		changed = true
	}
	return changed
}
