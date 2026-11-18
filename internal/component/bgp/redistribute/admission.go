// Design: docs/architecture/api/process-protocol.md -- accepted redistribution input.
// Related: consumer.go -- the sole BGP-facing adapter after configured acceptance.
package redistribute

import (
	"context"
	"errors"
	"math"
	"net/netip"
	"slices"
	"sort"
	"sync"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const bgpAcceptedRecordsMax = 1 << 20

// One current source has at most one all-peer record or concrete peer records.
// Zero instance/table are explicit aggregates, not missing source identities.
type bgpAcceptedKey struct {
	family   family.Family
	protocol redistevents.ProtocolID
	instance uint32
	table    uint32
	prefix   netip.Prefix
}

// The data is immutable after publication. Only state changes under records.mu.
// The SDK reply owns its community copy; no election lock or I/O occurs here.
type bgpAcceptedRecord struct {
	key      bgpAcceptedKey
	id       uint64
	snapshot rpc.OriginAdmissionSnapshot
	state    rpc.OriginAdmissionRecordState
	peer     netip.Addr
}

// This is the BGP adapter's accepted input, not another candidate RIB. One
// mutation gate orders the existing update-route calls and is cancellable.
// The separate leaf mutex protects only published accepted data, never RPC.
// Bounds: current accepted source/scope records plus their actual in-flight
// predecessor, capped at bgpAcceptedRecordsMax. Completed records leave no history.
// Each factory constructs exactly once before SDK.Run and publishes at Started;
// its ID clock never resets during that authenticated connection's lifetime.
type bgpAcceptedRecords struct {
	mu        sync.RWMutex
	mutations chan struct{}
	next      uint64
	groups    map[bgpAcceptedKey]map[netip.Addr]*bgpAcceptedRecord
	ids       map[uint64]*bgpAcceptedRecord
}

func newBGPAcceptedRecords() *bgpAcceptedRecords {
	return &bgpAcceptedRecords{mutations: make(chan struct{}, 1), groups: make(map[bgpAcceptedKey]map[netip.Addr]*bgpAcceptedRecord), ids: make(map[uint64]*bgpAcceptedRecord)}
}

// begin MUST be followed by release after every callback and final result joined.
func (records *bgpAcceptedRecords) begin(ctx context.Context) (func(), error) {
	select {
	case records.mutations <- struct{}{}:
		return func() { <-records.mutations }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func bgpAcceptedSource(fam family.Family, entry configredist.RouteEntry) (bgpAcceptedKey, rpc.OriginAdmissionSnapshot, error) {
	prefix, err := netip.ParsePrefix(entry.Prefix)
	if err != nil {
		return bgpAcceptedKey{}, rpc.OriginAdmissionSnapshot{}, err
	}
	if prefix != prefix.Masked() {
		return bgpAcceptedKey{}, rpc.OriginAdmissionSnapshot{}, errors.New("accepted BGP redistribution prefix has host bits; use its original network prefix")
	}
	protocol := redistevents.ProtocolName(entry.Protocol)
	if protocol == "" {
		return bgpAcceptedKey{}, rpc.OriginAdmissionSnapshot{}, errors.New("accepted BGP redistribution has no original protocol; retain its configured source before formatting")
	}
	if redistevents.WouldLoop(protocol, bgpConsumerName) {
		return bgpAcceptedKey{}, rpc.OriginAdmissionSnapshot{}, errors.New("BGP redistribution cannot import a BGP route into BGP")
	}
	scope := rpc.OriginAdmissionAllPeers
	if entry.Peer != "" {
		scope = rpc.OriginAdmissionExactPeer
	}
	snapshot := rpc.OriginAdmissionSnapshot{Protocol: protocol, SourceInstance: entry.SourceInstance, Table: entry.Table, SourceMetric: entry.SourceMetric,
		DistanceOverride: entry.DistanceOverride, HasDistanceOverride: entry.HasDistanceOverride, NextHop: entry.NextHop, OriginASN: entry.OriginASN,
		Community: slices.Clone(entry.Community), Tag: entry.Tag, Metric: entry.Metric, HasMetric: entry.HasMetric, RuleGeneration: entry.AdmissionGeneration,
		PeerScope: scope, Peer: entry.Peer}
	if fam == family.IPv4Unicast {
		if !prefix.Addr().Is4() {
			return bgpAcceptedKey{}, rpc.OriginAdmissionSnapshot{}, errors.New("accepted IPv4 source has an IPv6 prefix")
		}
	} else if fam == family.IPv6Unicast {
		if !prefix.Addr().Is6() {
			return bgpAcceptedKey{}, rpc.OriginAdmissionSnapshot{}, errors.New("accepted IPv6 source has an IPv4 prefix")
		}
	} else {
		return bgpAcceptedKey{}, rpc.OriginAdmissionSnapshot{}, errors.New("accepted redistribution requires an IPv4 or IPv6 unicast prefix")
	}
	if err := snapshot.Validate(entry.Peer); err != nil {
		return bgpAcceptedKey{}, rpc.OriginAdmissionSnapshot{}, err
	}
	return bgpAcceptedKey{family: fam, protocol: entry.Protocol, instance: entry.SourceInstance, table: entry.Table, prefix: prefix}, snapshot, nil
}

// offer publishes before engine RPC so the independent callback can read it.
// Its caller MUST hold begin through the result, then finish replaced records.
func (records *bgpAcceptedRecords) offer(fam family.Family, entry configredist.RouteEntry) (*bgpAcceptedRecord, []*bgpAcceptedRecord, error) {
	key, snapshot, err := bgpAcceptedSource(fam, entry)
	if err != nil {
		return nil, nil, err
	}
	peer := netip.Addr{}
	if entry.Peer != "" {
		peer, err = netip.ParseAddr(entry.Peer)
		if err != nil {
			return nil, nil, err
		}
	}
	records.mu.Lock()
	defer records.mu.Unlock()
	group := records.groups[key]
	if group != nil {
		if all := group[netip.Addr{}]; all != nil {
			if entry.Peer != "" {
				snapshot.PeerScope = rpc.OriginAdmissionAllPeers
				snapshot.Peer = ""
				if all.state == rpc.OriginAdmissionCurrent {
					if snapshot.RuleGeneration >= all.snapshot.RuleGeneration {
						// An unchanged reload sends no new admission. Reuse its
						// immutable original only when every source/build fact
						// still matches; a replay cannot move its generation.
						snapshot.RuleGeneration = all.snapshot.RuleGeneration
						if all.snapshot.Equal(snapshot) {
							return all, nil, nil
						}
					}
				}
				// A targeted refresh cannot replace the accepted all-peer source
				// with a changed build or older evaluator; a full offer owns that.
				return nil, nil, errors.New("targeted BGP replay differs from its accepted all-peer source; replay the current accepted contribution")
			}
		}
		if old := group[peer]; old != nil {
			if old.state == rpc.OriginAdmissionCurrent {
				if old.snapshot.Equal(snapshot) {
					return old, nil, nil
				}
			}
		}
	}
	if len(records.ids) >= bgpAcceptedRecordsMax {
		return nil, nil, errors.New("accepted BGP redistribution record limit reached; withdraw an existing source before admitting another")
	}
	if records.next == math.MaxUint64 {
		return nil, nil, errors.New("accepted BGP redistribution identities exhausted; restart the accepting process before another admission")
	}
	records.next++
	record := &bgpAcceptedRecord{key: key, id: records.next, snapshot: snapshot, state: rpc.OriginAdmissionCurrent, peer: peer}
	if group == nil {
		group = make(map[netip.Addr]*bgpAcceptedRecord)
		records.groups[key] = group
	}
	var replaced []*bgpAcceptedRecord
	if !peer.IsValid() {
		// Only this original source's admitted concrete scopes are inspected.
		for _, old := range group {
			old.state = rpc.OriginAdmissionRetiring
			replaced = append(replaced, old)
		}
		clear(group)
	} else if old := group[peer]; old != nil {
		old.state = rpc.OriginAdmissionRetiring
		replaced = append(replaced, old)
	}
	group[peer] = record
	records.ids[record.id] = record
	return record, replaced, nil
}

// finish MUST follow the actual joined result for this replacement. The old
// exact IDs were withdraw-only while that call owned them; no history remains.
func (records *bgpAcceptedRecords) finish(replaced []*bgpAcceptedRecord) {
	records.mu.Lock()
	defer records.mu.Unlock()
	for _, record := range replaced {
		delete(records.ids, record.id)
	}
}

func (records *bgpAcceptedRecords) retiring(fam family.Family, removal configredist.RouteRemoval) ([]*bgpAcceptedRecord, error) {
	prefix, err := netip.ParsePrefix(removal.Prefix)
	if err != nil {
		return nil, err
	}
	if redistevents.ProtocolName(removal.Protocol) == "" {
		return nil, errors.New("BGP withdrawal has no original source protocol; pass the accepted contribution identity")
	}
	key := bgpAcceptedKey{family: fam, protocol: removal.Protocol, instance: removal.SourceInstance, table: removal.Table, prefix: prefix}
	records.mu.Lock()
	defer records.mu.Unlock()
	group := records.groups[key]
	out := make([]*bgpAcceptedRecord, 0, len(group))
	for _, record := range group {
		record.state = rpc.OriginAdmissionRetiring
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out, nil
}

func (records *bgpAcceptedRecords) withdrawn(record *bgpAcceptedRecord) {
	records.mu.Lock()
	defer records.mu.Unlock()
	peer := record.peer
	group := records.groups[record.key]
	if group[peer] == record {
		delete(group, peer)
	}
	if len(group) == 0 {
		delete(records.groups, record.key)
	}
	delete(records.ids, record.id)
}

func (records *bgpAcceptedRecords) lookup(ctx context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	records.mu.RLock()
	record := records.ids[input.AdmissionID]
	if record == nil {
		records.mu.RUnlock()
		return nil, errors.New("accepted BGP redistribution record is absent; use its current original accepted ID")
	}
	snapshot := record.snapshot
	state := record.state
	fam := record.key.family
	prefix := record.key.prefix
	snapshot.Community = slices.Clone(snapshot.Community)
	records.mu.RUnlock()
	if input.Family != fam.String() {
		return nil, errors.New("accepted BGP redistribution family differs")
	}
	if input.Prefix != prefix.String() {
		return nil, errors.New("accepted BGP redistribution prefix differs")
	}
	output := &rpc.OriginAdmissionLookupOutput{AdmissionID: input.AdmissionID, Family: input.Family, Prefix: input.Prefix, Destination: input.Destination, PeerOwner: input.PeerOwner, Operation: input.Operation, State: state, Snapshot: snapshot}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return output, nil
}
