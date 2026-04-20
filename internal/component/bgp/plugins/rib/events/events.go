// Design: docs/architecture/api/process-protocol.md -- BGP-RIB event types
// Related: ../rib_bestchange.go -- publishes BestChange; format must stay in sync

// Package events defines event constants and typed event handles for the
// BGP RIB plugin.
//
// Engine-side producers and consumers use the typed handles (BestChange,
// ReplayRequest). External plugin processes receive JSON marshaling of the
// same types; json tags on the payload struct are the contract with them.
package events

import (
	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

// Namespace is the event namespace for the BGP RIB plugin.
const Namespace = "bgp-rib"

// RIB event types. These constants remain so code that still references
// them by name (external plugin authors, registries) keeps compiling.
const (
	EventCache         = "cache"
	EventRoute         = "route"
	EventBestChange    = "best-change"    // protocol RIB published a best-path change
	EventReplayRequest = "replay-request" // downstream consumer asking for full table replay
)

// BestChangeAction values for BestChangeEntry.Action.
const (
	BestChangeAdd      = "add"
	BestChangeUpdate   = "update"
	BestChangeWithdraw = "withdraw"
)

// BestChangeEntry is one per-prefix entry in a BestChangeBatch. Json tags
// define the wire format delivered to external plugin processes.
//
// AddPath flags whether the entry came from an ADD-PATH-negotiated family
// (RFC 7911). Consumers MUST read AddPath before interpreting PathID --
// PathID=0 is a valid identifier under ADD-PATH, and the `omitempty` tag
// elides it from JSON, so without AddPath the subscriber cannot tell
// "non-ADD-PATH" from "ADD-PATH with pathID=0". AddPath is always emitted
// for ADD-PATH entries (including pathID=0) and omitted for everything
// else.
type BestChangeEntry struct {
	Action       string `json:"action"`
	Prefix       string `json:"prefix"`
	AddPath      bool   `json:"add-path,omitempty"`
	PathID       uint32 `json:"path-id,omitempty"`
	NextHop      string `json:"next-hop,omitempty"`
	Priority     int    `json:"priority"`
	Metric       uint32 `json:"metric"`
	ProtocolType string `json:"protocol-type,omitempty"`
}

// BestChangeBatch is the payload of (bgp-rib, best-change). One batch is
// emitted per (protocol, family) combination. The Replay flag distinguishes
// a full-table replay batch from an incremental change batch.
type BestChangeBatch struct {
	Protocol string            `json:"protocol"`         // always "bgp" for the BGP RIB plugin
	Family   string            `json:"family"`           // e.g. "ipv4/unicast"
	Replay   bool              `json:"replay,omitempty"` // true for full-table replay batches
	Changes  []BestChangeEntry `json:"changes"`
}

// BestChange is the typed handle for (bgp-rib, best-change). Producers call
// BestChange.Emit(bus, batch); consumers call BestChange.Subscribe(bus, h).
var BestChange = events.Register[*BestChangeBatch](Namespace, EventBestChange)

// ReplayRequest is the typed handle for (bgp-rib, replay-request). Signal
// event with no payload — downstream consumers (e.g., sysrib) use it to
// request a full-table replay on startup.
var ReplayRequest = events.RegisterSignal(Namespace, EventReplayRequest)
