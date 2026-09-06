// Design: docs/architecture/core-design.md — ZeBGP to ExaBGP JSON event translation
// Overview: bridge.go — startup protocol, bridge runtime
// Related: bridge_command.go — ExaBGP text command translation
// Related: bridge_muxconn.go — MuxConn wire format parsing for post-startup I/O

package bridge

import (
	"maps"
	"os"
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
	msgTypeOpen         = "open"
	msgTypeUpdate       = "update"
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
	// PeerASN is the remote AS, as JSON delivered it.
	PeerASN float64
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
//	    "address": {"peer": "10.0.0.1"},
//	    "asn": {"peer": 65001},
//	    "direction": "receive",
//	    "message": {"update": {...}}
//	  }
//	}
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
		"address":   map[string]any{"peer": e.Peer},
		"asn":       map[string]any{"peer": e.PeerASN},
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
		update := convertUpdateIPC2(e.Data)
		if len(update) > 0 {
			neighbor["message"] = map[string]any{"update": update}
		}

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

// convertFamilyList converts a list of families from ZeBGP to ExaBGP format.
// Converts "ipv4/unicast" to "ipv4 unicast".
func convertFamilyList(families []any) []string {
	result := make([]string, 0, len(families))
	for _, f := range families {
		if s, ok := f.(string); ok {
			result = append(result, strings.ReplaceAll(s, "/", " "))
		}
	}
	return result
}

// convertFamilyKeyMap converts a map with family keys from ZeBGP to ExaBGP format.
// Keys: "ipv4/unicast" -> "ipv4 unicast". Values are preserved as-is.
func convertFamilyKeyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[strings.ReplaceAll(k, "/", " ")] = v
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
			exabgpFamily := strings.ReplaceAll(fam, "/", " ")

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
					nhKey := nextHop
					if nhKey == "" {
						nhKey = "null"
					}
					if announce[exabgpFamily] == nil {
						announce[exabgpFamily] = make(map[string][]any)
					}

					for _, nlri := range nlriList {
						if s, ok := nlri.(string); ok {
							announce[exabgpFamily][nhKey] = append(announce[exabgpFamily][nhKey], map[string]any{"nlri": s})
						} else {
							announce[exabgpFamily][nhKey] = append(announce[exabgpFamily][nhKey], nlri)
						}
					}
				case "del":
					for _, nlri := range nlriList {
						if s, ok := nlri.(string); ok {
							withdraw[exabgpFamily] = append(withdraw[exabgpFamily], map[string]any{"nlri": s})
						} else {
							withdraw[exabgpFamily] = append(withdraw[exabgpFamily], nlri)
						}
					}
				}
			}
		}
	}

	if len(announce) > 0 {
		update["announce"] = announce
	}
	if len(withdraw) > 0 {
		update["withdraw"] = withdraw
	}

	return update
}

func normalizeOutgoingAttributes(attrObj map[string]any) map[string]any {
	extComms, ok := attrObj["extended-community"]
	if !ok {
		return attrObj
	}
	normalized, changed := normalizeOutgoingExtendedCommunities(extComms)
	if !changed {
		return attrObj
	}
	cloned := make(map[string]any, len(attrObj))
	maps.Copy(cloned, attrObj)
	cloned["extended-community"] = normalized
	return cloned
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
