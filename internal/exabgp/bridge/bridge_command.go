// Design: docs/architecture/core-design.md — ExaBGP text command to ZeBGP translation
// Overview: bridge.go — startup protocol, bridge runtime
// Related: bridge_event.go — ZeBGP to ExaBGP JSON event translation
// Related: bridge_muxconn.go — MuxConn wire format parsing for post-startup I/O

package bridge

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
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
	//
	// The alternation is BUILT from bridgeSAFI rather than written beside it. It
	// was a second copy of that vocabulary until 2026-09-05, and the two had
	// drifted: mcast-vpn was in neither, so every one of api-mvpn's fourteen
	// frames was refused by a translator that could name the family perfectly
	// well once it reached the mapping.
	bridgeFamilyRE = regexp.MustCompile(`(?i)^(ipv[46])\s+(` + bridgeSAFIAlternation() + `)\s+(.+)$`)
)

// bridgeSAFI maps the SAFI an ExaBGP script writes to the one ze names. Most
// are the same word; the rows that differ are the whole reason a mapping exists
// rather than a passthrough.
var bridgeSAFI = map[string]string{
	"unicast":      "unicast",
	"multicast":    "multicast",
	"nlri-mpls":    "nlri-mpls",
	"flow":         "flow",
	"flowspec":     "flow",
	"flow-vpn":     "flow-vpn",
	"flowspec-vpn": "flow-vpn",
	"mcast-vpn":    "mvpn",
	"mup":          "mup",
}

// bridgeSAFIAlternation renders bridgeSAFI's keys as a regexp alternation,
// longest first so a reader never has to reason about which branch wins.
func bridgeSAFIAlternation() string {
	names := make([]string, 0, len(bridgeSAFI))
	for name := range bridgeSAFI {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) > len(names[j])
		}
		return names[i] < names[j]
	})
	return strings.Join(names, "|")
}

// Translation is what the bridge makes of one ExaBGP line: the ze command to
// dispatch, and the selector whose forward pool the caller flushes once that
// command is acknowledged.
//
// The selector travels WITH the command because the builder that wrote the
// command already knew it. Reading it back out of the finished string stated one
// fact twice, and the two statements disagreed in silence: the route test
// answered on a substring that survives any change of leading token, while the
// selector test answered on that token, so a moved grammar left every flush
// skipped with no error and no log line (ai/rules/principles.md).
type Translation struct {
	// Command is the line to hand to ze's dispatcher.
	Command string
	// Selector names the peers Command addresses. It is set whenever Route is,
	// and no caller reads it otherwise.
	Selector string
	// Route says Command puts an UPDATE on a wire, so a per-peer flush is owed
	// after the dispatch is acknowledged.
	Route bool
	// Local is set when the bridge answers the line itself. Command is empty
	// then, and a caller MUST read this before it reads Nothing: a local action
	// carries no command and is not an empty line.
	Local LocalAction
}

// LocalAction is a command the bridge performs ITSELF rather than dispatching.
//
// The ExaBGP API's ack control is the whole set. A script turns acks off, sends
// commands it does not want answered, and turns them back on, and the three
// words differ in WHEN the silence starts: `disable-ack` is acked and silences
// what follows, `silence-ack` is not acked at all, and `enable-ack` is acked and
// resumes.
type LocalAction uint8

const (
	// LocalNone is the ordinary line: it carries a command to dispatch.
	LocalNone LocalAction = iota
	// LocalAckEnable resumes acks, and this command is acked.
	LocalAckEnable
	// LocalAckDisableAfter stops acks after this command, which is acked.
	LocalAckDisableAfter
	// LocalAckDisableNow stops acks immediately, so this command is not acked.
	LocalAckDisableNow
)

// Nothing reports a line that carries no command: a blank line, or a comment.
// It is an ANSWER rather than a failure, and it is named so that no caller has
// to read an empty Command as one.
func (t Translation) Nothing() bool { return t.Command == "" }

// ErrLineNotTranslated is what TranslateLine answers for a line the bridge does
// not read. The bridge refuses such a line rather than forwarding it, because a
// forwarded line dies at ze's dispatcher as an unknown command with no mention
// of the bridge that sent it.
var ErrLineNotTranslated = errors.New("the ExaBGP bridge does not translate this line")

// bridgePassthrough is the ONE ExaBGP line that still reaches ze's dispatcher
// unchanged. `help` is the bridge's own word rather than a route, ze declares it
// as ze-bgp:help, and it is the single member bridgeSurface keeps
// (internal/component/command/grammar/checker.go).
const bridgePassthrough = "help"

// TranslateLine converts one ExaBGP text command into the ze command that sends
// it, and names the peers that command addresses.
//
// ExaBGP: neighbor <ip> announce route <prefix> next-hop <nh> [origin <o>] ...
// ZeBGP:  send bgp <ip> update text nhop <nh> origin <o> nlri ipv4/unicast add <prefix>.
//
// A line that names no neighbor names no destination, and ExaBGP sends such a
// line to every neighbor. The bridge reads it the same way, with the wildcard
// selector: `announce route <prefix>` becomes
// `send bgp * update text nlri ipv4/unicast add <prefix>`.
//
// Every other line is REFUSED by name. The passthrough that forwarded it used to
// carry ze's own announce and withdraw spellings, which have moved under
// `send bgp <selector>` and are translator output now, so what it carried is
// gone and what remains of it is an untyped path from a script's stdout to ze's
// dispatcher.
func TranslateLine(line string) (Translation, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return Translation{}, nil
	}

	selector := bridgeEveryPeer
	rest := line
	if match := bridgeNeighborRE.FindStringSubmatch(line); match != nil {
		selector = match[1]
		rest = strings.TrimSpace(match[2])
	}

	if translation, ok := convertControl(selector, rest); ok {
		return translation, nil
	}
	if command, translated := convertRoute(selector, rest); translated {
		return Translation{Command: command, Selector: selector, Route: true}, nil
	}
	if strings.EqualFold(rest, bridgePassthrough) {
		return Translation{Command: bridgePassthrough}, nil
	}
	return Translation{}, fmt.Errorf("%w: %q", ErrLineNotTranslated, line)
}

// convertControl translates the ExaBGP API commands that are not routes.
//
// An ExaBGP script drives more than announcements: it clears and flushes the
// adj-RIB, turns the command acknowledgement on and off, drives a watchdog
// group, and asks the daemon to stop. The bridge answered none of these until
// 2026-09-05, so every script that used one stopped there, and 29 of the 40
// ported qa/api tests ended after their first frame.
//
// Each mapping is to a command ze declares, checked against the live registry
// rather than assumed:
//
//	clear adj-rib in|out        -> clear bgp rib in|out       (ze-rib-api:clear-in/out)
//	flush adj-rib [in|out]      -> request peer <sel> flush   (ze-bgp:peer-flush)
//	announce watchdog <name>    -> request bgp watchdog announce <name>
//	withdraw watchdog <name>    -> request bgp watchdog withdraw <name>
//	shutdown, request shutdown  -> request shutdown           (ze-system:daemon-shutdown)
//	enable-ack, disable-ack, silence-ack -> answered by the bridge itself
//
// The watchdog forms are read BEFORE convertRoute although they start with the
// announce and withdraw verbs. convertRoute would refuse them anyway, because
// `watchdog <name>` states no family, but a reader should not have to know that
// to see which branch takes them.
func convertControl(selector, rest string) (Translation, bool) {
	fields := strings.Fields(strings.ToLower(rest))
	if len(fields) == 0 {
		return Translation{}, false
	}
	var tb textbuf.Buffer
	switch {
	case len(fields) == 1 && fields[0] == "enable-ack":
		return Translation{Local: LocalAckEnable}, true
	case len(fields) == 1 && fields[0] == "disable-ack":
		return Translation{Local: LocalAckDisableAfter}, true
	case len(fields) == 1 && fields[0] == "silence-ack":
		return Translation{Local: LocalAckDisableNow}, true
	case len(fields) == 1 && fields[0] == "shutdown",
		len(fields) == 2 && fields[0] == "request" && fields[1] == "shutdown":
		return Translation{Command: "request shutdown", Selector: selector}, true
	case len(fields) == 3 && fields[0] == "clear" && fields[1] == "adj-rib" &&
		(fields[2] == "in" || fields[2] == "out"):
		return Translation{
			Command:  tb.Str("clear bgp rib ").Str(fields[2]).String(),
			Selector: selector,
		}, true
	case fields[0] == "flush" && len(fields) >= 2 && fields[1] == "adj-rib":
		// ze drains a peer's forward pool rather than a named direction, so the
		// in/out word an ExaBGP script may add has no counterpart and is not
		// invented into one.
		return Translation{
			Command:  tb.Str("request peer ").Str(selector).Str(" flush").String(),
			Selector: selector,
		}, true
	case len(fields) == 3 && fields[1] == "watchdog" &&
		(fields[0] == "announce" || fields[0] == "withdraw"):
		name := strings.Fields(rest)[2]
		return Translation{
			Command:  tb.Str("request bgp watchdog ").Str(fields[0]).Byte(' ').Str(name).String(),
			Selector: selector,
		}, true
	}
	return Translation{}, false
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
		return tb.Str("send bgp ").Str(selector).Str(" update text nlri ipv4/unicast add").String()
	}

	prefix := parts[0]
	attrs := parts[1:]

	// Parse attributes
	cmdParts := make([]string, 1, len(attrs)+2)
	cmdParts[0] = "send bgp " + selector + " update text"

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
		return tb.Str("send bgp ").Str(selector).Str(" update text nlri ipv4/unicast del").String()
	}

	prefix := parts[0]
	fam := defaultFamily
	if strings.Contains(prefix, ":") {
		fam = "ipv6/unicast"
	}
	return tb.Str("send bgp ").Str(selector).Str(" update text nlri ").Str(fam).Str(" del ").Str(prefix).String()
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
	return tb.Reset().Str("send bgp ").Str(selector).Str(" update text nlri ").Str(fam).Str(" del ").Str(prefix).String(), true
}

// convertAnnounceSRPolicy translates ExaBGP SR-Policy announce to Ze's update text format.
//
// ExaBGP: announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4 preference 100 ...
// Ze:     send bgp <ip> update text nhop 1.2.3.4 nlri ipv4/sr-policy add distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 ...
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
	tb.Str("send bgp ").Str(selector).Str(" update text")
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
	tb.Str("send bgp ").Str(selector).Str(" update text nlri ").Str(afi).Str("/sr-policy del ").Str(rest)
	return tb.String()
}

// canonicalExabgpSAFI answers the SAFI ze names for the one an ExaBGP script
// wrote. An unmapped word is returned unchanged, because the regexp that
// selected it was built from the same map and cannot offer one.
func canonicalExabgpSAFI(safi string) string {
	if canonical, ok := bridgeSAFI[safi]; ok {
		return canonical
	}
	return safi
}

func convertAnnounceFlowSpec(selector, family, routeStr string) string {
	fam, attrs, rd, nlri := parseFlowSpecBridgeRoute(family, routeStr)
	cmdParts := make([]string, 1, len(attrs)+2)
	cmdParts[0] = "send bgp " + selector + " update text"
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
	tb.Str("send bgp ").Str(selector).Str(" update text nlri ").Str(fam).Str(" del")
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
		return tb.Str("send bgp ").Str(selector).Str(" update text nlri ").Str(family).Str(" add").String()
	}

	prefix := parts[0]
	attrs := parts[1:]

	var cmdParts []string
	cmdParts = append(cmdParts, "send bgp "+selector+" update text")

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
