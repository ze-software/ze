// Design: docs/architecture/exabgp-bridge.md — the negotiated block on every UPDATE line
// Overview: bridge_event.go — ZeBGP to ExaBGP JSON event translation
// Related: bridge.go — the external runner, which notes every event it renders
// Related: ../../plugins/exabgp/bridgerun/fleet.go — the internal runner, which does the same

package bridge

import (
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
)

// sessionStateDown is ze's state word for a session that left Established,
// which ends what Sessions keeps of it.
const sessionStateDown = "down"

// Sessions is what a runner keeps of each peer's session, so an event can be
// rendered with what an earlier event of the same session said.
//
// ExaBGP writes the session's negotiated block inside every UPDATE line, and
// inside every `packets` line, of a process the neighbor grants `negotiated`
// (src/exabgp/reactor/protocol.py, read_message: "negotiated =
// self.negotiated if self.neighbor.api.get('negotiated', False) else None";
// Response.JSON.update: "if negotiated: message.update(self._negotiated(...))").
// Ze states the block once, in its own `negotiated` event, so the runner keeps
// it per peer and hands it to each UPDATE of that peer.
//
// The zero value is ready to use. Safe for concurrent use. An entry lives from
// the peer's `negotiated` event to its `down`, so there is at most one per
// configured peer.
type Sessions struct {
	mu sync.Mutex
	// negotiated is each peer's block, by remote address, in ExaBGP's form.
	// A block is never changed once stored, so every line of the session
	// shares it.
	negotiated map[string]map[string]any
}

// Note reads one event into the sessions and gives the event what its
// session holds. A runner MUST call it on every event before it renders one,
// including events no script is granted, or a session's block is missed.
func (s *Sessions) Note(e *Event) {
	switch e.Kind {
	case msgTypeNegotiated:
		block := convertNegotiated(e.negotiatedData())
		s.mu.Lock()
		if s.negotiated == nil {
			s.negotiated = make(map[string]map[string]any)
		}
		s.negotiated[e.Peer] = block
		s.mu.Unlock()
	case msgTypeState:
		if state, _ := e.Payload["state"].(string); state == sessionStateDown {
			s.mu.Lock()
			delete(s.negotiated, e.Peer)
			s.mu.Unlock()
		}
	case msgTypeUpdate, msgTypeOpen, msgTypeNotification, msgTypeKeepalive, msgTypeRefresh:
		// An UPDATE line carries the block, and so does the packet line of
		// every message (Response.JSON.packets); the other parsed lines do not
		// write it.
		s.mu.Lock()
		e.Negotiated = s.negotiated[e.Peer]
		s.mu.Unlock()
	}
}

// negotiatedData is the event's `negotiated` object, or nil when it carries
// none. Event.Data is not it: ReadEvent falls back to the whole payload for an
// event with no object of its kind, which would state a session with no
// families.
func (e Event) negotiatedData() map[string]any {
	block, _ := e.Payload[msgTypeNegotiated].(map[string]any)
	return block
}

// APIKeyNegotiated is the grant that puts the negotiated block on a script's
// UPDATE lines, the same key that grants it the `negotiated` event.
const APIKeyNegotiated = apiKeyNegotiated

// WithoutNegotiated answers the event with no negotiated block, for a script
// the peer does not grant `negotiated`: ExaBGP passes such a process None, and
// its line then carries no block.
func (e Event) WithoutNegotiated() Event {
	e.Negotiated = nil
	return e
}

// convertNegotiated renders ze's negotiated block as ExaBGP's
// (src/exabgp/reactor/api/response/json.py, Response.JSON._negotiated):
//
//	{"message_size": 65535, "hold_time": 180, "asn4": true,
//	 "multisession": false, "operational": false, "refresh": "enhanced",
//	 "families": ["ipv4 unicast", "ipv6 unicast"], "nexthop": [],
//	 "add_path": {"send": [], "receive": []}}
//
// The families are in AFI then SAFI order, the order ExaBGP lists them in.
// `multisession` and `operational` are false on every ze session: ze opens no
// multi-session session (META-DECISIONS M16) and speaks no OPERATIONAL (M15).
// The empty lists are written, as ExaBGP writes them, so a helper that indexes
// `add_path["send"]` meets no KeyError.
//
// A nil block answers an empty map: there is nothing to state.
func convertNegotiated(zebgp map[string]any) map[string]any {
	if zebgp == nil {
		return map[string]any{}
	}

	result := map[string]any{
		"multisession": false,
		"operational":  false,
		"nexthop":      []string{},
		"add_path":     map[string]any{"send": []string{}, "receive": []string{}},
	}

	// Ze nests the hold time under `timer` and spells it `hold-time`, the
	// OPEN field's name.
	if timer, ok := zebgp["timer"].(map[string]any); ok {
		if v, ok := timer["hold-time"]; ok {
			result["hold_time"] = v
		}
	}
	if v, ok := zebgp["message-size"]; ok {
		result["message_size"] = v
	}
	if v, ok := zebgp["asn4"]; ok {
		result["asn4"] = v
	}
	// Ze's words are ExaBGP's: absent, normal, enhanced (REFRESH.json).
	if v, ok := zebgp["route-refresh"]; ok {
		result["refresh"] = v
	}
	if families, ok := zebgp["families"].([]any); ok {
		result["families"] = convertFamilyList(families)
	}
	if addPath, ok := zebgp["add-path"].(map[string]any); ok {
		converted := map[string]any{"send": []string{}, "receive": []string{}}
		if send, ok := addPath["send"].([]any); ok {
			converted["send"] = convertFamilyList(send)
		}
		if receive, ok := addPath["receive"].([]any); ok {
			converted["receive"] = convertFamilyList(receive)
		}
		result["add_path"] = converted
	}
	// ExaBGP writes one "<afi> <safi> <next-hop afi>" per RFC 8950 tuple.
	if nextHops, ok := zebgp["extended-next-hop"].(map[string]any); ok {
		result["nexthop"] = convertNextHops(nextHops)
	}
	// Paths-limit is ze's, with no ExaBGP spelling to match: its keys follow
	// the ExaBGP conventions (underscore, space-separated families).
	if pathsLimit, ok := zebgp["paths-limit"].(map[string]any); ok {
		converted := make(map[string]any)
		if send, ok := pathsLimit["send"].(map[string]any); ok {
			converted["send"] = convertFamilyKeyMap(send)
		}
		if receive, ok := pathsLimit["receive"].(map[string]any); ok {
			converted["receive"] = convertFamilyKeyMap(receive)
		}
		result["paths_limit"] = converted
	}
	return result
}

// convertNextHops answers ze's family to next-hop AFI map as ExaBGP's list,
// in the order of the families.
func convertNextHops(nextHops map[string]any) []string {
	families := make([]string, 0, len(nextHops))
	for name := range nextHops {
		families = append(families, name)
	}
	slices.SortStableFunc(families, compareFamilyNames)
	result := make([]string, 0, len(families))
	for _, name := range families {
		afi, _ := nextHops[name].(string)
		result = append(result, exabgpFamilyName(name)+" "+afi)
	}
	return result
}

// convertFamilyList converts a list of families from ZeBGP to ExaBGP format:
// "ipv4/unicast" becomes "ipv4 unicast", in AFI then SAFI order.
//
// Through exabgpFamilyName, not a slash swap. Two SAFIs are spelled differently
// by the two projects -- ze's mpls-label is ExaBGP's nlri-mpls, and ze's mvpn is
// its mcast-vpn -- and every other name is identical, which is exactly why a
// bare ReplaceAll looked right for years.
//
// The order is ExaBGP's, and ze's event does not keep one: it lists the
// families as a Go map iterates them, so two events of one session could
// disagree, and a 5.0 helper comparing the list met a different one on each
// line (TestOldResponses50).
func convertFamilyList(families []any) []string {
	names := make([]string, 0, len(families))
	for _, f := range families {
		if s, ok := f.(string); ok {
			names = append(names, s)
		}
	}
	slices.SortStableFunc(names, compareFamilyNames)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, exabgpFamilyName(name))
	}
	return result
}

// compareFamilyNames orders two ze family names by AFI, then SAFI. A name the
// family registry does not know sorts after every known one, and two unknown
// names keep the order they came in.
func compareFamilyNames(a, b string) int {
	first, firstKnown := family.LookupFamily(a)
	second, secondKnown := family.LookupFamily(b)
	if !firstKnown {
		if secondKnown {
			return 1
		}
		return 0
	}
	if !secondKnown {
		return -1
	}
	if family.FamilyLess(first, second) {
		return -1
	}
	if family.FamilyLess(second, first) {
		return 1
	}
	return 0
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
