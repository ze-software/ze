// Design: docs/architecture/exabgp-bridge.md — the negotiated block on a granted script's message lines
// Overview: bridge_event.go — ZeBGP to ExaBGP JSON event translation
// Related: bridge.go — the external runner, which notes every event it renders
// Related: ../../plugins/exabgp/bridgerun/fleet.go — the internal runner, which does the same

package bridge

import (
	"sync"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// sessionStateDown is ze's state word for a session that left Established,
// which ends what Sessions keeps of it.
const sessionStateDown = "down"

// Sessions is what a runner keeps of each peer's session, so an event can be
// rendered with what an earlier event of the same session said.
//
// ExaBGP writes the session's negotiated block inside the message lines of a
// process the neighbor grants `negotiated`, and inside none of a process it
// does not, whatever the API version: the decoded UPDATE, OPEN, NOTIFICATION,
// KEEPALIVE and ROUTE-REFRESH lines and every `packets` and `consolidate` line,
// sent and received. 5.0.1 decided so (src/exabgp/reactor/protocol.py,
// read_message: "negotiated = self.negotiated if
// self.neighbor.api.get('negotiated', False) else None"), and the pin
// f9bfb853b decides so again (Protocol._api_negotiated: "return
// self.negotiated if self._api.get('negotiated', False) else
// Negotiated.UNSET", which Response.JSON writes as nothing). A runner hands
// each script Event.ForGrant with the script's grant for the event's peer. Ze
// states the block once, in its own `negotiated` event, so the runner keeps it
// per peer and hands it to each message of that peer.
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
	// contexts is each peer's attribute encoding context, by remote address,
	// kept whether or not a script is granted the block: every UPDATE line
	// reads its AS_PATH with it (Event.overlayWire).
	contexts map[string]bgpctx.ContextID
}

// Note reads one event into the sessions and gives the event what its
// session holds. A runner MUST call it on every event before it renders one,
// including events no script is granted, or a session's block is missed.
func (s *Sessions) Note(e *Event) {
	switch e.Kind {
	case msgTypeNegotiated:
		block := convertNegotiated(e.negotiatedData())
		contextID := sessionAttrContext(block)
		s.mu.Lock()
		if s.negotiated == nil {
			s.negotiated = make(map[string]map[string]any)
			s.contexts = make(map[string]bgpctx.ContextID)
		}
		s.negotiated[e.Peer] = block
		s.contexts[e.Peer] = contextID
		s.mu.Unlock()
	case msgTypeState:
		if state, _ := e.Payload["state"].(string); state == sessionStateDown {
			s.mu.Lock()
			delete(s.negotiated, e.Peer)
			delete(s.contexts, e.Peer)
			s.mu.Unlock()
		}
	case msgTypeUpdate, msgTypeOpen, msgTypeNotification, msgTypeKeepalive, msgTypeRefresh:
		// An UPDATE line carries the block, and so does the packet line of
		// every message (Response.JSON.packets); the other parsed lines do not
		// write it. carriesNegotiated names the same kinds.
		s.mu.Lock()
		e.Negotiated = s.negotiated[e.Peer]
		e.attrContext = s.contexts[e.Peer]
		s.mu.Unlock()
	}
}

// sessionAttrContext answers the encoding context a session's attributes are
// read with, from its negotiated block: four-octet AS numbers or two (RFC 6793
// Section 4), and the zero ContextID, which overlayWire reads as unknown, for
// a block that states neither or a registry that is full.
func sessionAttrContext(block map[string]any) bgpctx.ContextID {
	asn4, stated := block[negotiatedASN4].(bool)
	if !stated {
		return 0
	}
	contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(asn4))
	if err != nil {
		bridgeLog().Warn("exabgp-bridge: no encoding context for the session, so its UPDATE lines state the AS path as ze flattens it",
			"error", err)
		return 0
	}
	return contextID
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
// message lines, the same key that grants it the `negotiated` event.
const APIKeyNegotiated = apiKeyNegotiated

// ForGrant answers the event as the pin hands it to a process, for every API
// version (Protocol._api_negotiated at f9bfb853b). Granted `negotiated`, a line
// that carries the block carries it always: the session's, or the session
// before its OPENs are exchanged (unexchangedNegotiated) while the runner holds
// none for the peer, such as on the packet line of an OPEN. Not granted, no
// line carries it, even once the session is up.
func (e Event) ForGrant(granted bool) Event {
	if !granted {
		e.Negotiated = nil
		return e
	}
	if e.Negotiated == nil && carriesNegotiated(e.Kind) {
		e.Negotiated = unexchangedNegotiated()
	}
	return e
}

// carriesNegotiated reports whether a line of the kind can carry the
// negotiated block: the kinds Sessions.Note gives one.
func carriesNegotiated(kind string) bool {
	switch kind {
	case msgTypeUpdate, msgTypeOpen, msgTypeNotification, msgTypeKeepalive, msgTypeRefresh:
		return true
	}
	return false
}

// The members of the negotiated block that state one capability's outcome
// (Negotiated in src/exabgp/bgp/message/open/capability/negotiated.py).
const (
	negotiatedASN4         = "asn4"
	negotiatedMultisession = "multisession"
	negotiatedNextHop      = "nexthop"
	negotiatedOperational  = "operational"
)

// unexchangedNegotiated is the pin's block for a session whose OPENs are not
// exchanged yet, the Protocol's own Negotiated before the exchange as
// Response.JSON._negotiated writes it to a granted process: no family, no hold
// time, and RFC 4271's 4096-octet message maximum
// (src/exabgp/bgp/message/open/capability/negotiated.py). It is not
// Negotiated.UNSET, which the pin hands a process not granted `negotiated` and
// writes as no block at all. A fresh map on every call, because a caller's
// line owns it.
func unexchangedNegotiated() map[string]any {
	return map[string]any{
		"message_size": float64(4096), "hold_time": float64(0), negotiatedASN4: false,
		negotiatedMultisession: false, negotiatedOperational: false, "refresh": "absent",
		"families": []string{}, negotiatedNextHop: []string{},
		"add_path": map[string]any{modeSend: []string{}, modeReceive: []string{}},
	}
}

// convertNegotiated renders ze's negotiated block as ExaBGP's
// (src/exabgp/reactor/api/response/json.py, Response.JSON._negotiated):
//
//	{"message_size": 65535, "hold_time": 180, "asn4": true,
//	 "multisession": false, "operational": false, "refresh": "enhanced",
//	 "families": ["ipv4 unicast", "ipv6 unicast"], "nexthop": [],
//	 "add_path": {"send": [], "receive": []}}
//
// The families keep the order ze's event carries, the peer's OPEN order, which
// is the order ExaBGP lists them in (negotiated.py, Negotiated.received): the
// bridge never reorders them.
// `multisession` and `operational` are false on every ze session: ze opens no
// multi-session session and speaks no OPERATIONAL (ai/META-DECISIONS.md).
// The empty lists are written, as ExaBGP writes them, so a helper that indexes
// `add_path["send"]` meets no KeyError.
//
// A nil block answers an empty map: there is nothing to state.
func convertNegotiated(zebgp map[string]any) map[string]any {
	if zebgp == nil {
		return map[string]any{}
	}

	result := map[string]any{
		negotiatedMultisession: false,
		negotiatedOperational:  false,
		negotiatedNextHop:      []string{},
		"add_path":             map[string]any{modeSend: []string{}, modeReceive: []string{}},
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
	if v, ok := zebgp[negotiatedASN4]; ok {
		result[negotiatedASN4] = v
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
		result[negotiatedNextHop] = convertNextHops(nextHops, zebgp["families"])
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

// convertNextHops answers ze's family to next-hop AFI map as ExaBGP's list.
// Ze's event carries the tuples as a JSON object, which holds no order, so the
// list follows the negotiated families: the peer's OPEN order, the order the
// event's `families` carries. A family the map names that the list does not
// is not negotiated, and ze never writes one (NegotiatedToDecoded).
func convertNextHops(nextHops map[string]any, families any) []string {
	names, _ := families.([]any)
	result := make([]string, 0, len(nextHops))
	for _, f := range names {
		name, ok := f.(string)
		if !ok {
			continue
		}
		afi, ok := nextHops[name].(string)
		if !ok {
			continue
		}
		result = append(result, exabgpFamilyName(name)+" "+afi)
	}
	return result
}

// convertFamilyList converts a list of families from ZeBGP to ExaBGP format:
// "ipv4/unicast" becomes "ipv4 unicast", in the order ze's event lists them.
//
// Through exabgpFamilyName, not a slash swap. Two SAFIs are spelled differently
// by the two projects -- ze's mpls-label is ExaBGP's nlri-mpls, and ze's mvpn is
// its mcast-vpn -- and every other name is identical, which is exactly why a
// bare ReplaceAll looked right for years.
//
// The order is not changed. Ze's event lists the families in the order the
// peer's OPEN carried them (capability.Negotiated.Families), and ExaBGP 5.0.1
// lists them the same way (negotiated.py, Negotiated.received), as does the
// pin (f9bfb853b: "self.families = [family for family in
// self._multiprotocol(recv_capa) if family in sent_families]"). The ADD-PATH
// lists are filtered from that list, in its order, on both sides.
func convertFamilyList(families []any) []string {
	result := make([]string, 0, len(families))
	for _, f := range families {
		if name, ok := f.(string); ok {
			result = append(result, exabgpFamilyName(name))
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
