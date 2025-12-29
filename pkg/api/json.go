package api

import (
	"encoding/json"
	"fmt"
	"os"
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

// message creates the base message structure.
func (e *JSONEncoder) message(peer PeerInfo, msgType string) map[string]any {
	now := e.timeFunc()
	return map[string]any{
		"zebgp":   e.version,
		"time":    float64(now.UnixNano()) / 1e9,
		"host":    e.hostname,
		"pid":     e.pid,
		"ppid":    e.ppid,
		"counter": e.counter(peer),
		"type":    msgType,
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

// RouteAnnounce returns JSON for a route announcement.
// Format matches ExaBGP v6 "update" message.
func (e *JSONEncoder) RouteAnnounce(peer PeerInfo, routes []RouteUpdate) string {
	return e.routeAnnounceInternal(peer, routes, "")
}

// RouteAnnounceWithRaw returns JSON for a route announcement including raw wire bytes.
// Used for format=full which includes both parsed content and raw hex.
func (e *JSONEncoder) RouteAnnounceWithRaw(peer PeerInfo, routes []RouteUpdate, rawHex string) string {
	return e.routeAnnounceInternal(peer, routes, rawHex)
}

// routeAnnounceInternal builds route announcement JSON, optionally with raw bytes.
func (e *JSONEncoder) routeAnnounceInternal(peer PeerInfo, routes []RouteUpdate, rawHex string) string {
	msg := e.message(peer, "update")
	peerObj := e.peerSection(peer)

	// Build announce section
	announce := make(map[string]any)
	for _, r := range routes {
		family := r.Family()
		if announce[family] == nil {
			announce[family] = make(map[string]any)
		}
		familyMap := announce[family].(map[string]any) //nolint:forcetypeassert // Type guaranteed by construction above

		nhStr := r.NextHop
		if familyMap[nhStr] == nil {
			familyMap[nhStr] = make(map[string]any)
		}
		nhMap := familyMap[nhStr].(map[string]any) //nolint:forcetypeassert // Type guaranteed by construction above
		nhMap[r.Prefix] = r.Attributes()
	}

	peerObj["message"] = map[string]any{
		"update": map[string]any{
			"announce": announce,
		},
	}
	msg["peer"] = peerObj

	// Include raw bytes if provided (format=full)
	if rawHex != "" {
		msg["raw"] = rawHex
	}

	return e.marshal(msg)
}

// RouteWithdraw returns JSON for a route withdrawal.
// Format matches ExaBGP v6 "update" message with withdraw section.
func (e *JSONEncoder) RouteWithdraw(peer PeerInfo, routes []RouteUpdate) string {
	msg := e.message(peer, "update")
	peerObj := e.peerSection(peer)

	// Build withdraw section
	withdraw := make(map[string]any)
	for _, r := range routes {
		family := r.Family()
		if withdraw[family] == nil {
			withdraw[family] = []string{}
		}
		prefixes := withdraw[family].([]string) //nolint:forcetypeassert // Type guaranteed by construction above
		withdraw[family] = append(prefixes, r.Prefix)
	}

	peerObj["message"] = map[string]any{
		"update": map[string]any{
			"withdraw": withdraw,
		},
	}
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
// ZeBGP extensions: code_name, subcode_name, message.
func (e *JSONEncoder) Notification(peer PeerInfo, notify DecodedNotification) string {
	msg := e.message(peer, "notification")
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

// Open returns JSON for an OPEN message received.
// Field names match ExaBGP: hold_time, router_id, capabilities (underscores).
func (e *JSONEncoder) Open(peer PeerInfo, open DecodedOpen) string {
	msg := e.message(peer, "open")
	peerObj := e.peerSection(peer)

	// Ensure capabilities is always an array (empty if none)
	caps := open.Capabilities
	if caps == nil {
		caps = []string{}
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
func (e *JSONEncoder) Keepalive(peer PeerInfo) string {
	msg := e.message(peer, "keepalive")
	msg["peer"] = e.peerSection(peer)
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

// RouteUpdate represents a route for JSON encoding.
type RouteUpdate struct {
	Prefix  string // e.g., "10.0.0.0/24"
	NextHop string // e.g., "192.168.1.1"
	AFI     string // "ipv4" or "ipv6"
	SAFI    string // "unicast", "multicast", "flowspec", etc.

	// Optional attributes
	Origin          string   // "igp", "egp", "incomplete"
	ASPath          []uint32 // AS path sequence
	LocalPref       uint32
	MED             uint32
	Communities     []string // e.g., ["65000:100", "no-export"]
	LargeCommunity  []string // e.g., ["65000:1:1"]
	ExtCommunity    []string // Extended communities
	AtomicAggregate bool
}

// Family returns the address family string for JSON.
func (r *RouteUpdate) Family() string {
	if r.AFI == "" {
		r.AFI = AFINameIPv4
	}
	if r.SAFI == "" {
		r.SAFI = SAFINameUnicast
	}
	return r.AFI + " " + r.SAFI
}

// Attributes returns the attribute map for JSON.
func (r *RouteUpdate) Attributes() map[string]any {
	attrs := make(map[string]any)

	if r.Origin != "" {
		attrs["origin"] = r.Origin
	}
	if len(r.ASPath) > 0 {
		attrs["as-path"] = r.ASPath
	}
	if r.LocalPref > 0 {
		attrs["local-preference"] = r.LocalPref
	}
	if r.MED > 0 {
		attrs["med"] = r.MED
	}
	if len(r.Communities) > 0 {
		attrs["community"] = r.Communities
	}
	if len(r.LargeCommunity) > 0 {
		attrs["large-community"] = r.LargeCommunity
	}
	if len(r.ExtCommunity) > 0 {
		attrs["extended-community"] = r.ExtCommunity
	}
	if r.AtomicAggregate {
		attrs["atomic-aggregate"] = true
	}

	// If empty, return empty object (ExaBGP style)
	if len(attrs) == 0 {
		return map[string]any{}
	}
	return attrs
}
