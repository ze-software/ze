// Design: docs/architecture/api/process-protocol.md -- BGP-RIB event types
// Related: ../../../component/bgp/plugins/rib/rib_bestchange.go -- publishes BestChange; format must stay in sync

// Package ribevents defines the (bgp-rib, ...) event constants and typed event
// handles: the best-path change contract between the BGP RIB and everything
// downstream of it.
//
// Engine-side producers and consumers use the typed handles (BestChange,
// ReplayRequest). External plugin processes receive JSON marshaling of the
// same types; json tags on the payload struct are the contract with them.
//
// It lives in internal/core because the CONSUMER side is always-on: sysrib
// arbitrates the Loc-RIB and flow-export enriches flows from best-change
// batches, and both must keep compiling when the BGP engine is compiled out
// (//go:build ze_bgp). Only the producer (internal/component/bgp/plugins/rib)
// is gated; with the engine absent nothing emits on the handle and the
// subscribers simply never fire.
package ribevents

import (
	"encoding/json"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
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

// BestChangeAction values for BestChangeEntry.Action. Aliases on the typed
// routeaction.Action so consumers that already imported these names keep
// compiling. Wire form ("add"/"update"/"withdraw") is preserved via
// RouteAction.MarshalText.
const (
	BestChangeAdd      = routeaction.Add
	BestChangeUpdate   = routeaction.Update
	BestChangeWithdraw = routeaction.Withdraw
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
	Action       routeaction.Action       `json:"action"`
	Prefix       netip.Prefix             `json:"prefix"`
	AddPath      bool                     `json:"add-path,omitempty"`
	PathID       uint32                   `json:"path-id,omitempty"`
	NextHop      netip.Addr               `json:"next-hop,omitzero"`
	Priority     int                      `json:"priority"`
	Metric       uint32                   `json:"metric"`
	ProtocolType routeaction.ProtocolType `json:"protocol-type,omitempty"`
	Labels       []uint32                 `json:"labels,omitempty"`
	SRv6SID      netip.Addr               `json:"srv6-sid,omitzero"`
	// OriginAS is the last ASN in the best path's AS_PATH (the route's origin);
	// ASPath is the full path. Both are populated on add/update from the winning
	// candidate, and are empty on withdraw and on full-table replay. Consumers
	// such as flow-export enrichment use them for bgpSource/DestinationAsNumber.
	OriginAS uint32   `json:"origin-as,omitempty"`
	ASPath   []uint32 `json:"as-path,omitempty"`
	// ECMPNextHops are the in-process Loc-RIB intra-source equal-cost sibling
	// next-hops of this entry's best, carried from sysrib's changeToBatch
	// (computed at Loc-RIB emit on locrib.Change.ECMP). sysrib uses them to
	// build the kernel multipath without re-reading the PathGroup. The forked
	// (cross-process) event-bus path has no Loc-RIB and leaves it nil; json:"-"
	// because it is an in-process hint only, never part of the wire contract.
	ECMPNextHops []netip.Addr `json:"-"`

	// BackupNextHop and BackupRepairLabels carry a pre-computed fast-reroute
	// backup next-hop (an IP fast-reroute alternate) plus its optional MPLS repair
	// label stack, from locrib.Path. In-process hint only (json:"-"): sysrib
	// forwards it as a DEDICATED backup next-hop, never an ECMP sibling.
	BackupNextHop      netip.Addr `json:"-"`
	BackupRepairLabels []uint32   `json:"-"`
}

// BestChangeBatch is the payload of (bgp-rib, best-change). One batch is
// emitted per (protocol, family) combination. IsReplay() distinguishes a
// full-table replay batch from an incremental change batch.
type BestChangeBatch struct {
	Protocol string        `json:"protocol"` // always "bgp" for the BGP RIB plugin
	Family   family.Family `json:"family"`   // typed family; JSON "ipv4/unicast" etc. via MarshalText
	// ReplayID is the replay correlation token echoed from the replay request.
	// For this broadcast hop it is 0 (incremental) or replay.Broadcast
	// (full-table replay). It is the single source of truth for the replay
	// marker; the historical `replay` boolean wire tag is derived from it in
	// MarshalJSON, so external plugin processes see an unchanged JSON contract
	// while in-process code has no second field to keep consistent (json:"-").
	ReplayID uint64            `json:"-"`
	Changes  []BestChangeEntry `json:"changes"`
	// FromLocRIB marks a batch built in-process from a unified Loc-RIB Change
	// (sysrib's changeToBatch), as opposed to an independent per-protocol event.
	// The Loc-RIB has already arbitrated across every source and emits exactly
	// one authoritative best per prefix, so the consumer replaces (not upserts)
	// its per-prefix entry and never re-arbitrates Loc-RIB sources by admin
	// distance. json:"-" because it is an in-process hint only: the cross-process
	// (forked-plugin) event-bus path never sets it and decodes it as false, which
	// correctly selects the legacy per-protocol arbitration.
	FromLocRIB bool `json:"-"`
}

// IsReplay reports whether this batch is a full-table replay (nonzero token)
// rather than an incremental change, via the shared replay predicate.
func (b *BestChangeBatch) IsReplay() bool { return replay.IsReplay(b.ReplayID) }

// MarshalJSON preserves the historical `replay` boolean wire tag (external
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

// BestChange is the typed handle for (bgp-rib, best-change). Producers call
// BestChange.Emit(bus, batch); consumers call BestChange.Subscribe(bus, h).
var BestChange = events.Register[*BestChangeBatch](Namespace, EventBestChange)

// ReplayRequest is the typed handle for (bgp-rib, replay-request). It carries
// the shared replay.Request; downstream consumers (e.g., sysrib) emit it with
// replay.Broadcast to request a full-table replay on startup. The handler
// ignores the token because this hop is broadcast (every subscriber receives
// the full table); the payload exists so all three replay hops share one
// request vocabulary.
var ReplayRequest = events.Register[*replay.Request](Namespace, EventReplayRequest)
