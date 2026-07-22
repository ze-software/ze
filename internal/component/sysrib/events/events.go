// Design: docs/architecture/api/process-protocol.md -- system-RIB event types
// Related: ../sysrib.go -- publishes BestChange; format must stay in sync

// Package events defines event constants and typed event handles for the
// system RIB plugin.
package events

import (
	"encoding/json"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/routeaction"

	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/replay"
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
	Action    routeaction.Action `json:"action"`
	Prefix    netip.Prefix       `json:"prefix"`
	NextHop   netip.Addr         `json:"next-hop,omitzero"`
	Protocol  string             `json:"protocol"`
	Labels    []uint32           `json:"labels,omitempty"`
	RouteType RouteType          `json:"route-type,omitempty"`
	Metric    uint32             `json:"metric,omitempty"`
	TableID   uint32             `json:"table-id,omitempty"`
	SRv6SID   netip.Addr         `json:"srv6-sid,omitzero"`
	ECMPPaths []ECMPPath         `json:"ecmp-paths,omitempty"`
	// Backup carries pre-computed fast-reroute backup next-hop(s) (an IP FRR
	// alternate + optional MPLS repair label stack). Each is programmed by the FIB
	// as a link-down/backup next-hop, DISTINCT from ECMPPaths (which load-share):
	// the kernel forwards to a backup only when the primary link is down.
	Backup []ECMPPath `json:"backup,omitempty"`
}

// BestChangeBatch is the payload of (system-rib, best-change). One batch is
// emitted per family. IsReplay() distinguishes full-table replay from
// incremental changes.
type BestChangeBatch struct {
	Family family.Family `json:"family"`
	// ReplayID is the replay correlation token echoed from the replay request.
	// For this broadcast hop it is 0 (incremental) or replay.Broadcast
	// (full-table replay). It is the single source of truth for the replay
	// marker; the historical `replay` boolean wire tag is derived from it in
	// MarshalJSON, so external FIB plugin processes see an unchanged JSON
	// contract while in-process code has no second field to keep consistent.
	ReplayID uint64            `json:"-"`
	Changes  []BestChangeEntry `json:"changes"`
}

// IsReplay reports whether this batch is a full-table replay (nonzero token)
// rather than an incremental change, via the shared replay predicate.
func (b *BestChangeBatch) IsReplay() bool { return replay.IsReplay(b.ReplayID) }

// MarshalJSON preserves the historical `replay` boolean wire tag (external FIB
// plugin processes decode it) while keeping ReplayID as the single source of
// truth: `replay` is emitted only for a replay batch and omitted otherwise.
// The `alias` type strips the method set so json.Marshal does not recurse.
func (b *BestChangeBatch) MarshalJSON() ([]byte, error) {
	type alias BestChangeBatch
	return json.Marshal(struct {
		*alias
		Replay bool `json:"replay,omitempty"`
	}{alias: (*alias)(b), Replay: b.IsReplay()})
}

// UnmarshalJSON maps the historical `replay` boolean back onto the token so a
// decoded batch reports IsReplay() correctly (round-trip symmetry with
// MarshalJSON). A true `replay` decodes to the broadcast token.
func (b *BestChangeBatch) UnmarshalJSON(data []byte) error {
	type alias BestChangeBatch
	aux := struct {
		*alias
		Replay bool `json:"replay,omitempty"`
	}{alias: (*alias)(b)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Replay {
		b.ReplayID = replay.Broadcast
	}
	return nil
}

// BestChange is the typed handle for (system-rib, best-change). FIB
// consumers (fibkernel, fibvpp, fibp4) subscribe via this handle.
var BestChange = events.Register[*BestChangeBatch](Namespace, EventBestChange)

// ReplayRequest is the typed handle for (system-rib, replay-request). It
// carries the shared replay.Request; FIB backends emit it with replay.Broadcast
// to request a full-table replay on startup. The handler ignores the token
// because this hop is broadcast; the payload exists so all three replay hops
// share one request vocabulary.
var ReplayRequest = events.Register[*replay.Request](Namespace, EventReplayRequest)
