// Design: docs/architecture/core-design.md — ExaBGP text command to ZeBGP translation
// Related: bridge.go — startup protocol, bridge runtime
// Related: bridge_event.go — ZeBGP to ExaBGP JSON event translation

package exabgp

import (
	"fmt"
	"regexp"
	"strings"
)

// ExabgpToZebgpCommand converts an ExaBGP text command to ZeBGP format.
//
// ExaBGP: neighbor <ip> announce route <prefix> next-hop <nh> [origin <o>] ...
// ZeBGP:  peer <ip> update text nhop set <nh> origin set <o> nlri ipv4/unicast add <prefix>.
func ExabgpToZebgpCommand(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}

	// Parse neighbor command
	neighborRE := regexp.MustCompile(`(?i)^neighbor\s+(\S+)\s+(.+)$`)
	match := neighborRE.FindStringSubmatch(line)
	if match == nil {
		// Not a neighbor command - pass through
		return line
	}

	peerIP := match[1]
	rest := strings.TrimSpace(match[2])
	restLower := strings.ToLower(rest)

	// Handle announce route
	if strings.HasPrefix(restLower, "announce route") {
		return convertAnnounce(peerIP, rest[14:])
	}

	// Handle withdraw route
	if strings.HasPrefix(restLower, "withdraw route") {
		return convertWithdraw(peerIP, rest[14:])
	}

	// Handle announce/withdraw for other families
	if strings.HasPrefix(restLower, "announce") {
		return convertAnnounceFamily(peerIP, rest[8:])
	}

	if strings.HasPrefix(restLower, "withdraw") {
		return convertWithdrawFamily(peerIP, rest[8:])
	}

	// Unknown command - pass through with peer prefix change
	return fmt.Sprintf("peer %s %s", peerIP, rest)
}

func convertAnnounce(peerIP, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		return fmt.Sprintf("peer %s update text nlri ipv4/unicast add", peerIP)
	}

	prefix := parts[0]
	attrs := parts[1:]

	// Parse attributes
	var cmdParts []string
	cmdParts = append(cmdParts, fmt.Sprintf("peer %s update text", peerIP))

	i := 0
	for i < len(attrs) {
		key := strings.ToLower(attrs[i])
		switch key {
		case "next-hop":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("nhop set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "origin":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("origin set %s", strings.ToLower(attrs[i+1])))
				i += 2
			} else {
				i++
			}
		case "as-path":
			if i+1 < len(attrs) {
				asp := attrs[i+1]
				i += 2
				if strings.HasPrefix(asp, "[") {
					// Collect until ]
					aspParts := []string{asp}
					for i < len(attrs) && !strings.Contains(aspParts[len(aspParts)-1], "]") {
						aspParts = append(aspParts, attrs[i])
						i++
					}
					asp = strings.Join(aspParts, " ")
				}
				asp = strings.Trim(asp, "[]")
				asp = strings.TrimSpace(asp)
				if asp != "" {
					cmdParts = append(cmdParts, fmt.Sprintf("as-path set %s", asp))
				}
			} else {
				i++
			}
		case "med":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("med set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "local-preference":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("local-preference set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "community":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("community add %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "large-community":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("large-community add %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		default: // unrecognized attribute keyword, skip
			i++
		}
	}

	// Determine family from prefix
	family := "ipv4/unicast"
	if strings.Contains(prefix, ":") {
		family = "ipv6/unicast"
	}
	cmdParts = append(cmdParts, fmt.Sprintf("nlri %s add %s", family, prefix))

	return strings.Join(cmdParts, " ")
}

func convertWithdraw(peerIP, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		return fmt.Sprintf("peer %s update text nlri ipv4/unicast del", peerIP)
	}

	prefix := parts[0]
	family := "ipv4/unicast"
	if strings.Contains(prefix, ":") {
		family = "ipv6/unicast"
	}
	return fmt.Sprintf("peer %s update text nlri %s del %s", peerIP, family, prefix)
}

func convertAnnounceFamily(peerIP, rest string) string {
	rest = strings.TrimSpace(rest)
	familyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+(unicast|multicast|nlri-mpls|flowspec)\s+(.+)$`)
	match := familyRE.FindStringSubmatch(rest)
	if match != nil {
		afi := strings.ToLower(match[1])
		safi := strings.ToLower(match[2])
		routeStr := match[3]
		family := fmt.Sprintf("%s/%s", afi, safi)
		return convertAnnounceWithFamily(peerIP, family, routeStr)
	}

	// Fall back to basic conversion
	return fmt.Sprintf("peer %s announce %s", peerIP, rest)
}

func convertWithdrawFamily(peerIP, rest string) string {
	rest = strings.TrimSpace(rest)
	familyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+(unicast|multicast|nlri-mpls|flowspec)\s+(.+)$`)
	match := familyRE.FindStringSubmatch(rest)
	if match != nil {
		afi := strings.ToLower(match[1])
		safi := strings.ToLower(match[2])
		prefix := strings.Fields(match[3])[0]
		family := fmt.Sprintf("%s/%s", afi, safi)
		return fmt.Sprintf("peer %s update text nlri %s del %s", peerIP, family, prefix)
	}

	return fmt.Sprintf("peer %s withdraw %s", peerIP, rest)
}

func convertAnnounceWithFamily(peerIP, family, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		return fmt.Sprintf("peer %s update text nlri %s add", peerIP, family)
	}

	prefix := parts[0]
	attrs := parts[1:]

	var cmdParts []string
	cmdParts = append(cmdParts, fmt.Sprintf("peer %s update text", peerIP))

	i := 0
	for i < len(attrs) {
		key := strings.ToLower(attrs[i])
		switch key {
		case "next-hop":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("nhop set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "origin":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("origin set %s", strings.ToLower(attrs[i+1])))
				i += 2
			} else {
				i++
			}
		case "label":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("label set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "rd":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("rd set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		default: // unrecognized attribute keyword, skip
			i++
		}
	}

	cmdParts = append(cmdParts, fmt.Sprintf("nlri %s add %s", family, prefix))
	return strings.Join(cmdParts, " ")
}
