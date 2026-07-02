// Design: docs/architecture/api/process-protocol.md -- system-RIB event types
// Related: ../sysrib.go -- publishes BestChange; format must stay in sync

// Package events defines event constants and typed event handles for the
// system RIB plugin.
package events

import (
	"net/netip"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Namespace is the event namespace for the system RIB plugin.
const Namespace = "system-rib"

// System-RIB event types.
const (
	EventBestChange    = "best-change"    // sysrib published a system-wide best change
	EventReplayRequest = "replay-request" // downstream consumer asking sysrib to replay
)

// RouteType identifies the forwarding action for a FIB entry.
// Values match Linux RTN_ constants for direct mapping in the kernel backend.
type RouteType uint8

const (
	RouteTypeUnicast     RouteType = 1
	RouteTypeBlackhole   RouteType = 6
	RouteTypeUnreachable RouteType = 7
	RouteTypeProhibit    RouteType = 8
)

// ECMPPath is a single next-hop within an ECMP group.
type ECMPPath struct {
	NextHop netip.Addr `json:"next-hop"`
	Weight  uint8      `json:"weight,omitempty"`
	Labels  []uint32   `json:"labels,omitempty"`
}

// MaxECMPPaths is the maximum number of paths in an ECMP group.
const MaxECMPPaths = 128

// BestChangeEntry is one per-prefix entry in a BestChangeBatch. Action is the
// typed wire token; JSON serializes as "add"/"update"/"withdraw" via
// RouteAction.MarshalText, so FIB consumers that already parse the JSON form
// keep working unchanged.
type BestChangeEntry struct {
	Action    bgptypes.RouteAction `json:"action"`
	Prefix    netip.Prefix         `json:"prefix"`
	NextHop   netip.Addr           `json:"next-hop,omitzero"`
	Protocol  string               `json:"protocol"`
	Labels    []uint32             `json:"labels,omitempty"`
	RouteType RouteType            `json:"route-type,omitempty"`
	Metric    uint32               `json:"metric,omitempty"`
	TableID   uint32               `json:"table-id,omitempty"`
	SRv6SID   netip.Addr           `json:"srv6-sid,omitzero"`
	ECMPPaths []ECMPPath           `json:"ecmp-paths,omitempty"`
	// Backup carries pre-computed fast-reroute backup next-hop(s) (an IP FRR
	// alternate + optional MPLS repair label stack). Each is programmed by the FIB
	// as a link-down/backup next-hop, DISTINCT from ECMPPaths (which load-share):
	// the kernel forwards to a backup only when the primary link is down.
	Backup []ECMPPath `json:"backup,omitempty"`
}

// BestChangeBatch is the payload of (system-rib, best-change). One batch is
// emitted per family. The Replay flag distinguishes full-table replay from
// incremental changes.
type BestChangeBatch struct {
	Family  family.Family     `json:"family"`
	Replay  bool              `json:"replay,omitempty"`
	Changes []BestChangeEntry `json:"changes"`
}

// BestChange is the typed handle for (system-rib, best-change). FIB
// consumers (fibkernel, fibvpp, fibp4) subscribe via this handle.
var BestChange = events.Register[*BestChangeBatch](Namespace, EventBestChange)

// ReplayRequest is the typed handle for (system-rib, replay-request).
// Signal event with no payload.
var ReplayRequest = events.RegisterSignal(Namespace, EventReplayRequest)
