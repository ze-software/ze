// Design: docs/architecture/plugin/rib-storage-design.md — show command filter parsing and matching
// Overview: rib_commands.go — RIB command handlers that use these filters
// Related: rib_attr_format.go — formatCommunities, formatASPath used by matchers
package rib

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
)

// showFilters holds parsed filter parameters for show commands.
// Extracted from command args by parseShowFilters.
type showFilters struct {
	family    string         // address family filter (e.g., "ipv4/unicast")
	prefix    string         // prefix filter (e.g., "10.0.0.0/24")
	community string         // community filter in "high:low" format (e.g., "65000:100")
	asPathRe  *regexp.Regexp // compiled AS-path regex, nil if not specified
}

// parseShowFilters extracts family, prefix, community, and regexp filters from command args.
// Family: "afi/safi" (e.g., "ipv4/unicast") — starts with letter, no colons.
// Prefix: IP/len (e.g., "10.0.0.0/24", "fc00::/7") — has digits or colons.
// Community: "community" keyword followed by "high:low" value.
// Regexp: "regexp" keyword followed by pattern (matched against space-separated AS-path).
// Returns error for invalid regexp or unrecognized arguments.
func parseShowFilters(args []string) (showFilters, error) {
	var f showFilters
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "community" {
			if i+1 >= len(args) {
				return showFilters{}, fmt.Errorf("community requires a value")
			}
			f.community = args[i+1]
			i += 2
			continue
		}
		if arg == "regexp" {
			if i+1 >= len(args) {
				return showFilters{}, fmt.Errorf("regexp requires a pattern")
			}
			re, err := regexp.Compile(args[i+1])
			if err != nil {
				return showFilters{}, fmt.Errorf("invalid regexp %q: %w", args[i+1], err)
			}
			f.asPathRe = re
			i += 2
			continue
		}
		if strings.Contains(arg, "/") {
			// Family names never contain colons; IPv6 prefixes always do.
			if arg[0] >= 'a' && arg[0] <= 'z' && !strings.Contains(arg, ":") {
				f.family = arg
			} else {
				f.prefix = arg
			}
			i++
			continue
		}
		return showFilters{}, fmt.Errorf("unrecognized filter argument: %s", arg)
	}
	return f, nil
}

// entryMatchesCommunity checks if a RouteEntry has a specific community.
// community should be in "high:low" format (e.g., "65000:100").
func entryMatchesCommunity(entry *storage.RouteEntry, community string) bool {
	if !entry.HasCommunities() {
		return false
	}
	data, err := pool.Communities.Get(entry.Communities)
	if err != nil {
		return false
	}
	return slices.Contains(formatCommunities(data), community)
}

// entryMatchesASPathRegexp checks if a RouteEntry's AS-path matches the regex.
// The AS-path is formatted as space-separated ASNs (e.g., "64501 64502").
func entryMatchesASPathRegexp(entry *storage.RouteEntry, re *regexp.Regexp) bool {
	if !entry.HasASPath() {
		return false
	}
	data, err := pool.ASPath.Get(entry.ASPath)
	if err != nil {
		return false
	}
	asPath := formatASPath(data)
	if asPath == nil {
		return false
	}
	parts := make([]string, len(asPath))
	for i, asn := range asPath {
		parts[i] = strconv.FormatUint(uint64(asn), 10)
	}
	return re.MatchString(strings.Join(parts, " "))
}
