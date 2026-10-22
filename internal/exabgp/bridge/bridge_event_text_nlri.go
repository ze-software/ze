// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP TEXT event encoder
// Overview: bridge_event_text.go -- the text line of every event kind
// Related: bridge_event.go -- convertUpdateIPC2, the ExaBGP document this file reads
//
// One NLRI of a text UPDATE line, written as the `extensive()` of the ExaBGP
// class that decodes it. Text.update writes `announced <extensive><attributes>`
// and `withdrawn <extensive>` (src/exabgp/reactor/api/response/text.py), and
// the per-family forms come from src/exabgp/bgp/message/update/nlri/*.py at
// 5.0.1.
//
// The route is read from the ExaBGP JSON document the bridge already writes
// for a JSON helper (convertUpdateIPC2, with the wire overlay), never from ze's
// own event members. That document states each route in ExaBGP's own members
// (`nlri`, `path-information`, `label`, `rd`, a flow's `string`), so the text
// and the JSON line of one UPDATE state the same route, and a member corrected
// for one is corrected for both.

package bridge

import (
	"strings"
)

// textRoutes walks the routes of one ExaBGP UPDATE document in the order ze
// stated them. convertUpdateIPC2 appends one document entry for every ze route,
// in ze's order, under the family and the next hop of its operation, so a
// cursor per (family, next hop) finds the entry of the n-th ze route.
//
// Not safe for concurrent use: one text line owns one walker.
type textRoutes struct {
	announce map[string]map[string][]any
	withdraw map[string][]any
	cursor   map[string]int
}

// newTextRoutes answers the walker over the `update` member of an ExaBGP UPDATE
// document. The two sections hold the Go types convertUpdateIPC2 builds them
// with, not the generic ones a decoded JSON object would hold. A document with
// no update member (an End-of-RIB) has no routes, and every lookup then
// answers false.
func newTextRoutes(message map[string]any) textRoutes {
	update, _ := message[msgTypeUpdate].(map[string]any)
	announce, _ := update["announce"].(map[string]map[string][]any)
	withdraw, _ := update["withdraw"].(map[string][]any)
	return textRoutes{announce: announce, withdraw: withdraw, cursor: map[string]int{}}
}

// next answers the document entry of the next route of family (ExaBGP's
// `ipv4 unicast`) under the operation, and false when the document holds no
// such entry. nextHop is the announce's next hop, or "" for a withdrawal.
func (r textRoutes) next(family, nextHop string, announced bool) (map[string]any, bool) {
	var entries []any
	key := family
	if announced {
		// The key convertUpdateIPC2 files a next-hop-less announce under.
		hopKey := nextHop
		if hopKey == "" {
			hopKey = "no-nexthop"
		}
		entries = r.announce[family][hopKey]
		key = family + " " + hopKey
	}
	if !announced {
		entries = r.withdraw[family]
	}
	index := r.cursor[key]
	if index >= len(entries) {
		return nil, false
	}
	r.cursor[key] = index + 1
	entry, ok := entries[index].(map[string]any)
	return entry, ok
}

// appendExtensive writes one route as extensive() of its ExaBGP class.
//
// family is ExaBGP's `<afi> <safi>`, nextHop the announce's next hop, and ""
// for a withdrawal, whose NLRI ExaBGP holds with NoNextHop.
// The class is chosen by the SAFI, which is how ExaBGP's NLRI registry picks it.
// Only five classes override Family.extensive: INET, Label and IPVPN (a
// prefix), Flow and VPLS. Every other one (mup, mcast-vpn, evpn, rtc, bgp-ls)
// inherits `afi <afi> safi <safi>` (src/exabgp/protocol/family.py), and writes
// no next hop on the line.
func appendExtensive(buf []byte, family string, entry map[string]any, nextHop string, announced bool) []byte {
	afi, safi, _ := strings.Cut(family, " ")
	switch safi {
	case bridgeFlowSAFI, bridgeFlowVPNSAFI:
		// Flow.extensive: 'flow' + rules + rd + next-hop. The document's
		// `string` member IS that extensive(): Flow.json writes
		// `"string": self.extensive()` (nlri/flow.py), and the bridge builds it
		// with the rd and the next hop already in it (appendFlowStringNextHop).
		text, _ := entry["string"].(string)
		return appendOneline(buf, text)
	case bridgeVPLSKeyword:
		return appendVPLSExtensive(buf, entry, nextHop)
	case "unicast", "multicast", "nlri-mpls", "mpls-vpn":
		prefix, _ := entry[bridgeUpdateNLRI].(string)
		return appendPrefixExtensive(buf, prefix, entry, nextHop, announced)
	default:
		// The set of SAFIs is open: every class but the five above inherits
		// Family.extensive (src/exabgp/protocol/family.py), whatever its route
		// type, so a route ze could not decode reads the same:
		//   return f'afi {self.afi} safi {self.safi}'
		buf = append(buf, "afi "...)
		buf = appendOneline(buf, afi)
		buf = append(buf, " safi "...)
		return appendOneline(buf, safi)
	}
}

// appendPrefixExtensive writes INET, Label and IPVPN extensive():
//
//	INET.prefix     = cidr + str(path_info)       ` path-information 1.2.3.4`
//	Label.prefix    = INET.prefix + str(labels)   ` label 100`, ` label [ 1 2 ]`
//	extensive       = prefix + ` next-hop <nh>`
//	IPVPN.extensive = Label.extensive + str(rd)    ` rd 65000:1`
//
// A withdrawal names the route by its prefix, path information and route
// distinguisher, and carries no label: the label of a withdrawn route is the
// RFC 8277 Section 2.4 withdrawal value 0x800000, not the route's.
func appendPrefixExtensive(buf []byte, prefix string, entry map[string]any, nextHop string, announced bool) []byte {
	buf = appendOneline(buf, prefix)
	if pathID, ok := entry[bridgeAttrPathInformation].(string); ok {
		buf = append(buf, " path-information "...)
		buf = appendOneline(buf, pathID)
	}
	if announced {
		buf = appendLabelsText(buf, entry[bridgeAttrLabel])
	}
	if nextHop != "" {
		buf = append(buf, " next-hop "...)
		buf = appendOneline(buf, nextHop)
	}
	if rd, ok := entry[bridgeNLRIRD].(string); ok {
		buf = append(buf, " rd "...)
		buf = appendOneline(buf, rd)
	}
	return buf
}

// appendLabelsText writes Labels.__str__ (nlri/qualifier/labels.py): ` label N`
// for one label, ` label [ N M ]` for a stack, nothing for none. The document
// states each label as `[value]`, ExaBGP's Labels.json pair without the raw
// octets, which the text omits too when they carry no flag beyond the
// bottom-of-stack bit.
func appendLabelsText(buf []byte, labels any) []byte {
	stack, _ := labels.([]any)
	if len(stack) == 0 {
		return buf
	}
	buf = append(buf, " label "...)
	if len(stack) > 1 {
		buf = append(buf, "[ "...)
	}
	for index, entry := range stack {
		if index > 0 {
			buf = append(buf, ' ')
		}
		value := entry
		if pair, isPair := entry.([]any); isPair && len(pair) > 0 {
			value = pair[0]
		}
		buf = appendJSONNumber(buf, value)
	}
	if len(stack) > 1 {
		buf = append(buf, " ]"...)
	}
	return buf
}

// appendVPLSExtensive writes VPLS.extensive (nlri/vpls.py):
//
//	'vpls{} endpoint {} base {} offset {} size {} {}'.format(rd, endpoint,
//	    base, offset, size, '' if nexthop is None else 'next-hop {}')
//
// str(rd) carries its own leading space (` rd 65001:100`). The space before
// the next hop is written even when there is none, as the format string does.
func appendVPLSExtensive(buf []byte, entry map[string]any, nextHop string) []byte {
	buf = append(buf, bridgeVPLSKeyword...)
	if rd, ok := entry[bridgeNLRIRD].(string); ok {
		buf = append(buf, " rd "...)
		buf = appendOneline(buf, rd)
	}
	buf = append(buf, " endpoint "...)
	buf = appendJSONNumber(buf, entry["endpoint"])
	buf = append(buf, " base "...)
	buf = appendJSONNumber(buf, entry["base"])
	buf = append(buf, " offset "...)
	buf = appendJSONNumber(buf, entry["offset"])
	buf = append(buf, " size "...)
	buf = appendJSONNumber(buf, entry[exabgpVPLSSize])
	buf = append(buf, ' ')
	if nextHop != "" {
		buf = append(buf, "next-hop "...)
		buf = appendOneline(buf, nextHop)
	}
	return buf
}
