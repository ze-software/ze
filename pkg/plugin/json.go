package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// JSONEncoder produces ExaBGP v6-compatible JSON output.
type JSONEncoder struct {
	version  string
	hostname string
	pid      int
	ppid     int

	// Per-peer message counters
	counters map[string]int
	mu       sync.Mutex

	// Time function (replaceable for testing)
	timeFunc func() time.Time
}

// NewJSONEncoder creates a new JSON encoder with the given API version.
func NewJSONEncoder(version string) *JSONEncoder {
	hostname, _ := os.Hostname()
	return &JSONEncoder{
		version:  version,
		hostname: hostname,
		pid:      os.Getpid(),
		ppid:     os.Getppid(),
		counters: make(map[string]int),
		timeFunc: time.Now,
	}
}

// SetHostname sets the hostname for JSON output (for testing).
func (e *JSONEncoder) SetHostname(hostname string) {
	e.hostname = hostname
}

// SetPID sets the PID and PPID for JSON output (for testing).
func (e *JSONEncoder) SetPID(pid, ppid int) {
	e.pid = pid
	e.ppid = ppid
}

// SetTimeFunc sets the time function (for testing).
func (e *JSONEncoder) SetTimeFunc(f func() time.Time) {
	e.timeFunc = f
}

// counter increments and returns the counter for a peer.
func (e *JSONEncoder) counter(peer PeerInfo) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := peer.Address.String()
	e.counters[key]++
	return e.counters[key]
}

// message creates the base message structure with "message" wrapper.
// The wrapper contains common fields: type, time.
// Caller sets "id" in the wrapper when msgID > 0.
func (e *JSONEncoder) message(peer PeerInfo, msgType string) map[string]any {
	now := e.timeFunc()
	return map[string]any{
		"zebgp":   e.version,
		"host":    e.hostname,
		"pid":     e.pid,
		"ppid":    e.ppid,
		"counter": e.counter(peer),
		"message": map[string]any{
			"type": msgType,
			"time": float64(now.UnixNano()) / 1e9,
		},
	}
}

// setMessageID sets the id field in the message wrapper if msgID > 0.
func setMessageID(msg map[string]any, msgID uint64) {
	if msgID > 0 {
		if wrapper, ok := msg["message"].(map[string]any); ok {
			wrapper["id"] = msgID
		}
	}
}

// peerSection creates the neighbor section of the message.
func (e *JSONEncoder) peerSection(peer PeerInfo) map[string]any {
	return map[string]any{
		"address": map[string]any{
			"local": peer.LocalAddress.String(),
			"peer":  peer.Address.String(),
		},
		"asn": map[string]any{
			"local": peer.LocalAS,
			"peer":  peer.PeerAS,
		},
	}
}

// StateUp returns JSON for a peer state "up" event.
func (e *JSONEncoder) StateUp(peer PeerInfo) string {
	msg := e.message(peer, "state")
	peerObj := e.peerSection(peer)
	peerObj["state"] = "up"
	msg["peer"] = peerObj
	return e.marshal(msg)
}

// StateDown returns JSON for a peer state "down" event.
func (e *JSONEncoder) StateDown(peer PeerInfo, reason string) string {
	msg := e.message(peer, "state")
	peerObj := e.peerSection(peer)
	peerObj["state"] = "down"
	peerObj["reason"] = reason
	msg["peer"] = peerObj
	return e.marshal(msg)
}

// StateConnected returns JSON for a peer "connected" event.
func (e *JSONEncoder) StateConnected(peer PeerInfo) string {
	msg := e.message(peer, "state")
	peerObj := e.peerSection(peer)
	peerObj["state"] = "connected"
	msg["peer"] = peerObj
	return e.marshal(msg)
}

// EOR returns JSON for an End-of-RIB marker.
func (e *JSONEncoder) EOR(peer PeerInfo, family string) string {
	msg := e.message(peer, "update")
	peerObj := e.peerSection(peer)

	peerObj["message"] = map[string]any{
		"eor": map[string]any{
			"afi":  family,
			"safi": SAFINameUnicast,
		},
	}
	msg["peer"] = peerObj
	return e.marshal(msg)
}

// Notification returns JSON for a NOTIFICATION message.
// ExaBGP fields: code, subcode, data (always present).
// ZeBGP extensions: code_name, subcode_name, message, direction, id in message wrapper.
func (e *JSONEncoder) Notification(peer PeerInfo, notify DecodedNotification, direction string, msgID uint64) string {
	msg := e.message(peer, "notification")
	msg["direction"] = direction
	setMessageID(msg, msgID)
	peerObj := e.peerSection(peer)

	// ExaBGP always includes data field (empty string if no data)
	dataHex := ""
	if len(notify.Data) > 0 {
		dataHex = fmt.Sprintf("%x", notify.Data)
	}

	notifyObj := map[string]any{
		"code":    notify.ErrorCode,
		"subcode": notify.ErrorSubcode,
		"data":    dataHex,
	}

	// ZeBGP extensions (use underscores for consistency)
	if notify.ErrorCodeName != "" {
		notifyObj["code_name"] = notify.ErrorCodeName
	}
	if notify.ErrorSubcodeName != "" {
		notifyObj["subcode_name"] = notify.ErrorSubcodeName
	}
	if notify.ShutdownMessage != "" {
		notifyObj["message"] = notify.ShutdownMessage
	}

	peerObj["notification"] = notifyObj
	msg["peer"] = peerObj
	return e.marshal(msg)
}

// Open returns JSON for an OPEN message.
// Field names match ExaBGP: hold_time, router_id, capabilities (underscores).
// Capabilities are structured as [{code, name, value}] for easy parsing.
func (e *JSONEncoder) Open(peer PeerInfo, open DecodedOpen, direction string, msgID uint64) string {
	msg := e.message(peer, "open")
	msg["direction"] = direction
	setMessageID(msg, msgID)
	peerObj := e.peerSection(peer)

	// Convert capabilities to structured JSON format
	caps := make([]map[string]any, 0, len(open.Capabilities))
	for _, cap := range open.Capabilities {
		capObj := map[string]any{
			"code": cap.Code,
			"name": cap.Name,
		}
		if cap.Value != "" {
			capObj["value"] = cap.Value
		}
		caps = append(caps, capObj)
	}

	peerObj["open"] = map[string]any{
		"version":      open.Version,
		"asn":          open.ASN,
		"hold_time":    open.HoldTime,
		"router_id":    open.RouterID,
		"capabilities": caps,
	}
	msg["peer"] = peerObj
	return e.marshal(msg)
}

// Keepalive returns JSON for a KEEPALIVE message.
func (e *JSONEncoder) Keepalive(peer PeerInfo, direction string, msgID uint64) string {
	msg := e.message(peer, "keepalive")
	msg["direction"] = direction
	setMessageID(msg, msgID)
	msg["peer"] = e.peerSection(peer)
	return e.marshal(msg)
}

// RouteRefresh returns JSON for a ROUTE-REFRESH message.
// RFC 7313: Type is "refresh" (subtype 0), "borr" (subtype 1), or "eorr" (subtype 2).
func (e *JSONEncoder) RouteRefresh(peer PeerInfo, decoded DecodedRouteRefresh, direction string, msgID uint64) string {
	// Use subtype name as event type for proper dispatch
	msg := e.message(peer, decoded.SubtypeName)
	msg["direction"] = direction
	setMessageID(msg, msgID)
	msg["peer"] = e.peerSection(peer)

	// Parse family "afi/safi" into separate fields
	if idx := strings.Index(decoded.Family, "/"); idx >= 0 {
		msg["afi"] = decoded.Family[:idx]
		msg["safi"] = decoded.Family[idx+1:]
	} else {
		// Fallback if format is unexpected
		msg["afi"] = decoded.Family
		msg["safi"] = ""
	}
	return e.marshal(msg)
}

// marshal converts a message to JSON string.
func (e *JSONEncoder) marshal(msg map[string]any) string {
	data, err := json.Marshal(msg)
	if err != nil {
		// Should never happen with our controlled input
		return `{"error":"json marshal failed"}`
	}
	return string(data)
}
