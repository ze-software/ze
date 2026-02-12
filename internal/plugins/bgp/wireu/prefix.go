package wireu

import "net/netip"

// ParseIPv4Prefixes parses a sequence of IPv4 prefixes from wire bytes.
func ParseIPv4Prefixes(data []byte) []netip.Prefix {
	return ParsePrefixes(data, 4)
}

// ParseIPv6Prefixes parses a sequence of IPv6 prefixes from wire bytes.
func ParseIPv6Prefixes(data []byte) []netip.Prefix {
	return ParsePrefixes(data, 16)
}

// ParsePrefixes parses a sequence of IP prefixes with the given address size (4 or 16).
func ParsePrefixes(data []byte, addrSize int) []netip.Prefix {
	var prefixes []netip.Prefix
	for i := 0; i < len(data); {
		prefixLen := int(data[i])
		i++
		prefixBytes := (prefixLen + 7) / 8
		if i+prefixBytes > len(data) {
			break
		}
		addrBytes := make([]byte, addrSize)
		copy(addrBytes, data[i:i+prefixBytes])
		i += prefixBytes

		var addr netip.Addr
		if addrSize == 4 {
			addr = netip.AddrFrom4([4]byte(addrBytes))
		} else {
			addr = netip.AddrFrom16([16]byte(addrBytes))
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, prefixLen))
	}
	return prefixes
}
