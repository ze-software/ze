// Design: docs/architecture/api/process-protocol.md -- original redistribution admission lookup.
// Related: current_selection.go -- independent read-only callback transport.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"
)

const MethodOriginAdmissionLookup = "ze-plugin-callback:origin-admission-lookup"

// OriginAdmissionCommunityMax is the public YANG community cardinality. The
// encoded response also has its separate transport frame bound.
const OriginAdmissionCommunityMax = 65535

const originAdmissionBGPProtocol = "bgp"

// OriginAdmissionRecordState is the actual accepted record's current lifecycle.
// Retiring records can answer exact removals but never authorize a new offer.
type OriginAdmissionRecordState uint8

const (
	OriginAdmissionRecordUnspecified OriginAdmissionRecordState = iota
	OriginAdmissionCurrent
	OriginAdmissionRetiring
)

// OriginAdmissionPeerScope is explicit. An omitted peer is never a wildcard.
type OriginAdmissionPeerScope uint8

const (
	OriginAdmissionPeerScopeUnspecified OriginAdmissionPeerScope = iota
	OriginAdmissionAllPeers
	OriginAdmissionExactPeer
)

// OriginAdmissionBGPClass preserves the original BGP source's preference class.
// Non-BGP sources use Unspecified; the protocol remains a separate fact.
type OriginAdmissionBGPClass uint8

const (
	OriginAdmissionBGPClassUnspecified OriginAdmissionBGPClass = iota
	OriginAdmissionIBGP
	OriginAdmissionEBGP
)

// OriginAdmissionLookupInput is raw callback input. AdmissionID is one opaque
// accepted-record identity, not an origin receipt or caller-supplied provenance.
// The caller MUST use the original accepting connection and separately check
// its process incarnation, actual grants and current concrete Peer publication.
type OriginAdmissionLookupInput struct {
	AdmissionID uint64                `json:"admission-id"`
	Family      string                `json:"family"`
	Prefix      string                `json:"prefix"`
	Destination string                `json:"destination"`
	PeerOwner   uint64                `json:"peer-owner"`
	Operation   OriginIntakeOperation `json:"operation"`
	DeadlineMS  int64                 `json:"deadline-ms"`
}

// OriginAdmissionSnapshot is descriptive original accepted data, never proof
// that a publisher, process, rule or peer is authorized. SourceMetric is the
// producer's pre-transform preference metric; Metric is optional transformed
// wire MED only when HasMetric is true. Zero distance and rule generation are
// valid explicit values. Empty NextHop preserves the existing local self choice.
// Output ownership, including Community, MUST pass to the receiving caller.
type OriginAdmissionSnapshot struct {
	Protocol            string                   `json:"protocol"`
	BGPClass            OriginAdmissionBGPClass  `json:"bgp-class"`
	SourceInstance      uint32                   `json:"source-instance"`
	Table               uint32                   `json:"table"`
	SourceMetric        uint32                   `json:"source-metric"`
	DistanceOverride    uint8                    `json:"distance-override"`
	HasDistanceOverride bool                     `json:"has-distance-override"`
	NextHop             string                   `json:"next-hop"`
	OriginASN           uint32                   `json:"origin-asn"`
	Community           []uint32                 `json:"community,omitempty"`
	Tag                 uint32                   `json:"tag"`
	Metric              uint32                   `json:"metric"`
	HasMetric           bool                     `json:"has-metric"`
	RuleGeneration      uint64                   `json:"rule-generation"`
	PeerScope           OriginAdmissionPeerScope `json:"peer-scope"`
	Peer                string                   `json:"peer"`
}

// OriginAdmissionLookupOutput is a complete successful snapshot for one exact
// query. Missing, stale and no-longer-accepted records return an error, never an
// empty snapshot. A retiring record answers only exact withdrawal queries until
// its cleanup joins; it MUST NOT authorize another announcement.
type OriginAdmissionLookupOutput struct {
	AdmissionID uint64                     `json:"admission-id"`
	Family      string                     `json:"family"`
	Prefix      string                     `json:"prefix"`
	Destination string                     `json:"destination"`
	PeerOwner   uint64                     `json:"peer-owner"`
	Operation   OriginIntakeOperation      `json:"operation"`
	State       OriginAdmissionRecordState `json:"state"`
	Snapshot    OriginAdmissionSnapshot    `json:"snapshot"`
}

func originAdmissionLookupFields(data []byte, names ...string) error {
	if len(data) > MaxMessageSize {
		return errors.New("origin-admission-lookup value exceeds the frame limit; split the request")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range names {
		value, exists := fields[name]
		if !exists {
			return fmt.Errorf("origin-admission-lookup requires %s; send the complete callback value", name)
		}
		if bytes.Equal(value, []byte("null")) {
			return fmt.Errorf("origin-admission-lookup %s is null; send an explicit value", name)
		}
	}
	return nil
}

// UnmarshalJSON distinguishes explicit valid zeros from missing facts.
func (input *OriginAdmissionLookupInput) UnmarshalJSON(data []byte) error {
	if err := originAdmissionLookupFields(data, "admission-id", "family", "prefix", "destination", "peer-owner", "operation", "deadline-ms"); err != nil {
		return err
	}
	type raw OriginAdmissionLookupInput
	var decoded raw
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*input = OriginAdmissionLookupInput(decoded)
	return nil
}

// UnmarshalJSON requires every original scalar, including explicit false values.
func (snapshot *OriginAdmissionSnapshot) UnmarshalJSON(data []byte) error {
	if err := originAdmissionLookupFields(data, "protocol", "bgp-class", "source-instance", "table", "source-metric", "distance-override", "has-distance-override", "next-hop", "origin-asn", "tag", "metric", "has-metric", "rule-generation", "peer-scope", "peer"); err != nil {
		return err
	}
	type raw OriginAdmissionSnapshot
	var decoded raw
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*snapshot = OriginAdmissionSnapshot(decoded)
	return nil
}

// UnmarshalJSON requires the complete echo and the accepted snapshot.
func (output *OriginAdmissionLookupOutput) UnmarshalJSON(data []byte) error {
	if err := originAdmissionLookupFields(data, "admission-id", "family", "prefix", "destination", "peer-owner", "operation", "state", "snapshot"); err != nil {
		return err
	}
	type raw OriginAdmissionLookupOutput
	var decoded raw
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*output = OriginAdmissionLookupOutput(decoded)
	return nil
}

// Validate checks the public value on both transports, not source authority.
func (input OriginAdmissionLookupInput) Validate(now time.Time) error {
	if input.AdmissionID == 0 {
		return errors.New("origin-admission-lookup ID is zero; use the original accepted record ID")
	}
	if input.PeerOwner == 0 {
		return errors.New("origin-admission-lookup Peer owner is zero; use the concrete configured Peer identity")
	}
	if err := originAdmissionAddress(input.Destination); err != nil {
		return err
	}
	prefix, err := netip.ParsePrefix(input.Prefix)
	if err != nil {
		return fmt.Errorf("origin-admission-lookup prefix: %w", err)
	}
	if prefix.String() != input.Prefix {
		return errors.New("origin-admission-lookup prefix is not canonical; use the original canonical source prefix")
	}
	if prefix.Masked() != prefix {
		return errors.New("origin-admission-lookup prefix has host bits; use the original network prefix")
	}
	// This callback serves the current unicast-prefix redistribution feed. The
	// public family set is open: other native families require their own carrier.
	switch input.Family {
	case "ipv4/unicast":
		if !prefix.Addr().Is4() {
			return errors.New("origin-admission-lookup IPv4 family has an IPv6 prefix; use the original family")
		}
	case "ipv6/unicast":
		if !prefix.Addr().Is6() {
			return errors.New("origin-admission-lookup IPv6 family has an IPv4 prefix; use the original family")
		}
		if prefix.Addr().Is4In6() {
			return errors.New("origin-admission-lookup IPv6 prefix is IPv4-mapped; use the original IPv4 prefix and family")
		}
	default:
		return errors.New("origin-admission-lookup family is not a unicast prefix family; use the original IPv4 or IPv6 unicast family")
	}
	// The operation is external and unknown values are refused.
	switch input.Operation {
	case OriginIntakeAnnounce, OriginIntakeWithdraw:
	case OriginIntakeUnspecified:
		return errors.New("origin-admission-lookup operation is unspecified; use announce or withdraw")
	default:
		return errors.New("origin-admission-lookup operation is unknown; use announce or withdraw")
	}
	if input.DeadlineMS <= now.UnixMilli() {
		return errors.New("origin-admission-lookup deadline expired; retry with a live deadline")
	}
	if input.DeadlineMS > now.Add(CancellableRequestTimeout).UnixMilli() {
		return errors.New("origin-admission-lookup deadline exceeds thirty seconds; use a bounded lookup")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if idPrefixMax+len(MethodOriginAdmissionLookup)+1+len(encoded) > MaxMessageSize {
		return errors.New("origin-admission-lookup request exceeds the frame limit; split the request")
	}
	return nil
}

func originAdmissionAddress(address string) error {
	if len(address) > 256 {
		return errors.New("origin-admission-lookup peer address is too long; use the original bounded peer address")
	}
	parsed, err := netip.ParseAddr(address)
	if err != nil {
		return fmt.Errorf("origin-admission-lookup peer address: %w", err)
	}
	if parsed.String() != address {
		return errors.New("origin-admission-lookup peer address is not canonical; use the original peer address")
	}
	return nil
}

// Validate requires the exact original query and explicit source/scope facts.
// The authenticated engine caller MUST additionally verify the captured process,
// actual accepted record lifetime, send grant and its matching immutable build.
func (output *OriginAdmissionLookupOutput) Validate(input OriginAdmissionLookupInput) error {
	if output == nil {
		return errors.New("origin-admission-lookup reply is missing; return the complete accepted snapshot")
	}
	if output.AdmissionID != input.AdmissionID {
		return errors.New("origin-admission-lookup ID differs; echo the original accepted ID")
	}
	if output.Family != input.Family {
		return errors.New("origin-admission-lookup family differs; echo the original family")
	}
	if output.Prefix != input.Prefix {
		return errors.New("origin-admission-lookup prefix differs; echo the original native prefix")
	}
	if output.Destination != input.Destination {
		return errors.New("origin-admission-lookup destination differs; echo the concrete queried peer")
	}
	if output.PeerOwner != input.PeerOwner {
		return errors.New("origin-admission-lookup Peer owner differs; echo the concrete configured Peer identity")
	}
	if output.Operation != input.Operation {
		return errors.New("origin-admission-lookup operation differs; echo the requested operation")
	}
	// State is external; missing and unknown records cannot grant authority.
	switch output.State {
	case OriginAdmissionCurrent:
	case OriginAdmissionRetiring:
		if input.Operation != OriginIntakeWithdraw {
			return errors.New("origin-admission-lookup record is retiring; use it only for its exact withdrawal")
		}
	case OriginAdmissionRecordUnspecified:
		return errors.New("origin-admission-lookup record state is unspecified; return its current accepted or retiring state")
	default:
		return errors.New("origin-admission-lookup record state is unknown; return its current accepted or retiring state")
	}
	if err := output.Snapshot.Validate(input.Destination); err != nil {
		return err
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return err
	}
	if idPrefixMax+len("ok ")+len(encoded) > MaxMessageSize {
		return errors.New("origin-admission-lookup reply exceeds the frame limit; use a bounded accepted route")
	}
	return nil
}

// OriginAdmissionLookupCallContext stamps an absolute bounded read deadline.
// The caller MUST call cancel. This read has no remote mutation to cancel.
func OriginAdmissionLookupCallContext(ctx context.Context, input *OriginAdmissionLookupInput) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(ctx, CancellableRequestTimeout)
	if input.DeadlineMS > 0 {
		deadline, _ := work.Deadline()
		if time.UnixMilli(input.DeadlineMS).Before(deadline) {
			cancel()
			work, cancel = context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
		}
	}
	deadline, _ := work.Deadline()
	input.DeadlineMS = deadline.UnixMilli()
	return work, cancel
}

// OriginAdmissionLookup asks this exact connection, never a name-resolved peer.
func (mux *MuxConn) OriginAdmissionLookup(ctx context.Context, input OriginAdmissionLookupInput) (*OriginAdmissionLookupOutput, error) {
	ctx, cancel := OriginAdmissionLookupCallContext(ctx, &input)
	defer cancel()
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	data, err := mux.CallRPC(ctx, MethodOriginAdmissionLookup, input)
	if err != nil {
		return nil, err
	}
	var output OriginAdmissionLookupOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("origin-admission-lookup reply: %w", err)
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return &output, nil
}

// Validate checks accepted source facts without inventing a query, peer owner
// or permission. Exact scope is checked against destination; an all-peer
// snapshot uses no destination at construction and is checked on each lookup.
func (snapshot OriginAdmissionSnapshot) Validate(destination string) error {
	if snapshot.Protocol == "" {
		return errors.New("origin-admission-lookup protocol is absent; return the original producer protocol")
	}
	if len(snapshot.Protocol) > 64 {
		return errors.New("origin-admission-lookup protocol is too long; return the registered protocol name")
	}
	for _, value := range snapshot.Protocol {
		if strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789-", value) {
			continue
		}
		return errors.New("origin-admission-lookup protocol has invalid characters; return the registered lowercase protocol name")
	}
	// Class is external. The original protocol and BGP class are separate facts.
	switch snapshot.BGPClass {
	case OriginAdmissionBGPClassUnspecified:
		if snapshot.Protocol == originAdmissionBGPProtocol {
			return errors.New("origin-admission-lookup BGP class is unspecified; retain original internal or external source class")
		}
	case OriginAdmissionIBGP, OriginAdmissionEBGP:
		if snapshot.Protocol != originAdmissionBGPProtocol {
			return errors.New("origin-admission-lookup BGP class contradicts protocol; preserve the original protocol and class")
		}
	default:
		return errors.New("origin-admission-lookup BGP class is unknown; return the original source class")
	}
	if snapshot.NextHop != "" {
		if err := originAdmissionAddress(snapshot.NextHop); err != nil {
			return err
		}
	}
	// Peer scope is external; zero or an omitted peer cannot imply fan-out.
	switch snapshot.PeerScope {
	case OriginAdmissionAllPeers:
		if snapshot.Peer != "" {
			return errors.New("origin-admission-lookup all-peer scope contains a peer; return one consistent scope")
		}
	case OriginAdmissionExactPeer:
		if err := originAdmissionAddress(snapshot.Peer); err != nil {
			return err
		}
		if snapshot.Peer != destination {
			return errors.New("origin-admission-lookup accepted scope names another peer; use a record admitted for this peer")
		}
	case OriginAdmissionPeerScopeUnspecified:
		return errors.New("origin-admission-lookup peer scope is unspecified; return explicit all or exact scope")
	default:
		return errors.New("origin-admission-lookup peer scope is unknown; return explicit all or exact scope")
	}
	if len(snapshot.Community) > OriginAdmissionCommunityMax {
		return fmt.Errorf("origin-admission-lookup communities exceed %d; return a route within the declared callback cardinality", OriginAdmissionCommunityMax)
	}
	return nil
}

// Equal compares every immutable accepted fact, including ordered community
// ownership. Equality is descriptive; it grants no process or peer authority.
func (snapshot OriginAdmissionSnapshot) Equal(other OriginAdmissionSnapshot) bool {
	if snapshot.Protocol != other.Protocol {
		return false
	}
	if snapshot.BGPClass != other.BGPClass {
		return false
	}
	if snapshot.SourceInstance != other.SourceInstance {
		return false
	}
	if snapshot.Table != other.Table {
		return false
	}
	if snapshot.SourceMetric != other.SourceMetric {
		return false
	}
	if snapshot.DistanceOverride != other.DistanceOverride {
		return false
	}
	if snapshot.HasDistanceOverride != other.HasDistanceOverride {
		return false
	}
	if snapshot.NextHop != other.NextHop {
		return false
	}
	if snapshot.OriginASN != other.OriginASN {
		return false
	}
	if snapshot.Tag != other.Tag {
		return false
	}
	if snapshot.Metric != other.Metric {
		return false
	}
	if snapshot.HasMetric != other.HasMetric {
		return false
	}
	if snapshot.RuleGeneration != other.RuleGeneration {
		return false
	}
	if snapshot.PeerScope != other.PeerScope {
		return false
	}
	if snapshot.Peer != other.Peer {
		return false
	}
	return slices.Equal(snapshot.Community, other.Community)
}
