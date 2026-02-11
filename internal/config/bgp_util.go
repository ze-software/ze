package config

import (
	"net/netip"
	"strings"
)

// ipToUint32 converts an IPv4 address to uint32.
func ipToUint32(ip netip.Addr) uint32 {
	if !ip.Is4() {
		return 0
	}
	b := ip.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// IPGlobMatch checks if an IP address matches a glob pattern.
// Pattern "*" matches any IP (IPv4 or IPv6).
// For IPv4, each octet can be "*" to match any value 0-255.
// For IPv6, supports trailing wildcard (2001:db8::*).
// CIDR notation is also supported (10.0.0.0/8, 2001:db8::/32).
// Examples: "192.168.*.*", "10.*.0.1", "*.*.*.1", "2001:db8::*", "10.0.0.0/8".
func IPGlobMatch(pattern, ip string) bool {
	// "*" matches everything
	if pattern == "*" {
		return true
	}

	// CIDR notation
	if strings.Contains(pattern, "/") {
		return cidrMatch(pattern, ip)
	}

	// IPv4 glob pattern (contains dots and wildcard)
	if strings.Contains(pattern, ".") && strings.Contains(ip, ".") {
		if strings.Contains(pattern, "*") {
			return ipv4GlobMatch(pattern, ip)
		}
		return pattern == ip
	}

	// IPv6 glob pattern (contains colons and wildcard)
	if strings.Contains(pattern, ":") && strings.Contains(ip, ":") {
		if strings.Contains(pattern, "*") {
			return ipv6GlobMatch(pattern, ip)
		}
		return pattern == ip
	}

	// Exact match fallback
	return pattern == ip
}

// cidrMatch checks if an IP is within a CIDR range.
func cidrMatch(cidr, ip string) bool {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	return prefix.Contains(addr)
}

// ipv6GlobMatch matches IPv6 addresses against trailing wildcard patterns.
// Supports patterns like "2001:db8::*" matching "2001:db8::1".
func ipv6GlobMatch(pattern, ip string) bool {
	// Handle trailing wildcard: 2001:db8::*
	if before, ok := strings.CutSuffix(pattern, "::*"); ok {
		prefix := before
		// The IP should start with the prefix followed by ::
		if strings.HasPrefix(ip, prefix+"::") {
			return true
		}
		// Or it could be an expanded form
		if strings.HasPrefix(ip, prefix+":") {
			return true
		}
		return false
	}

	// Handle mid-pattern wildcards (less common but supported)
	if strings.Contains(pattern, "*") {
		// Split on :: and handle each part
		patternParts := strings.Split(pattern, ":")
		ipParts := strings.Split(ip, ":")

		// Normalize both to full 8 groups for comparison
		patternParts = normalizeIPv6Parts(patternParts)
		ipParts = normalizeIPv6Parts(ipParts)

		if len(patternParts) != len(ipParts) {
			return false
		}

		for i := range patternParts {
			if patternParts[i] == "*" {
				continue
			}
			if patternParts[i] != ipParts[i] {
				return false
			}
		}
		return true
	}

	return pattern == ip
}

// normalizeIPv6Parts expands :: notation to full 8 groups.
func normalizeIPv6Parts(parts []string) []string {
	// Count empty strings (from :: split).
	emptyCount := 0
	for _, p := range parts {
		if p == "" {
			emptyCount++
		}
	}

	if emptyCount == 0 && len(parts) == 8 {
		return parts
	}

	// Need to expand :: to fill 8 groups.
	result := make([]string, 0, 8)
	for i, p := range parts {
		switch {
		case p == "" && i > 0 && i < len(parts)-1:
			// This is the :: expansion point.
			zerosNeeded := 8 - len(parts) + emptyCount
			for range zerosNeeded {
				result = append(result, "0")
			}
		case p != "":
			result = append(result, p)
		case i == 0 || i == len(parts)-1:
			// Leading or trailing empty from :: at start/end.
			result = append(result, "0")
		}
	}

	// Pad to 8 if needed.
	for len(result) < 8 {
		result = append(result, "0")
	}

	return result[:8]
}

// ipv4GlobMatch matches IPv4 addresses against glob patterns.
func ipv4GlobMatch(pattern, ip string) bool {
	patternParts := strings.Split(pattern, ".")
	ipParts := strings.Split(ip, ".")

	if len(patternParts) != 4 || len(ipParts) != 4 {
		return false
	}

	for i := range 4 {
		if patternParts[i] == "*" {
			continue // wildcard matches any octet
		}
		if patternParts[i] != ipParts[i] {
			return false
		}
	}
	return true
}

// PeerGlob holds a parsed peer glob pattern and its settings.
type PeerGlob struct {
	Pattern     string
	Specificity int
	Tree        *Tree
}

// ExtractEnvironment extracts environment configuration values from a parsed Tree.
// Returns a map suitable for passing to LoadEnvironmentWithConfig.
// The environment block is optional - returns empty map if not present.
func ExtractEnvironment(tree *Tree) map[string]map[string]string {
	envContainer := tree.GetContainer("environment")
	if envContainer == nil {
		return nil
	}

	result := make(map[string]map[string]string)

	// Extract each section (daemon, log, tcp, bgp, cache, api, reactor, debug)
	sections := []string{"daemon", "log", "tcp", "bgp", "cache", "api", "reactor", "debug"}
	for _, section := range sections {
		sectionContainer := envContainer.GetContainer(section)
		if sectionContainer == nil {
			continue
		}

		sectionValues := make(map[string]string)
		for _, option := range sectionContainer.Values() {
			value, _ := sectionContainer.Get(option)
			sectionValues[option] = value
		}

		if len(sectionValues) > 0 {
			result[section] = sectionValues
		}
	}

	return result
}
