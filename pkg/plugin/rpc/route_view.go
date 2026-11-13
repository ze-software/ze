// Design: docs/architecture/api/process-protocol.md -- authoritative routing views.
// Related: ../sdk/sdk_route_view.go -- the public calls carrying these values.

package rpc

import (
	"encoding/json"
	"fmt"
	"math"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
)

// Routing-view operations are registered by their owner, not shared dispatch.
const (
	MethodRouteViewOpen       = "ze-sysrib:route-view-open"
	MethodRouteViewRead       = "ze-sysrib:route-view-read"
	MethodRouteViewCandidates = "ze-sysrib:route-view-candidates"
	MethodRouteViewDistances  = "ze-sysrib:route-view-distances"
	MethodRouteViewPublish    = "ze-sysrib:route-view-publish"
	MethodRouteViewClose      = "ze-sysrib:route-view-close"
)

// RouteViewResultBudget is the maximum encoded JSON result that fits a reply
// with any correlation ID. The producer MUST page before crossing this bound;
// Direct consumers MUST enforce it even though they do not write a frame.
func RouteViewResultBudget() int {
	return MaxMessageSize - len(AppendResult(nil, math.MaxUint64, json.RawMessage("0"))) + 1
}

// Page and wait bounds also apply to Direct calls, which have no frame reader.
const (
	RouteViewRecordsMax    = 4096
	RouteViewWaitMillisMax = 1000
)

// RouteViewPhase names what one selected-view page contains. Reset explicitly
// invalidates the consumer's projection; an empty delta never means reset.
type RouteViewPhase uint8

const (
	RouteViewPhaseUnspecified RouteViewPhase = iota
	RouteViewPhaseSnapshot
	RouteViewPhaseDelta
	RouteViewPhaseReset
)

// MarshalText refuses unspecified and unknown phases at the sending boundary.
func (phase RouteViewPhase) MarshalText() ([]byte, error) {
	switch phase {
	case RouteViewPhaseSnapshot:
		return []byte("snapshot"), nil
	case RouteViewPhaseDelta:
		return []byte("delta"), nil
	case RouteViewPhaseReset:
		return []byte("reset"), nil
	case RouteViewPhaseUnspecified:
		return nil, fmt.Errorf("routing-view phase is unspecified")
	default: // Open boundary: a caller can supply an unknown numeric phase.
		return nil, fmt.Errorf("unknown routing-view phase %d", phase)
	}
}

// UnmarshalText leaves the receiver unchanged when a wire phase is unknown.
func (phase *RouteViewPhase) UnmarshalText(text []byte) error {
	switch string(text) {
	case "snapshot":
		*phase = RouteViewPhaseSnapshot
	case "delta":
		*phase = RouteViewPhaseDelta
	case "reset":
		*phase = RouteViewPhaseReset
	default:
		return fmt.Errorf("unknown routing-view phase %q", text)
	}
	return nil
}

// UnmarshalJSON refuses null instead of allowing it to become an unspecified
// phase. Assignment happens only after the complete value has been validated.
func (phase *RouteViewPhase) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("routing-view phase cannot be null")
	}
	return phase.UnmarshalText([]byte(*value))
}

// RouteViewAction reuses the forwarding action vocabulary without a second
// numeric or string mapping. A selected-view handler permits add/update/withdraw.
type RouteViewAction = routeaction.Action

const (
	RouteViewActionAdd      = routeaction.Add
	RouteViewActionUpdate   = routeaction.Update
	RouteViewActionWithdraw = routeaction.Withdraw
)

// Reader choices do not grant identity or change permitted plugin placement.
const (
	RouteViewReaderSelectedFeed   = "selected-feed"
	RouteViewReaderLocalAuthority = "local-authority"
)

// RouteViewOpenInput chooses a reader. Empty Reader requests selected-feed.
// The authenticated process and generation own the session identity.
type RouteViewOpenInput struct {
	Reader string `json:"reader,omitempty"`
}

// RouteViewOpenOutput binds subsequent calls to one server epoch and session.
// Cursor zero is a valid initial frontier, distinct from a missing result.
type RouteViewOpenOutput struct {
	Session  string `json:"session"`
	Epoch    string `json:"epoch"`
	Snapshot string `json:"snapshot"`
	Cursor   uint64 `json:"cursor"`
}

// RouteViewReadInput requests a bounded page after the consumer's frontier.
// WaitMillis is a wire duration in milliseconds and cannot exceed its deadline.
type RouteViewReadInput struct {
	Session    string `json:"session"`
	Cursor     uint64 `json:"cursor"`
	Limit      uint32 `json:"limit"`
	WaitMillis uint32 `json:"wait-ms"`
}

// RouteViewReadOutput contains owned values from one ordered view. The producer
// MUST bound the complete encoded JSON to MaxMessageSize as well as the record
// count. A consumer MUST apply a complete snapshot before forwarding from it.
type RouteViewReadOutput struct {
	Session  string           `json:"session"`
	Epoch    string           `json:"epoch"`
	Snapshot string           `json:"snapshot,omitempty"`
	Cursor   uint64           `json:"cursor"`
	Phase    RouteViewPhase   `json:"phase"`
	Complete bool             `json:"complete"`
	Entries  []RouteViewEntry `json:"entries,omitempty"`
	// RejectedPublication names the exact validated publication refused before
	// emission by this lease's recoverable reset. Zero means no such proof.
	// The consumer MUST validate the reset and operation before reconstruction.
	RejectedPublication uint64 `json:"rejected-publication,omitempty"`
}

// RouteViewEntry updates or removes one prefix in the selected projection.
// Withdraw carries no Path. An entry never asks its reader to run selection.
type RouteViewEntry struct {
	AFI    uint16          `json:"afi"`
	SAFI   uint8           `json:"safi"`
	Prefix netip.Prefix    `json:"prefix"`
	Action RouteViewAction `json:"action"`
	Path   RouteViewPath   `json:"path,omitzero"`
}

// RouteViewNextHop carries one equal-cost forwarding target. Its metadata stays
// with the target when a surviving member replaces an unreachable primary.
type RouteViewNextHop struct {
	// Protocol names a different owner from the primary. Empty explicitly
	// means the primary's protocol, including targets the primary Path carries.
	Protocol  string     `json:"protocol,omitempty"`
	NextHop   netip.Addr `json:"next-hop,omitzero"`
	Interface string     `json:"interface,omitempty"`
	OnLink    bool       `json:"on-link,omitempty"`
	Weight    uint8      `json:"weight,omitempty"`
	Labels    []uint32   `json:"labels,omitempty"`
	SRv6SID   netip.Addr `json:"srv6-sid,omitzero"`
}

// RouteViewPath is the selected path's self-contained value form. Protocol is
// a registered name, never a process-local ID. Distance is already resolved by
// the authority. HasTag and AIGPPresent distinguish absence from a real zero.
// The producer MUST transfer owned slices. The consumer MUST retain no borrowed
// Loc-RIB slice or forward handle through these values.
type RouteViewPath struct {
	Protocol           string             `json:"protocol"`
	Instance           uint32             `json:"instance"`
	Distance           uint8              `json:"distance"`
	Metric             uint32             `json:"metric"`
	NextHop            netip.Addr         `json:"next-hop,omitzero"`
	Interface          string             `json:"interface,omitempty"`
	OnLink             bool               `json:"on-link,omitempty"`
	Weight             uint8              `json:"weight,omitempty"`
	Labels             []uint32           `json:"labels,omitempty"`
	RouteType          uint8              `json:"route-type,omitempty"`
	SRv6SID            netip.Addr         `json:"srv6-sid,omitzero"`
	IsEBGP             bool               `json:"is-ebgp,omitempty"`
	IsBGP              bool               `json:"is-bgp,omitempty"`
	AIGP               uint64             `json:"aigp,omitempty"`
	AIGPPresent        bool               `json:"aigp-present,omitempty"`
	MetricRecursive    bool               `json:"metric-recursive,omitempty"`
	Tag                uint32             `json:"tag,omitempty"`
	HasTag             bool               `json:"has-tag,omitempty"`
	Level              uint8              `json:"level,omitempty"`
	OSPFRouteType      uint8              `json:"ospf-route-type,omitempty"`
	ECMP               []RouteViewNextHop `json:"ecmp,omitempty"`
	BackupNextHop      netip.Addr         `json:"backup-next-hop,omitzero"`
	BackupRepairLabels []uint32           `json:"backup-repair-labels,omitempty"`
	BackupInterface    string             `json:"backup-interface,omitempty"`
	BackupOnLink       bool               `json:"backup-on-link,omitempty"`
}

// RouteViewCandidatesInput names a separately pinned candidate snapshot.
// Empty Snapshot opens a scan. A zero family or invalid Prefix leaves that
// selector absent. A candidate snapshot never uses the selected-view cursor.
type RouteViewCandidatesInput struct {
	Session  string       `json:"session"`
	Snapshot string       `json:"snapshot,omitempty"`
	Cursor   uint64       `json:"cursor"`
	Limit    uint32       `json:"limit"`
	AFI      uint16       `json:"afi,omitempty"`
	SAFI     uint8        `json:"safi,omitempty"`
	Prefix   netip.Prefix `json:"prefix,omitzero"`
}

// RouteViewCandidatesOutput preserves the owner's existing candidate JSON.
// Each entry is one complete rendered prefix, including hidden paths. The
// producer MUST enforce both page bounds before replying. The consumer MUST
// treat entries as values, not as another selection or policy input.
type RouteViewCandidatesOutput struct {
	Session  string            `json:"session"`
	Epoch    string            `json:"epoch"`
	Snapshot string            `json:"snapshot"`
	Cursor   uint64            `json:"cursor"`
	Complete bool              `json:"complete"`
	Entries  []json.RawMessage `json:"entries,omitempty"`
}

// RouteViewDistance is one protocol-wide distance declaration. Zero is valid
// for a connected declaration; validation applies the owner's protocol rules.
type RouteViewDistance struct {
	Protocol string `json:"protocol"`
	Distance uint8  `json:"distance"`
}

// RouteViewDistancesInput publishes a complete table under an ordered identity.
// A retry MUST carry the same operation and values. The receiver MUST reject
// a reused operation with different values or an older operation.
type RouteViewDistancesInput struct {
	Session   string              `json:"session"`
	Operation uint64              `json:"operation"`
	Distances []RouteViewDistance `json:"distances"`
}

// RouteViewDistancesOutput acknowledges authoritative application/reselection.
type RouteViewDistancesOutput struct {
	Session   string `json:"session"`
	Operation uint64 `json:"operation"`
	Cursor    uint64 `json:"cursor"`
}

// RouteViewPublishInput wraps an unchanged system best-change batch. Raw JSON
// keeps public SDK code independent of the owning component's event type.
// The sender MUST retain its operation until acknowledged. The receiver MUST
// fence stale generations before emission and retain idempotent acknowledgments.
type RouteViewPublishInput struct {
	Session   string          `json:"session"`
	Cursor    uint64          `json:"cursor"`
	Operation uint64          `json:"operation"`
	Batch     json.RawMessage `json:"batch"`
}

// RouteViewPublishOutput acknowledges one emitted forwarding batch.
type RouteViewPublishOutput struct {
	Session   string `json:"session"`
	Operation uint64 `json:"operation"`
	Cursor    uint64 `json:"cursor"`
}

// RouteViewCloseInput closes only the authenticated caller's session.
type RouteViewCloseInput struct {
	Session string `json:"session"`
}

// RouteViewCloseOutput acknowledges the fenced session after scoped cleanup.
type RouteViewCloseOutput struct {
	Session string `json:"session"`
}
