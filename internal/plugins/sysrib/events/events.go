// Design: docs/architecture/api/process-protocol.md -- system-RIB event types
// Related: ../sysrib.go -- publishes BestChange; format must stay in sync

// Package events defines event constants and typed event handles for the
// system RIB plugin.
package events

import (
	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

// Namespace is the event namespace for the system RIB plugin.
const Namespace = "system-rib"

// System-RIB event types.
const (
	EventBestChange    = "best-change"    // sysrib published a system-wide best change
	EventReplayRequest = "replay-request" // downstream consumer asking sysrib to replay
)

// BestChangeEntry is one per-prefix entry in a BestChangeBatch.
type BestChangeEntry struct {
	Action   string `json:"action"`
	Prefix   string `json:"prefix"`
	NextHop  string `json:"next-hop,omitempty"`
	Protocol string `json:"protocol"`
}

// BestChangeBatch is the payload of (system-rib, best-change). One batch is
// emitted per family. The Replay flag distinguishes full-table replay from
// incremental changes.
type BestChangeBatch struct {
	Family  string            `json:"family"`
	Replay  bool              `json:"replay,omitempty"`
	Changes []BestChangeEntry `json:"changes"`
}

// BestChange is the typed handle for (system-rib, best-change). FIB
// consumers (fibkernel, fibvpp, fibp4) subscribe via this handle.
var BestChange = events.Register[*BestChangeBatch](Namespace, EventBestChange)

// ReplayRequest is the typed handle for (system-rib, replay-request).
// Signal event with no payload.
var ReplayRequest = events.RegisterSignal(Namespace, EventReplayRequest)
