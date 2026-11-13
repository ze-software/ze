// Design: docs/architecture/rib/unified-locrib.md -- authority-backed forwarding fixtures.

package sysrib

import (
	"context"
	"maps"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Typed producer fixtures retain the BGP observation contract as input data;
// only real authority Insert/Remove can deliver it to the selected consumer.
type incomingBatch = ribevents.BestChangeBatch
type incomingChange = ribevents.BestChangeEntry

// fixtureRank states whether the input describes a protocol's historical stamp
// or an explicit route override. Only the real Loc-RIB resolves either rank.
type fixtureRank uint8

const (
	fixtureRankUnspecified fixtureRank = iota
	fixtureStampedRank
	fixtureRouteOverride
)

// candidateForwardingFixture translates typed producer inputs into real
// authority candidates. It never selects candidates or assigns consumer slots.
// Its owner MUST let testing cleanup close the exact authority session.
type candidateForwardingFixture struct {
	*sysRIB
	t         *testing.T
	rib       *locrib.RIB
	authority routeViewAuthority
	owner     *process.Process
	session   string
	operation uint64
	rank      fixtureRank
}

func newCandidateForwardingFixture(t *testing.T, rank fixtureRank) *candidateForwardingFixture {
	t.Helper()
	if rank != fixtureStampedRank && rank != fixtureRouteOverride {
		t.Fatal("candidate fixture needs an explicit rank interpretation")
	}
	rib := getLocRIB()
	if rib == nil {
		rib = locrib.NewRIB()
	}
	fixture := &candidateForwardingFixture{sysRIB: newSysRIB(), t: t, rib: rib, rank: rank, owner: routeViewOwner()}
	if resolver := getNHResolver(); resolver != nil {
		fixture.resolver.Store(resolver)
	} else {
		fixture.resolver.Store(newNHResolver(rib))
	}
	opened, err := fixture.authority.open(t.Context(), fixture.owner, rib,
		rpc.RouteViewOpenInput{Reader: rpc.RouteViewReaderLocalAuthority})
	if err != nil {
		t.Fatal(err)
	}
	fixture.session = opened.Session
	t.Cleanup(func() {
		if err := fixture.authority.cleanup(fixture.owner); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(func() { distance.Set(nil) })
	fixture.acknowledgeDistances(nil)
	return fixture
}

// acknowledgeDistances applies one complete declaration through the exact
// owner/session before copying selected values into the common forwarder.
func (fixture *candidateForwardingFixture) acknowledgeDistances(changes map[string]int) map[family.Family][]outgoingChange {
	fixture.t.Helper()
	declared, err := parseAdminDistanceConfig("{}")
	if err != nil {
		fixture.t.Fatal(err)
	}
	maps.Copy(declared, changes)
	fixture.operation++
	input := rpc.RouteViewDistancesInput{Session: fixture.session, Operation: fixture.operation}
	for protocol, rank := range declared {
		if rank < 0 || rank > 255 {
			fixture.t.Fatalf("candidate fixture distance %s=%d exceeds uint8", protocol, rank)
		}
		input.Distances = append(input.Distances, rpc.RouteViewDistance{Protocol: protocol, Distance: uint8(rank)})
	}
	ack, err := fixture.authority.applyDistances(context.Background(), fixture.owner, input)
	if err != nil {
		fixture.t.Fatal(err)
	}
	if ack.Session != fixture.session || ack.Operation != fixture.operation || ack.Cursor != 0 {
		fixture.t.Fatalf("candidate fixture received a foreign or incomplete declaration acknowledgment: %+v", ack)
	}
	fixture.mu.Lock()
	fixture.adminDist = maps.Clone(declared)
	fixture.mu.Unlock()
	keys := make(map[prefixKey]struct{})
	// Re-evaluate the prefixes this algorithm fixture already consumed. Reader
	// covers inserted by its topology remain real authority state; consuming
	// them here would invent FIB inputs absent from the original fixture. The
	// worker fixtures separately consume the authority's complete selected view.
	fixture.mu.RLock()
	for key := range fixture.routes {
		keys[key] = struct{}{}
	}
	fixture.mu.RUnlock()
	return fixture.forwardSelected(keys)
}

func (fixture *candidateForwardingFixture) offerCandidates(batch *incomingBatch) (family.Family, []outgoingChange) {
	fixture.t.Helper()
	if batch == nil || batch.Family == (family.Family{}) {
		fixture.t.Fatal("candidate fixture needs a typed batch and family")
	}
	keys := make(map[prefixKey]struct{})
	for _, change := range batch.Changes {
		if !change.Prefix.IsValid() || change.Prefix != change.Prefix.Masked() {
			fixture.t.Fatalf("candidate fixture has no canonical prefix: %s", change.Prefix)
		}
		key := prefixKey{family: batch.Family, prefix: change.Prefix}
		keys[key] = struct{}{}
		switch change.Action {
		case routeaction.Withdraw:
			if batch.Protocol == "" {
				var candidates []locrib.Path
				fixture.rib.Inspect(batch.Family, change.Prefix, func(group locrib.PathGroup) {
					candidates = append(candidates, group.Paths...)
				})
				for _, candidate := range candidates {
					fixture.rib.Remove(batch.Family, change.Prefix, candidate.Source, candidate.Instance)
				}
			} else {
				source, ok := redistevents.ProtocolIDOf(batch.Protocol)
				if !ok {
					fixture.t.Fatalf("candidate fixture withdrawal has no producer %q", batch.Protocol)
				}
				fixture.rib.Remove(batch.Family, change.Prefix, source, 0)
			}
		case routeaction.Add, routeaction.Update:
			fixture.rib.Insert(batch.Family, change.Prefix, fixture.candidatePath(batch.Protocol, change))
		case routeaction.Unspecified, routeaction.Del:
			fixture.t.Fatalf("candidate fixture has no offered route action: %s", change.Action)
		default:
			fixture.t.Fatalf("candidate fixture has an unknown action: %s", change.Action)
		}
	}
	batches := fixture.forwardSelected(keys)
	return batch.Family, batches[batch.Family]
}

func (fixture *candidateForwardingFixture) candidatePath(protocol string, change incomingChange) locrib.Path {
	fixture.t.Helper()
	if protocol == "" {
		fixture.t.Fatal("candidate fixture add has no producer")
	}
	path := locrib.Path{Source: redistevents.RegisterProtocol(protocol), NextHop: change.NextHop,
		Interface: change.Interface, OnLink: change.OnLink, Weight: change.Weight,
		Metric: change.Metric, Labels: change.Labels, SRv6SID: change.SRv6SID, RouteType: change.RouteType,
		ECMP: change.ECMPNextHops, BackupNextHop: change.BackupNextHop, BackupRepairLabels: change.BackupRepairLabels,
		BackupInterface: change.BackupInterface, BackupOnLink: change.BackupOnLink,
		IsBGP: protocol == "bgp", IsEBGP: protocol == "bgp" && change.ProtocolType != routeaction.ProtocolIBGP,
	}
	if fixture.rank == fixtureRouteOverride {
		if change.Priority < 0 || change.Priority > 255 {
			fixture.t.Fatalf("candidate fixture route override %d exceeds uint8", change.Priority)
		}
		path.DistanceOverride = uint8(change.Priority)
		path.HasDistanceOverride = true
	}
	return path
}

func (fixture *candidateForwardingFixture) forwardSelected(keys map[prefixKey]struct{}) map[family.Family][]outgoingChange {
	fixture.t.Helper()
	selected := make([]routeViewSelectedChange, 0, len(keys))
	for key := range keys {
		var prepared routeViewSelectedChange = routeViewSelectedRemove{key: key}
		var err error
		fixture.rib.Inspect(key.family, key.prefix, func(group locrib.PathGroup) {
			prepared, err = prepareRouteViewLocal(key.family, key.prefix, &group)
		})
		if err != nil {
			fixture.t.Fatal(err)
		}
		selected = append(selected, prepared)
	}
	batches, err := fixture.applySelectedBatch(selected)
	if err != nil {
		fixture.t.Fatal(err)
	}
	return batches
}
