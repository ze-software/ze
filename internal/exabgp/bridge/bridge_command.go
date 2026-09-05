// Design: docs/architecture/core-design.md — ExaBGP text command to ZeBGP translation
// Overview: bridge.go — startup protocol, bridge runtime
// Related: bridge_event.go — ZeBGP to ExaBGP JSON event translation
// Related: bridge_muxconn.go — MuxConn wire format parsing for post-startup I/O

package bridge

import (
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	bridgeAttrNextHop = "next-hop"
	bridgeAttrOrigin  = "origin"
	bridgeFlowSAFI    = "flow"
	bridgeFlowVPNSAFI = "flow-vpn"

	// bridgeEveryPeer is the ze peer selector for every configured session. An
	// ExaBGP line that names no neighbor goes to every neighbor, so the bridge
	// translates it with this selector.
	bridgeEveryPeer = "*"
)

var (
	// bridgeNeighborRE matches an ExaBGP line that names one neighbor. It
	// captures the address and the command that follows it.
	bridgeNeighborRE = regexp.MustCompile(`(?i)^neighbor\s+(\S+)\s+(.+)$`)

	// bridgeSRPolicyRE matches an SR-Policy route, which states the AFI and
	// then the policy fields in place of a prefix.
	bridgeSRPolicyRE = regexp.MustCompile(`(?i)^(ipv[46])\s+sr-policy\s+(.+)$`)

	// bridgeFamilyRE matches a route that states its family as an AFI and a
	// SAFI. It captures both and the route that follows them.
	bridgeFamilyRE = regexp.MustCompile(`(?i)^(ipv[46])\s+(unicast|multicast|nlri-mpls|flow|flowspec|flow-vpn|flowspec-vpn)\s+(.+)$`)
)

// ExabgpToZebgpCommand converts an ExaBGP text command to ZeBGP format.
//
// ExaBGP: neighbor <ip> announce route <prefix> next-hop <nh> [origin <o>] ...
// ZeBGP:  peer <ip> update text nhop <nh> origin <o> nlri ipv4/unicast add <prefix>.
//
// A line that names no neighbor names no destination, and ExaBGP sends such a
// line to every neighbor. The bridge translates it the same way, with the
// wildcard selector: `announce route <prefix>` becomes
// `peer * update text nlri ipv4/unicast add <prefix>`.
//
// A line the bridge has no form for keeps its words. It passes through to ze's
// CLI, where ze declares its own announce and withdraw spellings.
func ExabgpToZebgpCommand(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}

	match := bridgeNeighborRE.FindStringSubmatch(line)
	if match == nil {
		command, translated := convertRoute(bridgeEveryPeer, line)
		if !translated {
			return line
		}
		return command
	}

	selector := match[1]
	rest := strings.TrimSpace(match[2])

	command, translated := convertRoute(selector, rest)
	if !translated {
		// The line names one neighbor, so it keeps that destination under ze's
		// peer keyword in place of ExaBGP's neighbor one.
		var tb textbuf.Buffer
		return tb.Str("peer ").Str(selector).Byte(' ').Str(rest).String()
	}
	return command
}

// convertRoute translates one ExaBGP announce or withdraw into the ze command
// that sends it to the peers the selector names. It reports false when the line
// names no form the bridge translates, and it writes nothing then.
//
// The caller decides what an untranslated line becomes, because the answer
// differs between a line that names a neighbor and a line that does not.
func convertRoute(selector, rest string) (string, bool) {
	const (
		announceRoute = "announce route"
		withdrawRoute = "withdraw route"
		announceVerb  = "announce"
		withdrawVerb  = "withdraw"
	)

	restLower := strings.ToLower(rest)

	if strings.HasPrefix(restLower, announceRoute) {
		return convertAnnounce(selector, rest[len(announceRoute):]), true
	}
	if strings.HasPrefix(restLower, withdrawRoute) {
		return convertWithdraw(selector, rest[len(withdrawRoute):]), true
	}
	if strings.HasPrefix(restLower, announceVerb) {
		return convertAnnounceFamily(selector, rest[len(announceVerb):])
	}
	if strings.HasPrefix(restLower, withdrawVerb) {
		return convertWithdrawFamily(selector, rest[len(withdrawVerb):])
	}
	return "", false
}

func convertAnnounce(selector, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		var tb textbuf.Buffer
		return tb.Str("peer ").Str(selector).Str(" update text nlri ipv4/unicast add").String()
	}

	prefix := parts[0]
	attrs := parts[1:]

	// Parse attributes
	cmdParts := make([]string, 1, len(attrs)+2)
	cmdParts[0] = "peer " + selector + " update text"

	i := 0
	for i < len(attrs) {
		key := strings.ToLower(attrs[i])
		switch key {
		case bridgeAttrNextHop:
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, "nhop "+attrs[i+1])
				i += 2
			} else {
				i++
			}
		case bridgeAttrOrigin:
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
	fam := defaultFamily
	if strings.Contains(prefix, ":") {
		fam = "ipv6/unicast"
	}
	cmdParts = append(cmdParts, "nlri "+fam+" add "+prefix)

	return textbuf.Join(cmdParts, " ")
}

func convertWithdraw(selector, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	var tb textbuf.Buffer
	if len(parts) == 0 {
		return tb.Str("peer ").Str(selector).Str(" update text nlri ipv4/unicast del").String()
	}

	prefix := parts[0]
	fam := defaultFamily
	if strings.Contains(prefix, ":") {
		fam = "ipv6/unicast"
	}
	return tb.Str("peer ").Str(selector).Str(" update text nlri ").Str(fam).Str(" del ").Str(prefix).String()
}

// convertAnnounceFamily translates an ExaBGP announce that states its family,
// which is every announce apart from a plain `announce route`. It reports false
// when the text after the verb states no family the bridge reads.
func convertAnnounceFamily(selector, rest string) (string, bool) {
	rest = strings.TrimSpace(rest)

	if match := bridgeSRPolicyRE.FindStringSubmatch(rest); match != nil {
		afi := strings.ToLower(match[1])
		return convertAnnounceSRPolicy(selector, afi, match[2]), true
	}

	match := bridgeFamilyRE.FindStringSubmatch(rest)
	if match == nil {
		return "", false
	}

	afi := strings.ToLower(match[1])
	safi := canonicalExabgpSAFI(strings.ToLower(match[2]))
	routeStr := match[3]

	var tb textbuf.Buffer
	fam := tb.Str(afi).Byte('/').Str(safi).String()
	if safi == bridgeFlowSAFI || safi == bridgeFlowVPNSAFI {
		return convertAnnounceFlowSpec(selector, fam, routeStr), true
	}
	return convertAnnounceWithFamily(selector, fam, routeStr), true
}

// convertWithdrawFamily translates an ExaBGP withdraw that states its family,
// which is every withdraw apart from a plain `withdraw route`. It reports false
// when the text after the verb states no family the bridge reads.
func convertWithdrawFamily(selector, rest string) (string, bool) {
	rest = strings.TrimSpace(rest)

	if match := bridgeSRPolicyRE.FindStringSubmatch(rest); match != nil {
		afi := strings.ToLower(match[1])
		return convertWithdrawSRPolicy(selector, afi, match[2]), true
	}

	match := bridgeFamilyRE.FindStringSubmatch(rest)
	if match == nil {
		return "", false
	}

	afi := strings.ToLower(match[1])
	safi := canonicalExabgpSAFI(strings.ToLower(match[2]))
	routeStr := match[3]

	var tb textbuf.Buffer
	fam := tb.Str(afi).Byte('/').Str(safi).String()
	if safi == bridgeFlowSAFI || safi == bridgeFlowVPNSAFI {
		return convertWithdrawFlowSpec(selector, fam, routeStr), true
	}

	// The regexp reads the route out of a line with no trailing space, so the
	// capture ends on a non-space byte and always holds one field or more.
	prefix := strings.Fields(routeStr)[0]
	return tb.Reset().Str("peer ").Str(selector).Str(" update text nlri ").Str(fam).Str(" del ").Str(prefix).String(), true
}

// convertAnnounceSRPolicy translates ExaBGP SR-Policy announce to Ze's update text format.
//
// ExaBGP: announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4 preference 100 ...
// Ze:     peer <ip> update text nhop 1.2.3.4 nlri ipv4/sr-policy add distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 ...
//
// Extracts next-hop and the three NLRI fields (distinguisher, color, endpoint),
// then appends all remaining tunnel-encap tokens verbatim.
func convertAnnounceSRPolicy(selector, afi, rest string) string {
	rest = strings.TrimSpace(rest)
	parts := strings.Fields(rest)

	var nhop, distinguisher, color, endpoint string
	var extra []string
	for i := 0; i < len(parts); i++ {
		key := strings.ToLower(parts[i])
		switch key {
		case bridgeAttrNextHop, "distinguisher", "color", "endpoint":
			if i+1 >= len(parts) {
				break
			}
			switch key {
			case bridgeAttrNextHop:
				nhop = parts[i+1]
			case "distinguisher":
				distinguisher = parts[i+1]
			case "color":
				color = parts[i+1]
			case "endpoint":
				endpoint = parts[i+1]
			}
			i++
		default:
			extra = append(extra, parts[i])
		}
	}

	var tb textbuf.Buffer
	tb.Str("peer ").Str(selector).Str(" update text")
	if nhop != "" {
		tb.Str(" nhop ").Str(nhop)
	}
	tb.Str(" nlri ").Str(afi).Str("/sr-policy add")
	tb.Str(" distinguisher ").Str(distinguisher)
	tb.Str(" color ").Str(color)
	tb.Str(" endpoint ").Str(endpoint)
	for _, tok := range extra {
		tb.Str(" ").Str(tok)
	}
	return tb.String()
}

// convertWithdrawSRPolicy translates ExaBGP SR-Policy withdraw to Ze's update text format.
func convertWithdrawSRPolicy(selector, afi, rest string) string {
	rest = strings.TrimSpace(rest)

	var tb textbuf.Buffer
	tb.Str("peer ").Str(selector).Str(" update text nlri ").Str(afi).Str("/sr-policy del ").Str(rest)
	return tb.String()
}

func canonicalExabgpSAFI(safi string) string {
	switch safi {
	case "flowspec":
		return "flow"
	case "flowspec-vpn":
		return "flow-vpn"
	default:
		return safi
	}
}

func convertAnnounceFlowSpec(selector, family, routeStr string) string {
	fam, attrs, rd, nlri := parseFlowSpecBridgeRoute(family, routeStr)
	cmdParts := make([]string, 1, len(attrs)+2)
	cmdParts[0] = "peer " + selector + " update text"
	cmdParts = append(cmdParts, attrs...)

	var nlriPart textbuf.Buffer
	nlriPart.Str("nlri ").Str(fam).Str(" add")
	if rd != "" {
		nlriPart.Str(" rd ").Str(rd)
	}
	if nlri != "" {
		nlriPart.Byte(' ').Str(nlri)
	}
	cmdParts = append(cmdParts, nlriPart.String())
	return textbuf.Join(cmdParts, " ")
}

func convertWithdrawFlowSpec(selector, family, routeStr string) string {
	fam, _, rd, nlri := parseFlowSpecBridgeRoute(family, routeStr)
	var tb textbuf.Buffer
	tb.Str("peer ").Str(selector).Str(" update text nlri ").Str(fam).Str(" del")
	if rd != "" {
		tb.Str(" rd ").Str(rd)
	}
	if nlri != "" {
		tb.Byte(' ').Str(nlri)
	}
	return tb.String()
}

func parseFlowSpecBridgeRoute(family, routeStr string) (string, []string, string, string) {
	parts := strings.Fields(strings.TrimSpace(routeStr))
	attrs := make([]string, 0, 4)
	nlri := make([]string, 0, len(parts))
	rd := ""
	fam := family
	currentComponent := ""
	inList := false

	for i := 0; i < len(parts); {
		key := strings.ToLower(parts[i])
		switch key {
		case bridgeAttrNextHop:
			if i+1 < len(parts) {
				attrs = append(attrs, "nhop "+parts[i+1])
				i += 2
			} else {
				i++
			}
		case bridgeAttrOrigin:
			if i+1 < len(parts) {
				attrs = append(attrs, "origin "+strings.ToLower(parts[i+1]))
				i += 2
			} else {
				i++
			}
		case "community", "large-community", "extended-community":
			if i+1 < len(parts) {
				value, next := collectBridgeAttrValue(parts, i+1)
				if key == "extended-community" {
					value = normalizeFlowSpecExtCommunityValue(value)
				}
				attrs = append(attrs, key+" "+value)
				i = next
			} else {
				i++
			}
		case "rd":
			if i+1 < len(parts) {
				rd = parts[i+1]
				fam = flowSpecVPNFamily(fam)
				i += 2
			} else {
				i++
			}
		default:
			token := normalizeFlowSpecComponentToken(fam, parts[i])
			if isFlowSpecComponentKeyword(token) {
				currentComponent = strings.ToLower(token)
				nlri = append(nlri, token)
			} else {
				nlri = append(nlri, normalizeFlowSpecComponentValue(currentComponent, token))
				switch {
				case strings.Contains(parts[i], "[") && !strings.Contains(parts[i], "]"):
					inList = true
				case strings.Contains(parts[i], "]"):
					inList = false
					currentComponent = ""
				case !inList:
					currentComponent = ""
				}
			}
			i++
		}
	}

	return fam, attrs, rd, textbuf.Join(nlri, " ")
}

// isFlowSpecComponentKeyword answers whether a token names a match component.
//
// Its one caller tests the token AFTER normalizeFlowSpecComponentToken has run,
// so the bare aliases never reach it and the qualified spellings always do.
func isFlowSpecComponentKeyword(token string) bool {
	switch strings.ToLower(token) {
	case "destination-ipv4", "destination-ipv6", "source-ipv4", "source-ipv6",
		"protocol", "next-header", "port", "destination-port", "source-port",
		"icmp-type", "icmp-code", "tcp-flags", "packet-length", "dscp",
		"fragment", "traffic-class", "flow-label":
		return true
	default:
		return false
	}
}

func normalizeFlowSpecComponentValue(component, token string) string {
	switch component {
	case "protocol", "next-header":
		return strings.TrimPrefix(token, "=")
	default:
		return token
	}
}

func flowSpecVPNFamily(family string) string {
	if strings.HasPrefix(family, "ipv6/") {
		return "ipv6/flow-vpn"
	}
	return "ipv4/flow-vpn"
}

func collectBridgeAttrValue(parts []string, start int) (string, int) {
	if start >= len(parts) {
		return "", start
	}
	if strings.Contains(parts[start], "[") && !strings.Contains(parts[start], "]") {
		valueParts := []string{parts[start]}
		i := start + 1
		for i < len(parts) {
			valueParts = append(valueParts, parts[i])
			if strings.Contains(parts[i], "]") {
				return textbuf.Join(valueParts, " "), i + 1
			}
			i++
		}
		return textbuf.Join(valueParts, " "), i
	}
	return parts[start], start + 1
}

func normalizeFlowSpecExtCommunityValue(value string) string {
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return value
		}
		fields := strings.Fields(inner)
		for i := range fields {
			fields[i] = normalizeFlowSpecExtCommunityToken(fields[i])
		}
		return "[" + textbuf.Join(fields, " ") + "]"
	}
	return normalizeFlowSpecExtCommunityToken(value)
}

func normalizeFlowSpecExtCommunityToken(value string) string {
	if rate, ok := strings.CutPrefix(value, "rate-limit-packets:"); ok {
		var tb textbuf.Buffer
		return tb.Str("rate-limit:").Str(rate).Str(":packets").String()
	}
	if strings.HasSuffix(value, ":bytes") && strings.HasPrefix(value, "rate-limit:") {
		return strings.TrimSuffix(value, ":bytes")
	}
	return value
}

// normalizeFlowSpecComponentToken rewrites ExaBGP's bare prefix keywords into
// the family-qualified ones ze speaks.
//
// ExaBGP accepts `source` and `destination` as aliases of the qualified
// spellings on input, and emits only the qualified ones
// (src/exabgp/configuration/announce/flow.py and
// src/exabgp/bgp/message/update/nlri/flow.py, both on 5.0 and main). An operator
// config written by hand may still use the alias, so the bare word arrives here
// and the route's family says which spelling it meant.
func normalizeFlowSpecComponentToken(family, token string) string {
	v6 := strings.HasPrefix(family, "ipv6/")
	switch strings.ToLower(token) {
	case "source":
		if v6 {
			return "source-ipv6"
		}
		return "source-ipv4"
	case "destination":
		if v6 {
			return "destination-ipv6"
		}
		return "destination-ipv4"
	}
	return token
}

func convertAnnounceWithFamily(selector, family, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		var tb textbuf.Buffer
		return tb.Str("peer ").Str(selector).Str(" update text nlri ").Str(family).Str(" add").String()
	}

	prefix := parts[0]
	attrs := parts[1:]

	var cmdParts []string
	cmdParts = append(cmdParts, "peer "+selector+" update text")

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
