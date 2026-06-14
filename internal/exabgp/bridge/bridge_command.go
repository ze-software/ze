// Design: docs/architecture/core-design.md — ExaBGP text command to ZeBGP translation
// Overview: bridge.go — startup protocol, bridge runtime
// Related: bridge_event.go — ZeBGP to ExaBGP JSON event translation
// Related: bridge_muxconn.go — MuxConn wire format parsing for post-startup I/O

package bridge

import (
	"regexp"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// ExabgpToZebgpCommand converts an ExaBGP text command to ZeBGP format.
//
// ExaBGP: neighbor <ip> announce route <prefix> next-hop <nh> [origin <o>] ...
// ZeBGP:  peer <ip> update text nhop <nh> origin <o> nlri ipv4/unicast add <prefix>.
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
	var tb textbuf.Buffer
	return tb.Str("peer ").Str(peerIP).Byte(' ').Str(rest).String()
}

func convertAnnounce(peerIP, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		var tb textbuf.Buffer
		return tb.Str("peer ").Str(peerIP).Str(" update text nlri ipv4/unicast add").String()
	}

	prefix := parts[0]
	attrs := parts[1:]

	// Parse attributes
	var cmdParts []string
	cmdParts = append(cmdParts, "peer "+peerIP+" update text")

	i := 0
	for i < len(attrs) {
		key := strings.ToLower(attrs[i])
		switch key {
		case "next-hop":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "nhop "+attrs[i+1])
				i += 2
			} else {
				i++
			}
		case "origin":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "origin "+strings.ToLower(attrs[i+1]))
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
					asp = textbuf.Join(aspParts, " ")
				}
				asp = strings.Trim(asp, "[]")
				asp = strings.TrimSpace(asp)
				if asp != "" {
					cmdParts = append(cmdParts, "as-path "+asp)
				}
			} else {
				i++
			}
		case "med":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "med "+attrs[i+1])
				i += 2
			} else {
				i++
			}
		case "local-preference":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "local-preference "+attrs[i+1])
				i += 2
			} else {
				i++
			}
		case "community":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "community "+attrs[i+1])
				i += 2
			} else {
				i++
			}
		case "large-community":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "large-community "+attrs[i+1])
				i += 2
			} else {
				i++
			}
		default: // unrecognized attribute keyword, skip
			i++
		}
	}

	// Determine family from prefix
	fam := "ipv4/unicast"
	if strings.Contains(prefix, ":") {
		fam = "ipv6/unicast"
	}
	cmdParts = append(cmdParts, "nlri "+fam+" add "+prefix)

	return textbuf.Join(cmdParts, " ")
}

func convertWithdraw(peerIP, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	var tb textbuf.Buffer
	if len(parts) == 0 {
		return tb.Str("peer ").Str(peerIP).Str(" update text nlri ipv4/unicast del").String()
	}

	prefix := parts[0]
	fam := "ipv4/unicast"
	if strings.Contains(prefix, ":") {
		fam = "ipv6/unicast"
	}
	return tb.Str("peer ").Str(peerIP).Str(" update text nlri ").Str(fam).Str(" del ").Str(prefix).String()
}

func convertAnnounceFamily(peerIP, rest string) string {
	rest = strings.TrimSpace(rest)

	// SR-Policy has a different syntax: ipv4/ipv6 sr-policy distinguisher ...
	srPolicyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+sr-policy\s+(.+)$`)
	if match := srPolicyRE.FindStringSubmatch(rest); match != nil {
		afi := strings.ToLower(match[1])
		return convertAnnounceSRPolicy(peerIP, afi, match[2])
	}

	familyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+(unicast|multicast|nlri-mpls|flowspec)\s+(.+)$`)
	match := familyRE.FindStringSubmatch(rest)
	if match != nil {
		afi := strings.ToLower(match[1])
		safi := strings.ToLower(match[2])
		routeStr := match[3]
		var tb textbuf.Buffer
		fam := tb.Str(afi).Byte('/').Str(safi).String()
		return convertAnnounceWithFamily(peerIP, fam, routeStr)
	}

	var tb textbuf.Buffer
	return tb.Str("peer ").Str(peerIP).Str(" announce ").Str(rest).String()
}

func convertWithdrawFamily(peerIP, rest string) string {
	rest = strings.TrimSpace(rest)

	// SR-Policy withdraw: ipv4/ipv6 sr-policy distinguisher ...
	srPolicyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+sr-policy\s+(.+)$`)
	if match := srPolicyRE.FindStringSubmatch(rest); match != nil {
		afi := strings.ToLower(match[1])
		return convertWithdrawSRPolicy(peerIP, afi, match[2])
	}

	familyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+(unicast|multicast|nlri-mpls|flowspec)\s+(.+)$`)
	match := familyRE.FindStringSubmatch(rest)
	var tb textbuf.Buffer
	if match != nil {
		afi := strings.ToLower(match[1])
		safi := strings.ToLower(match[2])
		prefix := strings.Fields(match[3])[0]
		fam := tb.Str(afi).Byte('/').Str(safi).String()
		return tb.Reset().Str("peer ").Str(peerIP).Str(" update text nlri ").Str(fam).Str(" del ").Str(prefix).String()
	}

	return tb.Str("peer ").Str(peerIP).Str(" withdraw ").Str(rest).String()
}

// convertAnnounceSRPolicy translates ExaBGP SR-Policy announce to Ze's update text format.
//
// ExaBGP: announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4 preference 100 ...
// Ze:     peer <ip> update text nhop 1.2.3.4 nlri ipv4/sr-policy add distinguisher 0 color 100 endpoint 10.0.0.1
//
// Extracts next-hop and the three NLRI fields (distinguisher, color, endpoint).
// Tunnel encapsulation tokens (preference, segment-list, etc.) are not yet supported.
func convertAnnounceSRPolicy(peerIP, afi, rest string) string {
	rest = strings.TrimSpace(rest)
	parts := strings.Fields(rest)

	var nhop, distinguisher, color, endpoint string
	for i := 0; i < len(parts); i++ {
		key := strings.ToLower(parts[i])
		if i+1 >= len(parts) {
			break
		}
		switch key {
		case "next-hop":
			nhop = parts[i+1]
			i++
		case "distinguisher":
			distinguisher = parts[i+1]
			i++
		case "color":
			color = parts[i+1]
			i++
		case "endpoint":
			endpoint = parts[i+1]
			i++
		}
	}

	var tb textbuf.Buffer
	tb.Str("peer ").Str(peerIP).Str(" update text")
	if nhop != "" {
		tb.Str(" nhop ").Str(nhop)
	}
	tb.Str(" nlri ").Str(afi).Str("/sr-policy add")
	tb.Str(" distinguisher ").Str(distinguisher)
	tb.Str(" color ").Str(color)
	tb.Str(" endpoint ").Str(endpoint)
	return tb.String()
}

// convertWithdrawSRPolicy translates ExaBGP SR-Policy withdraw to Ze's update text format.
func convertWithdrawSRPolicy(peerIP, afi, rest string) string {
	rest = strings.TrimSpace(rest)

	var tb textbuf.Buffer
	tb.Str("peer ").Str(peerIP).Str(" update text nlri ").Str(afi).Str("/sr-policy del ").Str(rest)
	return tb.String()
}

func convertAnnounceWithFamily(peerIP, family, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		var tb textbuf.Buffer
		return tb.Str("peer ").Str(peerIP).Str(" update text nlri ").Str(family).Str(" add").String()
	}

	prefix := parts[0]
	attrs := parts[1:]

	var cmdParts []string
	cmdParts = append(cmdParts, "peer "+peerIP+" update text")

	i := 0
	for i < len(attrs) {
		key := strings.ToLower(attrs[i])
		switch key {
		case "next-hop":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "nhop "+attrs[i+1])
				i += 2
			} else {
				i++
			}
		case "origin":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "origin "+strings.ToLower(attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "label":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "label "+attrs[i+1])
				i += 2
			} else {
				i++
			}
		case "rd":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "rd "+attrs[i+1])
				i += 2
			} else {
				i++
			}
		default: // unrecognized attribute keyword, skip
			i++
		}
	}

	cmdParts = append(cmdParts, "nlri "+family+" add "+prefix)
	return textbuf.Join(cmdParts, " ")
}
