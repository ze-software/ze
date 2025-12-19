package api

import (
	"encoding/json"
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

	// Per-neighbor message counters
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
		"exabgp":  e.version,
		"time":    float64(now.UnixNano()) / 1e9,
		"host":    e.hostname,
		"pid":     e.pid,
		"ppid":    e.ppid,
		"counter": e.counter(peer),
		"type":    msgType,
	}
}

// neighborSection creates the neighbor section of the message.
func (e *JSONEncoder) neighborSection(peer PeerInfo) map[string]any {
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
	neighbor := e.neighborSection(peer)
	neighbor["state"] = "up"
	msg["neighbor"] = neighbor
	return e.marshal(msg)
}

// StateDown returns JSON for a peer state "down" event.
func (e *JSONEncoder) StateDown(peer PeerInfo, reason string) string {
	msg := e.message(peer, "state")
	neighbor := e.neighborSection(peer)
	neighbor["state"] = "down"
	neighbor["reason"] = reason
	msg["neighbor"] = neighbor
	return e.marshal(msg)
}

// StateConnected returns JSON for a peer "connected" event.
func (e *JSONEncoder) StateConnected(peer PeerInfo) string {
	msg := e.message(peer, "state")
	neighbor := e.neighborSection(peer)
	neighbor["state"] = "connected"
	msg["neighbor"] = neighbor
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
