// Design: docs/architecture/core-design.md — ZeBGP to ExaBGP JSON event translation
// Overview: bridge.go — startup protocol, bridge runtime
// Related: bridge_command.go — ExaBGP text command translation
// Related: bridge_muxconn.go — MuxConn wire format parsing for post-startup I/O

package bridge

import (
	"maps"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Version is the ExaBGP JSON envelope version. Set to 6.0.0 to match
// ExaBGP main, which is the syntax target for the bridge (RFC 8955
// packet-rate, SR-Policy, unified rate-limit:N:packets spelling).
const Version = "6.0.0"

// Mode/direction string constants.
// Used for both ExaBGP JSON direction field and ADD-PATH CLI mode.
const (
	modeReceive = "receive"
	modeSend    = "send"
	modeBoth    = "both"
)

// Message type constants: the `bgp.message.type` values ze writes
// (docs/architecture/api/json-format.md, "Event Types").
const (
	msgTypeOpen   = "open"
	msgTypeUpdate = "update"
	// msgTypeEOR is ze's kind for an End-of-RIB marker, and it is also the key
	// the marker's family sits under inside an UPDATE's own body
	// (internal/component/bgp/format/text_update.go, AppendEOR and
	// appendEORUpdateJSON write the same word for the same thing).
	msgTypeEOR          = "eor"
	msgTypeState        = "state"
	msgTypeKeepalive    = "keepalive"
	msgTypeNotification = "notification"
	msgTypeRefresh      = "refresh"
	msgTypeNegotiated   = "negotiated"
	msgTypeFSM          = "fsm"
	msgTypeSignal       = "signal"

	// msgTypeSent is ze's own kind for an UPDATE this speaker SENT. Its data
	// sits under `update` like any other UPDATE, and ze's own event reader
	// folds it back to `update` before a consumer sees it
	// (internal/component/bgp/event.go, the EventKindSent branch). The bridge
	// does the same fold, so a script meets one UPDATE kind in two directions
	// rather than two kinds.
	//
	// It was NOT folded until 2026-09-06, and the cost was total: every UPDATE
	// ze sent reached a script as an event named `sent`, which the ExaBGP
	// vocabulary has no word for, so the JSON encoder wrote an envelope with no
	// message in it and the peer's own routes were invisible to the script.
	msgTypeSent = "sent"
)

// Event is one ze BGP event, read off its JSON once.
//
// The bridge fans one event out to every script, and two scripts can declare
// different encoders, so the read happens once for the fleet and each encoder
// renders from this. Peer is also what the fan-out filters on, so the address
// is taken from the producing field rather than looked for again in each
// encoder.
type Event struct {
	// Payload is the `bgp` object: the event stripped of its ze envelope.
	Payload map[string]any
	// Data is the object named by Kind, or Payload itself for a state event,
	// whose value is a string at the `bgp` level rather than a container.
	Data map[string]any
	// Kind is the `message.type`.
	Kind string
	// Direction is ExaBGP's word for the way the message traveled: `receive`
	// or `send`.
	Direction string
	// Peer is the remote address, the one key both encoders and the fan-out
	// filter read.
	Peer string
	// Local is THIS speaker's address for the session, empty when the event
	// carries none. ExaBGP states both halves of a session on every line, so
	// the JSON encoder owes it (src/exabgp/reactor/api/response/json.py,
	// _neighbor).
	Local string
	// PeerASN is the remote AS, as JSON delivered it.
	PeerASN float64
	// LocalASN is THIS speaker's AS, as JSON delivered it.
	LocalASN float64
	// RouterID is THIS speaker's BGP Identifier for the session, empty when the
	// event carries none.
	RouterID string
}

// ReadEvent reads one ze JSON event.
//
// The ze-bgp shape it reads (docs/architecture/api/json-format.md):
//
//	{
//	  "type": "bgp",
//	  "bgp": {
//	    "peer": {"local": {"address": "...", "as": ...}, "remote": {"address": "10.0.0.1", "as": 65001}},
//	    "message": {"id": 1, "direction": "received", "type": "update"},
//	    "update": {"attr": {"origin": "igp"}, "nlri": {"ipv4/unicast": [...]}}
//	  }
//	}
//
// A state event carries its value as a string at the `bgp` level rather than in
// a container of its own:
//
//	{"type": "bgp", "bgp": {"message": {"type": "state"}, "peer": {...}, "state": "up"}}
//
// An event with no `message.type` is named by the key it carries instead, which
// is what the ze CLI's own decode path produces.
func ReadEvent(zebgp map[string]any) Event {
	event := Event{Payload: zebgp, Direction: modeReceive}

	// Strip the ze-bgp JSON wrapper when it is present.
	if rootType, _ := zebgp["type"].(string); rootType == "bgp" {
		if bgp, ok := zebgp["bgp"].(map[string]any); ok {
			event.Payload = bgp
		}
	}

	if message, ok := event.Payload["message"].(map[string]any); ok {
		event.Kind, _ = message["type"].(string)
		if direction, ok := message["direction"].(string); ok {
			switch direction {
			case "received":
				event.Direction = modeReceive
			case "sent":
				event.Direction = modeSend
			}
		}
	}
	if event.Kind == msgTypeSent {
		event.Kind = msgTypeUpdate
		event.Direction = modeSend
	}
	if event.Kind == "" {
		event.Kind = eventKindByKey(event.Payload)
	}

	if peer, ok := event.Payload["peer"].(map[string]any); ok {
		if remote, ok := peer["remote"].(map[string]any); ok {
			event.Peer, _ = remote["address"].(string)
			event.PeerASN, _ = remote["as"].(float64)
		}
		// ze states the near end of the session under `local`
		// (internal/component/bgp/format/text.go, appendPeerJSON). It was read
		// by nothing until 2026-09-19, so every ExaBGP JSON line the bridge
		// wrote named one half of a session ExaBGP states both halves of.
		if local, ok := peer["local"].(map[string]any); ok {
			event.Local, _ = local["address"].(string)
			event.LocalASN, _ = local["as"].(float64)
		}
		event.RouterID, _ = peer["router-id"].(string)
	}

	// A state event carries its value as a string at the `bgp` level, so it has
	// no container of its own to descend into.
	event.Data = event.Payload
	if event.Kind != msgTypeState {
		if nested, ok := event.Payload[event.Kind].(map[string]any); ok {
			event.Data = nested
		}
	}
	return event
}

// eventKindByKey names the event by the key it carries, for a payload whose
// `message.type` is absent.
//
// It answers the EMPTY string for a payload that carries none of them, and both
// encoders read that as an event they cannot render. Answering `update` was the
// silently-wrong value `ai/rules/principles.md` bans: the bridge subscribes to
// every event ze publishes, so it meets envelopes that carry no BGP message at
// all, and each one reached a script as an UPDATE announcing nothing, from a
// peer named by the empty string.
//
// An UPDATE stripped of its metadata is still named, by the `attr` and `nlri`
// keys its body carries, which is the shape ze's own decode path produces.
func eventKindByKey(payload map[string]any) string {
	for _, kind := range []string{msgTypeOpen, msgTypeUpdate, msgTypeState} {
		if _, ok := payload[kind]; ok {
			return kind
		}
	}
	for _, body := range []string{bridgeUpdateAttr, bridgeUpdateNLRI} {
		if _, ok := payload[body]; ok {
			return msgTypeUpdate
		}
	}
	return ""
}

// ZebgpToExabgpJSON reads one ze JSON event and renders it as ExaBGP JSON. It
// is the two steps in one call, for a caller that renders a single event in a
// single format.
func ZebgpToExabgpJSON(zebgp map[string]any) map[string]any {
	return ReadEvent(zebgp).ExabgpJSON()
}

// ExabgpJSON renders the event as one ExaBGP JSON object.
//
// ExaBGP format (nested):
//
//	{
//	  "exabgp": "6.0.0",
//	  "type": "update",
//	  "neighbor": {
//	    "address": {"local": "10.0.0.2", "peer": "10.0.0.1"},
//	    "asn": {"local": 65002, "peer": 65001},
//	    "direction": "receive",
//	    "message": {"update": {...}}
//	  }
//	}
//
// Both halves of `address` and `asn` are written on every line, and neither key
// is omitted when the event carries no value for it. ExaBGP's own encoder
// renders the pair unconditionally (src/exabgp/reactor/api/response/json.py,
// _neighbor), and a script that indexes `address["local"]` must not meet a
// KeyError on the one line whose session was not yet established.
func (e Event) ExabgpJSON() map[string]any {
	result := map[string]any{
		"exabgp": Version,
		"time":   float64(time.Now().Unix()),
		"host":   hostname(),
		"pid":    os.Getpid(),
		"ppid":   os.Getppid(),
		"type":   e.Kind,
	}

	neighbor := map[string]any{
		"address":   map[string]any{"local": e.Local, "peer": e.Peer},
		"asn":       map[string]any{"local": e.LocalASN, "peer": e.PeerASN},
		"direction": e.Direction,
	}
	if e.RouterID != "" {
		neighbor["router-id"] = e.RouterID
	}

	switch e.Kind {
	case msgTypeState:
		// State is a simple string at bgp level (not a container).
		state, _ := e.Payload["state"].(string)
		neighbor["state"] = state

	case msgTypeUpdate:
		neighbor["message"] = updateMessageJSON(e.Data)

	case msgTypeNotification:
		neighbor[msgTypeNotification] = map[string]any{
			"code":    e.Data["code"],
			"subcode": e.Data["subcode"],
			"data":    e.Data["data"],
		}

	case msgTypeNegotiated:
		result[msgTypeNegotiated] = convertNegotiated(e.Data)
	}

	result["neighbor"] = neighbor
	return result
}

// convertNegotiated converts ZeBGP negotiated caps to ExaBGP format.
//
// Key conversions:
//   - Family format: "ipv4/unicast" -> "ipv4 unicast".
//   - Timer: ZeBGP nests hold-time under "timer", ExaBGP expects flat "hold_time".
//     The protocol-level JSON uses "hold-time" (BGP OPEN field), not "receive-hold-time" (config key).
func convertNegotiated(zebgp map[string]any) map[string]any {
	if zebgp == nil {
		return map[string]any{}
	}

	result := make(map[string]any)

	// Extract hold-time from timer container (Ze protocol JSON uses "hold-time")
	if timer, ok := zebgp["timer"].(map[string]any); ok {
		if v, ok := timer["hold-time"]; ok {
			result["hold_time"] = v
		}
	}

	// Map ZeBGP keys to ExaBGP underscored keys
	if v, ok := zebgp["asn4"]; ok {
		result["asn4"] = v
	}

	// Convert families: "ipv4/unicast" -> "ipv4 unicast"
	if families, ok := zebgp["families"].([]any); ok {
		result["families"] = convertFamilyList(families)
	}

	// Convert add-path (ZeBGP uses hyphen, ExaBGP uses underscore)
	if addPath, ok := zebgp["add-path"].(map[string]any); ok {
		converted := make(map[string]any)
		if send, ok := addPath["send"].([]any); ok {
			converted["send"] = convertFamilyList(send)
		}
		if recv, ok := addPath["receive"].([]any); ok {
			converted["receive"] = convertFamilyList(recv)
		}
		result["add_path"] = converted
	}

	// Convert paths-limit (ZeBGP uses hyphen, ExaBGP uses underscore; families use space separator)
	if pathsLimit, ok := zebgp["paths-limit"].(map[string]any); ok {
		converted := make(map[string]any)
		if send, ok := pathsLimit["send"].(map[string]any); ok {
			converted["send"] = convertFamilyKeyMap(send)
		}
		if recv, ok := pathsLimit["receive"].(map[string]any); ok {
			converted["receive"] = convertFamilyKeyMap(recv)
		}
		result["paths_limit"] = converted
	}

	return result
}

// convertFamilyList converts a list of families from ZeBGP to ExaBGP format:
// "ipv4/unicast" becomes "ipv4 unicast".
//
// Through exabgpFamilyName, not a slash swap. Two SAFIs are spelled differently
// by the two projects -- ze's mpls-label is ExaBGP's nlri-mpls, and ze's mvpn is
// its mcast-vpn -- and every other name is identical, which is exactly why a
// bare ReplaceAll looked right for years.
func convertFamilyList(families []any) []string {
	result := make([]string, 0, len(families))
	for _, f := range families {
		if s, ok := f.(string); ok {
			result = append(result, exabgpFamilyName(s))
		}
	}
	return result
}

// convertFamilyKeyMap converts a map with family keys from ZeBGP to ExaBGP
// format. Keys: "ipv4/unicast" -> "ipv4 unicast". Values are preserved as-is.
//
// Through exabgpFamilyName, for the reason convertFamilyList states.
func convertFamilyKeyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[exabgpFamilyName(k)] = v
	}
	return result
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// updateMessageJSON renders the `message` member of an UPDATE event.
//
// ExaBGP writes one of two bodies and NEVER omits the member
// (src/exabgp/reactor/api/response/json.py, _update): an End-of-RIB is
// `{"eor": {"afi": "ipv4", "safi": "unicast"}}`, and every other UPDATE is
// `{"update": {...}}`, empty object included.
//
// Omitting it was the silently-wrong value `ai/rules/principles.md` bans. A
// sent End-of-RIB carries no announce, no withdraw and no attribute, so the
// member was dropped and the script met an `update` event with no message in
// it: test/exabgp-compat/etc/run/api-api.receive.run classifies each line it
// reads and took its failure branch on that one.
func updateMessageJSON(data map[string]any) map[string]any {
	// A literal rather than msgTypeEOR, for the reason convertUpdateIPC2 spells
	// `announce` and `withdraw` as literals: this one names a key in ExaBGP's
	// own UPDATE JSON, and a rename of ze's event kind must not follow it here.
	if afi, safi, ok := eorFamilyParts(data); ok {
		return map[string]any{"eor": map[string]any{"afi": afi, "safi": safi}}
	}
	return map[string]any{msgTypeUpdate: convertUpdateIPC2(data)}
}

// eorFamilyParts answers the AFI and the SAFI of an End-of-RIB marker, and
// whether the UPDATE is one.
//
// ze states the marker as `"eor": {"family": "ipv4/unicast"}` inside the update
// object (internal/component/bgp/format/text_update.go, appendEORUpdateJSON),
// and ExaBGP states the same two words apart. A family that does not split is
// not answered, because an AFI invented from half a name is worse for a reader
// than the ordinary update body.
func eorFamilyParts(data map[string]any) (afi, safi string, ok bool) {
	marker, isMarker := data[msgTypeEOR].(map[string]any)
	if !isMarker {
		return "", "", false
	}
	name, named := marker["family"].(string)
	if !named {
		return "", "", false
	}
	return strings.Cut(name, "/")
}

// convertUpdateIPC2 converts ze-bgp JSON UPDATE event data to ExaBGP format.
// ze-bgp JSON: attr in "attr" object, nlri in "nlri" object with family keys.
func convertUpdateIPC2(eventData map[string]any) map[string]any {
	update := make(map[string]any)

	// Extract attributes from "attr" object.
	// Strip redundant :bytes suffix from rate-limit extended communities.
	if attrObj, ok := eventData["attr"].(map[string]any); ok && len(attrObj) > 0 {
		update["attribute"] = normalizeOutgoingAttributes(attrObj)
	}

	// Convert NLRI sections from "nlri" object
	announce := make(map[string]map[string][]any)
	withdraw := make(map[string][]any)

	if nlriObj, ok := eventData["nlri"].(map[string]any); ok {
		for fam, value := range nlriObj {
			// Convert family: "ipv4/unicast" -> "ipv4 unicast"
			exabgpFamily := exabgpFamilyName(fam)

			entries, ok := value.([]any)
			if !ok {
				continue
			}

			for _, e := range entries {
				entry, ok := e.(map[string]any)
				if !ok {
					continue
				}

				action, _ := entry["action"].(string)
				nlriList, _ := entry["nlri"].([]any)
				nextHop, _ := entry["next-hop"].(string)

				switch action {
				case "add":
					// ExaBGP's word for "this family carries no next hop",
					// which is every flow route: its json.py writes the
					// announce under `no-nexthop`. `null` was ze's own
					// spelling and no ExaBGP script keys on it.
					nhKey := nextHop
					if nhKey == "" {
						nhKey = "no-nexthop"
					}
					if announce[exabgpFamily] == nil {
						announce[exabgpFamily] = make(map[string][]any)
					}

					for _, nlri := range nlriList {
						if s, ok := nlri.(string); ok {
							announce[exabgpFamily][nhKey] = append(announce[exabgpFamily][nhKey], map[string]any{bridgeUpdateNLRI: s})
						} else {
							announce[exabgpFamily][nhKey] = append(announce[exabgpFamily][nhKey], exabgpNLRIObject(exabgpFamily, nlri))
						}
					}
				case "del":
					for _, nlri := range nlriList {
						if s, ok := nlri.(string); ok {
							withdraw[exabgpFamily] = append(withdraw[exabgpFamily], map[string]any{bridgeUpdateNLRI: s})
						} else {
							withdraw[exabgpFamily] = append(withdraw[exabgpFamily], nlri)
						}
					}
				}
			}
		}
	}

	// Literals rather than announceVerb and withdrawVerb, which spell the same
	// two words for a different fact: those name the API COMMAND grammar, these
	// name keys in ExaBGP's UPDATE JSON. One declaration serving both would let
	// a rename of either follow the other in silence.
	if len(announce) > 0 {
		update["announce"] = announce
	}
	if len(withdraw) > 0 {
		update["withdraw"] = withdraw
	}

	return update
}

// flowComponentOrder is the order ExaBGP prints a flow's components in its
// `string` member, which is the order a flow route is WRITTEN in rather than
// the ascending component-type order the wire carries.
var flowComponentOrder = []string{
	bridgeFlowDestIPv4, bridgeFlowSourceIPv4, bridgeFlowDestIPv6, bridgeFlowSourceIPv6,
	bridgeFlowProtocol, bridgeFlowNextHeader, "port", "destination-port", "source-port",
	bridgeFlowICMPType, "icmp-code", bridgeFlowTCPFlags, "packet-length", bridgeFlowDSCP, bridgeFlowTrafficClass,
	bridgeFlowFragment, "flow-label",
}

// The flow component names this file and bridge_command.go both spell.
const (
	bridgeFlowDestIPv4     = "destination-ipv4"
	bridgeFlowSourceIPv4   = "source-ipv4"
	bridgeFlowDestIPv6     = "destination-ipv6"
	bridgeFlowSourceIPv6   = "source-ipv6"
	bridgeFlowNextHeader   = "next-header"
	bridgeFlowICMPType     = "icmp-type"
	bridgeFlowProtocol     = "protocol"
	bridgeFlowTCPFlags     = "tcp-flags"
	bridgeFlowFragment     = "fragment"
	bridgeFlowDSCP         = "dscp"
	bridgeFlowTrafficClass = "traffic-class"
)

// exabgpNLRIMembers renames the members of a non-flow NLRI object to the words
// ExaBGP writes, for the families where the two projects chose differently.
//
// `prefix` against `nlri` and `labels` against `label` are labeled-unicast
// (RFC 8277); the five l2vpn names are VPLS (RFC 4761). Every other member is
// spelled the same on both sides, which is why only these are listed.
var exabgpNLRIMembers = map[string]string{
	"prefix":          bridgeUpdateNLRI,
	"labels":          "label",
	"label-base":      "base",
	"ve-id":           "endpoint",
	"ve-block-offset": "offset",
	"ve-block-size":   "size",
}

// exabgpNLRIObject restates one non-string NLRI the way ExaBGP writes it: a
// flow through its own rules, anything else by renaming the members above.
//
// The FAMILY decides which, rather than the members the object happens to
// carry. Asking whether an IPv4 source or destination was present answered
// wrong for `flow packet-length [ >200&<300 ]`, which is a flow that matches no
// address at all: RFC 8955 Section 4.2 makes every component optional, so no
// member is the one that identifies the family.
func exabgpNLRIObject(exabgpFamily string, nlri any) any {
	object, ok := nlri.(map[string]any)
	if !ok {
		return nlri
	}
	if isFlowFamily(exabgpFamily) {
		return exabgpFlowNLRI(exabgpFamily, nlri)
	}
	renamed := make(map[string]any, len(object))
	for key, value := range object {
		if key == bridgeNLRIRD {
			renamed[key] = exabgpRD(value)
			continue
		}
		if key == bridgeNLRIPathID {
			renamed["path-information"] = exabgpPathInformation(value)
			continue
		}
		if exabgpName, held := exabgpNLRIMembers[key]; held {
			renamed[exabgpName] = value
			continue
		}
		renamed[key] = value
	}
	return renamed
}

// bridgeNLRIRD is the member every VPN family states its route distinguisher
// in, spelled the same by both projects. bridgeNLRIPathID is ze's name for the
// RFC 7911 Path Identifier.
const (
	bridgeNLRIRD     = "rd"
	bridgeNLRIPathID = "path-id"
)

// exabgpPathInformation restates an RFC 7911 Path Identifier the way ExaBGP
// writes it: the four octets joined with dots, `1.2.3.4` for 16909060.
//
// ze states the number, which is what Section 3 calls it ("a 4-octet value").
// ExaBGP prints the octets (PathInfo.json, nlri/qualifier/path.py), and
// operators read them that way because a path identifier is commonly derived
// from a router id. Nothing is lost either way, so the fold is a spelling and
// not a conversion.
func exabgpPathInformation(value any) any {
	number, isNumber := value.(float64)
	if !isNumber || number < 0 || number > math.MaxUint32 {
		return value
	}
	identifier := uint32(number)
	var tb textbuf.Buffer
	tb.Uint8(uint8(identifier >> 24)).Byte('.').Uint8(uint8(identifier >> 16)).Byte('.').
		Uint8(uint8(identifier >> 8)).Byte('.').Uint8(uint8(identifier))
	return tb.String()
}

// exabgpRD restates a route distinguisher the way ExaBGP writes it.
//
// ze states the RFC 4364 Section 4.2 type ahead of the value -- `0:65000:1`,
// `1:192.168.201.1:123` -- because Type 0 and Type 2 are indistinguishable for
// an AS number of 65535 or less, and ParseRDString has to read String()'s own
// output back (internal/core/bgp/nlri/rd.go). ExaBGP writes the value alone,
// `65000:1`, and lives with the ambiguity because make_from_elements picks the
// type from how large the administrator is (_str, qualifier/rd.py).
//
// Nothing is lost by dropping the type here. The wire carried it, ze's own
// readers still see it, and this text is read by a person rather than parsed
// back into an RD.
func exabgpRD(value any) any {
	text, isText := value.(string)
	if !isText {
		return value
	}
	declared, rest, split := strings.Cut(text, ":")
	if !split {
		return value
	}
	// Only the three types ze prefixes. Its fallback arm writes `rd-typeN:` for
	// anything else, which ExaBGP renders as hex, so leaving it alone states
	// something a reader can still act on rather than a truncation.
	switch declared {
	case "0", "1", "2":
		return rest
	}
	return value
}

// exabgpFlowComponentName renames the one component the two projects spell
// differently, and answers every other name unchanged.
//
// RFC 8956 Section 8 registers component type 11 as `DSCP` under both its IPv4
// and its IPv6 name, which is the name ze publishes. ExaBGP calls the IPv6 one
// `traffic-class` (FlowTrafficClass.NAME, its flow.py), after the IPv6 header
// field RFC 8200 Section 3 gives those octets, and a script keying on the
// ExaBGP name finds nothing under ze's.
//
// The family decides it, because the SAME component type is `dscp` on IPv4 and
// `traffic-class` on IPv6 in ExaBGP's vocabulary.
func exabgpFlowComponentName(exabgpFamily, component string) string {
	if component == bridgeFlowDSCP && strings.HasPrefix(exabgpFamily, "ipv6 ") {
		return bridgeFlowTrafficClass
	}
	return component
}

// isFlowFamily answers for the two SAFIs RFC 8955 Section 3 defines, `flow`
// (SAFI 133) and `flow-vpn` (SAFI 134), which the bridge writes as the second
// word of a family name.
func isFlowFamily(exabgpFamily string) bool {
	_, safi, split := strings.Cut(exabgpFamily, " ")
	return split && strings.HasPrefix(safi, "flow")
}

// exabgpFlowNLRI restates one flow NLRI the way ExaBGP writes it.
//
// Three differences, all of them ze's own representation showing through:
//
//   - ze nests each component's values twice, because componentToJSON answers
//     [][]string to carry flowspec's OR-of-ANDs (plugins/nlri/flowspec,
//     plugin_decode.go). ExaBGP writes one flat list per component.
//   - an IPv4 prefix carries a trailing offset, `10.0.1.0/24/0`. RFC 5575 gives
//     an IPv4 flow prefix no offset at all -- it is RFC 8956's IPv6 form that
//     has one -- so a zero offset is noise no ExaBGP reader expects.
//   - ExaBGP adds a `string` member holding the flow as an operator writes it,
//     and ze writes none.
//
// Anything that is not a flow object is returned untouched, so this is safe on
// every family.
func exabgpFlowNLRI(exabgpFamily string, nlri any) any {
	components, ok := nlri.(map[string]any)
	if !ok {
		return nlri
	}
	flat := make(map[string]any, len(components)+1)
	for key, value := range components {
		if key == bridgeNLRIRD {
			flat[key] = exabgpRD(value)
			continue
		}
		flat[exabgpFlowComponentName(exabgpFamily, key)] = flattenFlowComponent(key, value)
	}
	if text := flowComponentString(flat); text != "" {
		flat["string"] = text
	}
	return flat
}

// flattenFlowComponent restates one component's values the way ExaBGP does.
//
// ze's outer list is the OR and its INNER list is the AND (componentToJSON,
// plugins/nlri/flowspec/plugin_decode.go). ExaBGP states the same thing on one
// level by joining an AND group with `&`: `[[">8080", "<8088"], ["=3128"]]`
// becomes `[">8080&<8088", "=3128"]`. Flattening both levels instead would say
// three independent matches where the route states two, which is a different
// filter.
func flattenFlowComponent(key string, value any) any {
	outer, ok := value.([]any)
	if !ok {
		return value
	}
	joined := make([]any, 0, len(outer))
	for _, member := range outer {
		inner, nested := member.([]any)
		if !nested {
			joined = append(joined, flowValueText(key, member))
			continue
		}
		var tb textbuf.Buffer
		for index, item := range inner {
			if index > 0 {
				tb.Byte('&')
			}
			text, isText := flowValueText(key, item).(string)
			if !isText {
				return value
			}
			tb.Str(text)
		}
		joined = append(joined, tb.String())
	}
	return joined
}

// flowICMPTypeNames is ExaBGP main's own table, transcribed from
// ICMPType.names (src/exabgp/protocol/ip/icmp.py): the IANA icmp-parameters
// values it chooses to name, lowercased with underscores as hyphens. ze writes
// the number, so a script matching on `=echo-request` sees nothing.
//
// A type absent here keeps its number, which is what ExaBGP does for one it
// does not name.
var flowICMPTypeNames = map[string]string{
	"0": "echo-reply", "3": "unreachable", "5": "redirect", "8": "echo-request",
	"9": "router-advertisement", "10": "router-solicit", "11": "time-exceeded",
	"12": "parameter-problem", "13": "timestamp", "14": "timestamp-reply",
	"40": "photuris", "41": "experimental-mobility",
	"42": "extended-echo-request", "43": "extended-echo-reply",
	"253": "experimental-one", "254": "experimental-two",
}

// flowValueText normalises one value: an IPv4 prefix loses the offset it does
// not have, and an ICMP type is named rather than numbered.
//
// A bitmask flag used to lose a `=` here, because ze wrote one for every
// operator including RFC 8955 Section 4.2.1.2's bare "any bit set". That is
// fixed at the decoder now (formatBitmaskValue, plugins/nlri/flowspec), so the
// `=` that arrives is the one the peer really sent and stripping it would lose
// the distinction a second time.
func flowValueText(key string, value any) any {
	trimmed := trimFlowPrefixOffset(key, value)
	text, ok := trimmed.(string)
	if !ok {
		return trimmed
	}
	if key == bridgeFlowICMPType {
		// The operator travels with the value: `=8` is named `=echo-request`.
		operator, number := flowSplitOperator(text)
		if name, named := flowICMPTypeNames[number]; named {
			return operator + name
		}
	}
	return text
}

// flowSplitOperator separates a match operator from the value it compares, so
// a rename touches the value alone. ExaBGP writes the same operators ze does.
func flowSplitOperator(text string) (string, string) {
	for index, char := range text {
		if char != '=' && char != '<' && char != '>' && char != '!' && char != '&' {
			return text[:index], text[index:]
		}
	}
	return text, ""
}

// trimFlowPrefixOffset drops the `/0` ze appends to an IPv4 flow prefix.
//
// Only for the IPv4 keys, and only a ZERO offset: RFC 8956 Section 3 gives an
// IPv6 flow prefix a real offset, and dropping a non-zero one would state a
// different match.
func trimFlowPrefixOffset(key string, value any) any {
	if key != bridgeFlowSourceIPv4 && key != bridgeFlowDestIPv4 {
		return value
	}
	text, ok := value.(string)
	if !ok {
		return value
	}
	return strings.TrimSuffix(text, "/0")
}

// flowComponentString renders the `string` member: the flow as an operator
// writes it, which is what ExaBGP puts beside the components.
func flowComponentString(flat map[string]any) string {
	var tb textbuf.Buffer
	tb.Str("flow")
	wrote := false
	for _, key := range flowComponentOrder {
		values, held := flat[key].([]any)
		if !held || len(values) == 0 {
			continue
		}
		tb.Byte(' ').Str(key)
		// ExaBGP brackets a component that states more than one match and
		// writes a single one bare, which is how an operator writes it back.
		if len(values) > 1 {
			tb.Str(" [")
		}
		for _, value := range values {
			text, isText := value.(string)
			if !isText {
				return ""
			}
			tb.Byte(' ').Str(text)
		}
		if len(values) > 1 {
			tb.Str(" ]")
		}
		wrote = true
	}
	if !wrote {
		return ""
	}
	// The route distinguisher closes the line, which is where an operator
	// writes it and where ExaBGP's extensive() puts it: `flow` + rules + rd.
	if rd, held := flat[bridgeNLRIRD].(string); held && rd != "" {
		tb.Str(" rd ").Str(rd)
	}
	return tb.String()
}

// exabgpAttributeNames maps ze's name for an attribute to ExaBGP's, for the
// members where the two projects disagree.
//
// ExaBGP writes the community attributes SINGULAR -- `community`,
// `extended-community`, `large-community` -- where ze pluralises, and it names
// attribute 9 `originator-id` where ze falls back to `attr-9`. Every other
// attribute name is identical on both sides, which is why the disagreement
// survived: it is invisible unless something compares the two documents, and
// nothing did until the fixtures' own json expectations were turned on
// (readExaBGPCase, internal/le/interoplab/bgp).
var exabgpAttributeNames = map[string]string{
	"communities":          bridgeAttrCommunity,
	"extended-communities": "extended-community",
	"large-communities":    bridgeAttrLargeCommunity,
	// RFC 5701's 20-octet form. ExaBGP puts the family word LAST here and
	// first nowhere else (attribute.py, Attribute.CODE names), so the two
	// spellings differ by more than ze's plural.
	"ipv6-extended-communities": "extended-community-ipv6",
	// attr-9 is gone: bgp-rr now registers a real originator-id formatter, so
	// ze names it the same as ExaBGP does and there is nothing to translate.
}

func normalizeOutgoingAttributes(attrObj map[string]any) map[string]any {
	extComms, hasExt := attrObj["extended-community"]
	var normalized any
	changed := false
	if hasExt {
		normalized, changed = normalizeOutgoingExtendedCommunities(extComms)
	}

	// ExaBGP states the next hop as the KEY the prefixes hang under inside
	// `announce`, and never again inside `attribute`. Ze's own document carries
	// it in both places, which is right for a ze reader and is one member too
	// many for a script parsing ExaBGP's shape.
	//
	// An EMPTY as-path is dropped for the same reason: ExaBGP writes the member
	// only when the path has hops, so an `"as-path": []` is a member its readers
	// never see. Both were measured against test/exabgp-compat/api/api-api.ci,
	// which states the exact document ExaBGP produces for one announce.
	_, hasNextHop := attrObj["next-hop"]
	emptyPath := false
	if path, ok := attrObj["as-path"].([]any); ok && len(path) == 0 {
		emptyPath = true
	}
	renameable := false
	for zeName := range exabgpAttributeNames {
		if _, held := attrObj[zeName]; held {
			renameable = true
			break
		}
	}
	if !changed && !hasNextHop && !emptyPath && !renameable {
		return attrObj
	}

	cloned := make(map[string]any, len(attrObj))
	maps.Copy(cloned, attrObj)
	if changed {
		cloned["extended-communities"] = normalized
	}
	delete(cloned, "next-hop")
	if emptyPath {
		delete(cloned, "as-path")
	}
	for zeName, exabgpName := range exabgpAttributeNames {
		value, held := cloned[zeName]
		if !held {
			continue
		}
		delete(cloned, zeName)
		cloned[exabgpName] = value
	}
	// ExaBGP states a community as its NUMBERS, not as the text a human reads:
	// `[[30740, 0]]` where ze writes `["30740:0"]`, and `[[1, 2, 3]]` for a
	// large community where ze writes `["1:2:3"]`. A script indexes those
	// numbers, so the colon form is unreadable to it.
	if split := splitColonCommunities(cloned[bridgeAttrCommunity], 2); split != nil {
		cloned[bridgeAttrCommunity] = split
	}
	if split := splitColonCommunities(cloned[bridgeAttrLargeCommunity], 3); split != nil {
		cloned[bridgeAttrLargeCommunity] = split
	}
	return cloned
}

// splitColonCommunities turns ze's colon-joined community strings into the
// number tuples ExaBGP writes, and answers nil when the value is not that
// shape, so a caller leaves what it was given alone.
//
// parts is how many numbers one community holds: two for a community, three for
// a large one. A member with any other count is left as a string rather than
// guessed at, because a wrong tuple is worse than an unconverted one.
func splitColonCommunities(value any, parts int) []any {
	list, ok := value.([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	converted := make([]any, 0, len(list))
	for _, member := range list {
		text, isText := member.(string)
		if !isText {
			return nil
		}
		fields := strings.Split(text, ":")
		if len(fields) != parts {
			return nil
		}
		numbers := make([]any, 0, parts)
		for _, field := range fields {
			number, err := strconv.ParseUint(field, 10, 32)
			if err != nil {
				return nil
			}
			numbers = append(numbers, float64(number))
		}
		converted = append(converted, numbers)
	}
	return converted
}

func normalizeOutgoingExtendedCommunities(value any) (any, bool) {
	switch typed := value.(type) {
	case []any:
		return normalizeOutgoingExtendedCommunityList(typed)
	case map[string]any:
		values, ok := typed["value"]
		if !ok {
			return value, false
		}
		normalized, changed := normalizeOutgoingExtendedCommunities(values)
		if !changed {
			return value, false
		}
		cloned := make(map[string]any, len(typed))
		maps.Copy(cloned, typed)
		cloned["value"] = normalized
		return cloned, true
	default:
		return value, false
	}
}

func normalizeOutgoingExtendedCommunityList(entries []any) (any, bool) {
	var cloned []any
	for i, entry := range entries {
		normalized, changed := normalizeOutgoingExtendedCommunityEntry(entry)
		if !changed {
			if cloned != nil {
				cloned = append(cloned, entry)
			}
			continue
		}
		if cloned == nil {
			cloned = make([]any, 0, len(entries))
			cloned = append(cloned, entries[:i]...)
		}
		cloned = append(cloned, normalized)
	}
	if cloned == nil {
		return entries, false
	}
	return cloned, true
}

func normalizeOutgoingExtendedCommunityEntry(entry any) (any, bool) {
	typed, ok := entry.(map[string]any)
	if !ok {
		return entry, false
	}
	value, ok := typed["string"].(string)
	if !ok {
		return entry, false
	}
	normalized := normalizeOutgoingExtendedCommunityToken(value)
	if normalized == value {
		return entry, false
	}
	cloned := make(map[string]any, len(typed))
	maps.Copy(cloned, typed)
	cloned["string"] = normalized
	return cloned, true
}

func normalizeOutgoingExtendedCommunityToken(value string) string {
	if strings.HasSuffix(value, ":bytes") && strings.HasPrefix(value, "rate-limit:") {
		return strings.TrimSuffix(value, ":bytes")
	}
	return value
}

// ExaBGP's own names for the four events that are not a BGP message.
const (
	apiKeyNeighborChanges = "neighbor-changes"
	apiKeyNegotiated      = "negotiated"
	apiKeyFSM             = "fsm"
	apiKeySignal          = "signal"
)

// APIKey answers the name an ExaBGP neighbor's `api` block uses to grant this
// event to a process.
//
// ExaBGP flattens an api block into one key per event: the four standalone
// words below, and `<direction>-<message>` for a BGP message, where the message
// word is Message.CODE.SHORT -- `open`, `update`, `notification`, `keepalive`,
// `refresh`, `operational`. Its dispatcher then walks the processes THAT key
// names rather than every process it runs
// (src/exabgp/configuration/neighbor/api.py, ParseAPI.flatten;
// src/exabgp/reactor/api/processes.py, Processes._notify).
//
// Ze spells the same message kinds the same way, so the key is the direction
// and the kind with a hyphen between them.
func (e Event) APIKey() string {
	switch e.Kind {
	case msgTypeState:
		return apiKeyNeighborChanges
	case msgTypeNegotiated:
		return apiKeyNegotiated
	case msgTypeFSM:
		return apiKeyFSM
	case msgTypeSignal:
		return apiKeySignal
	}

	var tb textbuf.Buffer
	return tb.Str(e.Direction).Byte('-').Str(e.Kind).String()
}
