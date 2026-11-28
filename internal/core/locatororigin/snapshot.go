// Design: docs/architecture/isis/isis-4-component-config.md -- locator-originator EventBus snapshot contract
// Related: index.go -- read-locked in-daemon index updated from snapshots
//
// Package locatororigin defines the leaf EventBus contract and read-locked
// in-daemon index mapping SRv6 locator prefixes to their IGP originating
// router's IPv6 and IPv4 addresses and prefixes. It imports only the Go standard library
// and internal/core/events so BGP can consult IGP locator originators with zero
// RPC on the receive hot path and zero package coupling to IS-IS.
package locatororigin

import (
	"encoding/json"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/events"
)

const (
	// Namespace is the EventBus namespace for IGP locator-originator snapshots.
	Namespace = "locator-origin"
	// EventSnapshot carries a *Snapshot replacing a source's full locator set.
	EventSnapshot = "snapshot"
	// EventSnapshotRequest asks active sources to replay their latest snapshot.
	EventSnapshotRequest = "snapshot-request"
)

// SnapshotEvent carries a source's full locator-originator snapshot across the
// EventBus. External and internal plugins emit compact JSON via
// sdk.Plugin.EmitEvent; pluginserver.deliverEvent decodes it into *Snapshot for
// in-daemon subscribers.
var SnapshotEvent = events.Register[*Snapshot](Namespace, EventSnapshot)

// SnapshotRequest asks every active locator-originator source to publish its
// current snapshot again so a late or restarted consumer reconciles immediately.
var SnapshotRequest = events.RegisterSignal(Namespace, EventSnapshotRequest)

// ReasonClass categorizes why a locator originator could not be resolved. Its
// zero value (ReasonNone) marks a resolved entry; every unresolved outcome
// carries a non-zero class so callers can build bounded diagnostic subjects.
type ReasonClass uint8

const (
	// ReasonNone indicates no unresolved condition.
	ReasonNone ReasonClass = iota
	// ReasonNoActiveSource means no IGP locator-originator source is active.
	ReasonNoActiveSource
	// ReasonNoCoveringLocator means no advertised SRv6 locator covers the SID.
	ReasonNoCoveringLocator
	// ReasonAnycast means the locator is anycast (A-flag set or multiple originators).
	ReasonAnycast
	// ReasonReadvertised means every covering locator is re-advertised (R or X set)
	// with no matching IPv6 Source Router ID.
	ReasonReadvertised
	// ReasonMissingFlags means the covering locator carries no Prefix Attribute Flags
	// sub-TLV (RFC 9352 Section 7.1).
	ReasonMissingFlags
	// ReasonUnsupportedNextHopLen means the MP_REACH_NLRI next hop length is not
	// 4, 16, or 32 octets.
	ReasonUnsupportedNextHopLen
	// ReasonUndecodedTLV237 means the originator's LSP carries TLV 237 (MT IPv6
	// reachability) which core IS-IS packet decoding does not cover for host
	// enumeration.
	ReasonUndecodedTLV237
	// ReasonNoSRv6SID means the Prefix-SID attribute carries no SRv6 Service SID.
	ReasonNoSRv6SID
)

// String returns the stable kebab-case identifier of c.
func (c ReasonClass) String() string {
	switch c {
	case ReasonNone:
		return "none"
	case ReasonNoActiveSource:
		return "no-active-source"
	case ReasonNoCoveringLocator:
		return "no-covering-locator"
	case ReasonAnycast:
		return "anycast"
	case ReasonReadvertised:
		return "re-advertised"
	case ReasonMissingFlags:
		return "missing-flags"
	case ReasonUnsupportedNextHopLen:
		return "unsupported-next-hop-length"
	case ReasonUndecodedTLV237:
		return "undecoded-tlv237"
	case ReasonNoSRv6SID:
		return "no-srv6-sid"
	default:
		panic("BUG: locatororigin: invalid ReasonClass")
	}
}

// MarshalJSON encodes ReasonClass as its stable kebab-case label.
func (c ReasonClass) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// UnmarshalJSON decodes a ReasonClass from its kebab-case label.
func (c *ReasonClass) UnmarshalJSON(data []byte) error {
	var label string
	if err := json.Unmarshal(data, &label); err != nil {
		return err
	}
	switch label {
	case "", "none":
		*c = ReasonNone
	case "no-active-source":
		*c = ReasonNoActiveSource
	case "no-covering-locator":
		*c = ReasonNoCoveringLocator
	case "anycast":
		*c = ReasonAnycast
	case "re-advertised":
		*c = ReasonReadvertised
	case "missing-flags":
		*c = ReasonMissingFlags
	case "unsupported-next-hop-length":
		*c = ReasonUnsupportedNextHopLen
	case "undecoded-tlv237":
		*c = ReasonUndecodedTLV237
	case "no-srv6-sid":
		*c = ReasonNoSRv6SID
	default:
		return errors.New("locatororigin: unknown reason class " + label)
	}
	return nil
}

// MatchOutcome is the verdict of comparing a BGP next hop or ISD prefix against
// a resolved locator originator. Zero is invalid so an uninitialized verdict
// cannot pass for a real match.
type MatchOutcome uint8

const (
	// MatchUnspecified is the invalid zero value.
	MatchUnspecified MatchOutcome = iota
	// MatchKnown means the compared address or prefix belongs to the locator's
	// proven originator (or matches a covering IPv6 Source Router ID sub-TLV).
	MatchKnown
	// MatchMismatch means the locator originator is proven and does not hold the
	// compared address or prefix.
	MatchMismatch
	// MatchUnknown means the originator could not be proven; callers keep the
	// route and report the named reason.
	MatchUnknown
)

// Snapshot is one complete, self-contained locator-originator view published by
// an IGP source. It carries no pointers into producer state so it crosses both
// DirectBridge and TLS RPC identically.
//
// Generation increases monotonically per running source; subscribers drop any
// snapshot whose Generation is less than or equal to the last applied generation
// for that source. Active false marks a graceful shutdown or unconfigured IGP
// instance, removing the source's locators while retaining its generation floor.
type Snapshot struct {
	Source     string         `json:"source"`
	Generation uint64         `json:"generation"`
	Active     bool           `json:"active"`
	Entries    []LocatorEntry `json:"entries,omitempty"`
}

// LocatorEntry describes the resolved originator state for one advertised SRv6
// locator prefix across a source's link-state database.
type LocatorEntry struct {
	Prefix             netip.Prefix   `json:"prefix"`
	Known              bool           `json:"known"`
	Originator         string         `json:"originator,omitempty"`
	Addresses          []netip.Addr   `json:"addresses,omitempty"`
	Prefixes           []netip.Prefix `json:"prefixes,omitempty"`
	SourceRouterIDs    []netip.Addr   `json:"source-router-ids,omitempty"`
	HasUndecodedTLV237 bool           `json:"has-undecoded-tlv237,omitempty"`
	ReasonClass        ReasonClass    `json:"reason-class,omitempty"`
	Reason             string         `json:"reason,omitempty"`
}

// Result is the self-contained value returned by Index.Resolve for one SRv6 SID.
// Either Known is true (Originator and its advertised IPv6 and IPv4 Addresses/Prefixes
// are populated) or Known is false with a non-zero ReasonClass and named Reason
// (and optional SourceRouterIDs for re-advertised locators that carry RFC 7794
// Section 2.2 IPv6 Source Router ID sub-TLVs).
type Result struct {
	Prefix             netip.Prefix
	Known              bool
	Originator         string
	Addresses          []netip.Addr
	Prefixes           []netip.Prefix
	SourceRouterIDs    []netip.Addr
	HasUndecodedTLV237 bool
	ReasonClass        ReasonClass
	Reason             string
}
