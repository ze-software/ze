// Design: docs/architecture/api/architecture.md -- BGP secondary IP participant path resolution
// Related: register.go -- BGP config initialization hooks
// Related: ../../config/bus_validate.go -- generic BusSecondaryPathResolver seam

package bgpconfig

import (
	"net/netip"
	"slices"
	"strings"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	bgpSecondaryIPPrefix = "/bgp/ip/"
	bgpDynamicIPKeyword  = "dynamic"
)

// resolveBGPSecondaryIPPath resolves a secondary BGP participant lookup path of
// the form /bgp/ip/<address> against tree.
//
// Matching static peers (under bgp > peer or bgp > group > peer) return their
// canonical YANG participant paths with dormant=false. When no static peer uses
// the address, any configured dynamic group (connection > remote > ip ==
// "dynamic") whose connection > remote > range contains the address returns its
// canonical /bgp/group/<name> path with dormant=true (M-18). Addresses matching
// neither are refused with all configured peer IPs and dynamic-group ranges.
func resolveBGPSecondaryIPPath(tree *zeconfig.Tree, path string) (canonicalPaths []string, dormant bool, validDesc string, err error) {
	addrText := strings.TrimPrefix(path, bgpSecondaryIPPrefix)
	if addrText == "" {
		return nil, false, "", opererr.New(
			"bus path "+path+" is missing an IP address after /bgp/ip/",
			"/bgp/ip/<address> must name a valid IPv4 or IPv6 peer address",
			"write /bgp/ip/<ipv4-or-ipv6-address>")
	}
	if strings.Contains(addrText, "/") {
		return nil, false, "", opererr.New(
			"bus path "+path+" must name a single IP address, not a prefix or subpath",
			"/bgp/ip/<address> identifies one peer by remote IP address",
			"write /bgp/ip/<ipv4-or-ipv6-address>")
	}
	addr, parseErr := netip.ParseAddr(addrText)
	if parseErr != nil {
		var problem textbuf.Buffer
		problem.Str("bus path ").Quoted(path).Str(" does not contain a valid IP address: ").Quoted(addrText)
		return nil, false, "", opererr.New(
			problem.String(),
			"/bgp/ip/<address> requires a literal IPv4 or IPv6 address",
			"replace "+addrText+" with a valid IPv4 or IPv6 address")
	}
	addr = addr.Unmap()

	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil, false, "no BGP peers or dynamic groups configured", opererr.New(
			"bus path "+path+" matches no configured BGP peer",
			"no bgp configuration section is present",
			"configure the BGP peer or dynamic group first, or remove "+path)
	}

	var matchedPeers []string
	var matchedDynamicGroups []string
	var peerIPs []string
	var dynamicRanges []string

	// Walk standalone bgp > peer entries.
	for _, entry := range bgp.GetListOrdered("peer") {
		ip, ok := peerRemoteIP(entry.Key, entry.Value)
		if !ok {
			continue
		}
		peerIPs = append(peerIPs, ip.String())
		if ip == addr {
			matchedPeers = append(matchedPeers, "/bgp/peer/"+entry.Key)
		}
	}

	// Walk bgp > group entries (both dynamic ranges and grouped static peers).
	for _, groupEntry := range bgp.GetListOrdered("group") {
		groupName, groupTree := groupEntry.Key, groupEntry.Value
		if isDynamicGroupTree(groupTree) {
			prefixes := dynamicGroupRanges(groupTree)
			for _, pfx := range prefixes {
				dynamicRanges = append(dynamicRanges, pfx.String()+" (group "+groupName+")")
				if pfx.Contains(addr) {
					groupPath := "/bgp/group/" + groupName
					if !slices.Contains(matchedDynamicGroups, groupPath) {
						matchedDynamicGroups = append(matchedDynamicGroups, groupPath)
					}
				}
			}
		}
		for _, peerEntry := range groupTree.GetListOrdered("peer") {
			ip, ok := peerRemoteIP(peerEntry.Key, peerEntry.Value)
			if !ok {
				continue
			}
			peerIPs = append(peerIPs, ip.String())
			if ip == addr {
				matchedPeers = append(matchedPeers, "/bgp/group/"+groupName+"/peer/"+peerEntry.Key)
			}
		}
	}

	validDesc = formatValidBGPTargets(peerIPs, dynamicRanges)
	if len(matchedPeers) > 0 {
		slices.Sort(matchedPeers)
		return matchedPeers, false, validDesc, nil
	}
	if len(matchedDynamicGroups) > 0 {
		slices.Sort(matchedDynamicGroups)
		return matchedDynamicGroups, true, validDesc, nil
	}

	var problem, why, fix textbuf.Buffer
	problem.Str("bus path ").Quoted(path).Str(" matches no configured BGP peer IP or dynamic group range")
	why.Str("unconfigured /bgp/ip/<address> targets are accepted only when inside a dynamic group's remote range (").Str(validDesc).Byte(')')
	fix.Str("configure a peer with remote ip ").Str(addr.String()).Str(", add a dynamic group range covering it, or remove ").Str(path)
	return nil, false, validDesc, opererr.New(problem.String(), why.String(), fix.String())
}

func peerRemoteIP(peerKey string, peerTree *zeconfig.Tree) (netip.Addr, bool) {
	if peerTree != nil {
		if conn := peerTree.GetContainer("connection"); conn != nil {
			if remote := conn.GetContainer("remote"); remote != nil {
				if ipStr, ok := remote.Get("ip"); ok && ipStr != "" && ipStr != bgpDynamicIPKeyword {
					if addr, err := netip.ParseAddr(ipStr); err == nil {
						return addr.WithZone("").Unmap(), true
					}
				}
			}
		}
	}
	if addr, err := netip.ParseAddr(peerKey); err == nil {
		return addr.WithZone("").Unmap(), true
	}
	return netip.Addr{}, false
}

func isDynamicGroupTree(groupTree *zeconfig.Tree) bool {
	if groupTree == nil {
		return false
	}
	conn := groupTree.GetContainer("connection")
	if conn == nil {
		return false
	}
	remote := conn.GetContainer("remote")
	if remote == nil {
		return false
	}
	ipStr, ok := remote.Get("ip")
	return ok && ipStr == bgpDynamicIPKeyword
}

func dynamicGroupRanges(groupTree *zeconfig.Tree) []netip.Prefix {
	if groupTree == nil {
		return nil
	}
	conn := groupTree.GetContainer("connection")
	if conn == nil {
		return nil
	}
	remote := conn.GetContainer("remote")
	if remote == nil {
		return nil
	}
	rawRanges := remote.GetSlice("range")
	if len(rawRanges) == 0 {
		if single, ok := remote.Get("range"); ok && single != "" {
			rawRanges = []string{single}
		}
	}
	var out []netip.Prefix
	for _, r := range rawRanges {
		if pfx, err := netip.ParsePrefix(r); err == nil {
			out = append(out, pfx)
		}
	}
	return out
}

func formatValidBGPTargets(peerIPs, dynamicRanges []string) string {
	slices.Sort(peerIPs)
	peerIPs = slices.Compact(peerIPs)
	slices.Sort(dynamicRanges)
	dynamicRanges = slices.Compact(dynamicRanges)

	var tb textbuf.Buffer
	if len(peerIPs) > 0 {
		tb.Str("configured peer IPs: [ ").Join(peerIPs, ", ").Str(" ]")
	} else {
		tb.Str("configured peer IPs: none")
	}
	tb.Str("; ")
	if len(dynamicRanges) > 0 {
		tb.Str("dynamic group ranges: [ ").Join(dynamicRanges, ", ").Str(" ]")
	} else {
		tb.Str("dynamic group ranges: none")
	}
	return tb.String()
}
