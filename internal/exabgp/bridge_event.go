// Design: docs/architecture/core-design.md — ZeBGP to ExaBGP JSON event translation
// Overview: bridge.go — startup protocol, bridge runtime
// Related: bridge_command.go — ExaBGP text command translation

package exabgp

import (
	"os"
	"strings"
	"time"
)

// Version is the ExaBGP version string used in the JSON envelope.
const Version = "5.0.0"

// Mode/direction string constants.
// Used for both ExaBGP JSON direction field and ADD-PATH CLI mode.
const (
	modeReceive = "receive"
	modeSend    = "send"
	modeBoth    = "both"
)

// Message type constants.
const (
	msgTypeOpen   = "open"
	msgTypeUpdate = "update"
	msgTypeState  = "state"
)

// ZebgpToExabgpJSON converts a ZeBGP JSON event to ExaBGP JSON format.
//
// ZeBGP ze-bgp JSON format (per docs/architecture/api/json-format.md):
//
//	{
//	  "type": "bgp",
//	  "bgp": {
//	    "peer": {"address": "10.0.0.1", "asn": 65001},
//	    "message": {"id": 1, "direction": "received", "type": "update"},
//	    "update": {
//	      "attr": {"origin": "igp"},
//	      "ipv4/unicast": [...]
//	    }
//	  }
//	}
//
// State events use simple string (not container):
//
//	{"type": "bgp", "bgp": {"message": {"type": "state"}, "peer": {...}, "state": "up"}}
//
// ExaBGP format (nested):
//
//	{
//	  "exabgp": "5.0.0",
//	  "type": "update",
//	  "neighbor": {
//	    "address": {"peer": "10.0.0.1"},
//	    "asn": {"peer": 65001},
//	    "direction": "receive",
//	    "message": {"update": {...}}
//	  }
//	}
func ZebgpToExabgpJSON(zebgp map[string]any) map[string]any {
	// Extract from ze-bgp JSON wrapper if present
	bgpPayload := zebgp
	if rootType, _ := zebgp["type"].(string); rootType == "bgp" {
		if bgp, ok := zebgp["bgp"].(map[string]any); ok {
			bgpPayload = bgp
		}
	}

	// Get message metadata from bgp.message
	var msgType string
	direction := modeReceive
	if msg, ok := bgpPayload["message"].(map[string]any); ok {
		msgType, _ = msg["type"].(string)
		if dir, ok := msg["direction"].(string); ok {
			switch dir {
			case "received":
				direction = modeReceive
			case "sent":
				direction = modeSend
			}
		}
	}
	if msgType == "" {
		// Fallback: detect type by presence of key
		if _, ok := bgpPayload[msgTypeOpen]; ok {
			msgType = msgTypeOpen
		} else if _, ok := bgpPayload[msgTypeUpdate]; ok {
			msgType = msgTypeUpdate
		} else if _, ok := bgpPayload[msgTypeState]; ok {
			msgType = msgTypeState
		} else {
			msgType = msgTypeUpdate
		}
	}

	// Get peer from bgp level (ze-bgp JSON format)
	peer, _ := bgpPayload["peer"].(map[string]any)
	peerAddr, _ := peer["address"].(string)
	peerASN, _ := peer["asn"].(float64)

	// Get event-specific data from nested key (except state which is a string)
	eventData := bgpPayload
	if msgType != msgTypeState {
		if nested, ok := bgpPayload[msgType].(map[string]any); ok {
			eventData = nested
		}
	}

	// Build ExaBGP envelope
	result := map[string]any{
		"exabgp": Version,
		"time":   float64(time.Now().Unix()),
		"host":   hostname(),
		"pid":    os.Getpid(),
		"ppid":   os.Getppid(),
		"type":   msgType,
	}

	// Build neighbor section
	neighbor := map[string]any{
		"address":   map[string]any{"peer": peerAddr},
		"asn":       map[string]any{"peer": peerASN},
		"direction": direction,
	}

	switch msgType {
	case "state":
		// State is a simple string at bgp level (not a container)
		state, _ := bgpPayload["state"].(string)
		neighbor["state"] = state

	case "update":
		update := convertUpdateIPC2(eventData)
		if len(update) > 0 {
			neighbor["message"] = map[string]any{"update": update}
		}

	case "notification":
		// Fields in notification object
		neighbor["notification"] = map[string]any{
			"code":    eventData["code"],
			"subcode": eventData["subcode"],
			"data":    eventData["data"],
		}

	case "negotiated":
		// Fields in negotiated object
		result["negotiated"] = convertNegotiated(eventData)
	}

	result["neighbor"] = neighbor
	return result
}

// convertNegotiated converts ZeBGP negotiated caps to ExaBGP format.
//
// Key conversions:
//   - Family format: "ipv4/unicast" -> "ipv4 unicast".
//   - Field names: ZeBGP uses hyphens ("hold-time"), ExaBGP uses underscores ("hold_time").
func convertNegotiated(zebgp map[string]any) map[string]any {
	if zebgp == nil {
		return map[string]any{}
	}

	result := make(map[string]any)

	// Map ZeBGP hyphenated keys to ExaBGP underscored keys
	keyMapping := map[string]string{
		"hold-time": "hold_time",
		"asn4":      "asn4", // Same in both
	}
	for zebgpKey, exabgpKey := range keyMapping {
		if v, ok := zebgp[zebgpKey]; ok {
			result[exabgpKey] = v
		}
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

	// Extract attributes from "attr" object
	if attrObj, ok := eventData["attr"].(map[string]any); ok && len(attrObj) > 0 {
		update["attribute"] = attrObj
	}

	// Convert NLRI sections from "nlri" object
	announce := make(map[string]map[string][]any)
	withdraw := make(map[string][]any)

	if nlriObj, ok := eventData["nlri"].(map[string]any); ok {
		for family, value := range nlriObj {
			// Convert family: "ipv4/unicast" -> "ipv4 unicast"
			exabgpFamily := strings.ReplaceAll(family, "/", " ")

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
