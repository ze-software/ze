// Design: docs/architecture/core-design.md -- independent IPsec route producers
// Related: redistribute.go -- Child SA source identity and rekey transfer

package engine

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/slogutil"

	"github.com/stretchr/testify/require"
)

// sourceIdentityBus owns snapshots of actual producer events. The test reads
// their source membership; it makes no BGP acceptance claim.
type sourceIdentityBus struct {
	entries []redistevents.RouteChangeEntry
}

func (b *sourceIdentityBus) Emit(_, _ string, payload any) (int, error) {
	batch, ok := payload.(*redistevents.RouteChangeBatch)
	if ok {
		b.entries = append(b.entries, batch.Entries...)
	}
	return 1, nil
}

func (b *sourceIdentityBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

// TestChildSARouteSourceIdentitySurvivesRekeyAndRemoval publishes two real
// ChildSA projections sharing a prefix and interface, rekeys one with a changed
// selector set and removes its successor. Every event retains its logical
// source while the unrelated child's shared and unique prefixes remain live.
func TestChildSARouteSourceIdentitySurvivesRekeyAndRemoval(t *testing.T) {
	first := rekeyTestChild(t, "10.255.2.0/24", "10.255.3.0/24")
	second := rekeyTestChild(t, "10.255.2.0/24", "10.255.4.0/24")
	require.Equal(t, first.IfID, second.IfID)
	bus := &sourceIdentityBus{}
	log := slogutil.DiscardLogger()
	emitRouteAdd(bus, first, log)
	emitRouteAdd(bus, second, log)
	require.Len(t, bus.entries, 4)
	firstID := bus.entries[0].SourceInstance
	secondID := bus.entries[2].SourceInstance
	require.NotZero(t, firstID)
	require.NotZero(t, secondID)
	require.NotEqual(t, firstID, secondID)
	shared := netip.MustParsePrefix("10.255.2.0/24")
	require.Equal(t, shared, bus.entries[0].Prefix)
	require.Equal(t, shared, bus.entries[2].Prefix)
	successor := newRekeyedChild(first, 31, 32, nil, true, nil, modeTunnel)
	replacement := rekeyTestChild(t, "10.255.2.0/24", "10.255.5.0/24")
	successor.Selectors = replacement.Selectors
	successor.TSLocal = replacement.TSLocal
	successor.TSRemote = replacement.TSRemote
	rekeyTunnelRoutes(bus, "first", first, successor, log)
	require.Len(t, bus.entries, 6, "rekey publishes only its changed prefixes")
	require.Equal(t, redistevents.ActionAdd, bus.entries[4].Action)
	require.Equal(t, firstID, bus.entries[4].SourceInstance)
	require.Equal(t, netip.MustParsePrefix("10.255.5.0/24"), bus.entries[4].Prefix)
	require.Equal(t, redistevents.ActionRemove, bus.entries[5].Action)
	require.Equal(t, firstID, bus.entries[5].SourceInstance)
	require.Equal(t, netip.MustParsePrefix("10.255.3.0/24"), bus.entries[5].Prefix)
	emitRouteRemove(bus, successor, log)
	require.Len(t, bus.entries, 8)
	for _, entry := range bus.entries[6:] {
		require.Equal(t, redistevents.ActionRemove, entry.Action)
		require.Equal(t, firstID, entry.SourceInstance)
	}
	live := map[uint32]map[netip.Prefix]bool{}
	for _, entry := range bus.entries {
		if live[entry.SourceInstance] == nil {
			live[entry.SourceInstance] = map[netip.Prefix]bool{}
		}
		if entry.Action == redistevents.ActionAdd {
			live[entry.SourceInstance][entry.Prefix] = true
		} else {
			delete(live[entry.SourceInstance], entry.Prefix)
		}
	}
	require.Empty(t, live[firstID])
	require.Equal(t, map[netip.Prefix]bool{shared: true, netip.MustParsePrefix("10.255.4.0/24"): true}, live[secondID])
}

// TestChildSARouteSourceIdentityExhaustionRefusesWithoutWrap exercises the
// allocator's final legal identity and its paired refusal without changing the
// production allocator or publishing an unspecified/aliased source.
func TestChildSARouteSourceIdentityExhaustionRefusesWithoutWrap(t *testing.T) {
	allocator := childRouteSourceAllocator{next: ^uint32(0) - 1}
	last := &ChildSA{}
	id, err := allocator.instance(last)
	require.NoError(t, err)
	require.Equal(t, ^uint32(0), id)
	repeated, err := allocator.instance(last)
	require.NoError(t, err)
	require.Equal(t, id, repeated)
	refused := &ChildSA{}
	_, err = allocator.instance(refused)
	require.ErrorIs(t, err, errChildRouteSourceExhausted)
	require.Zero(t, refused.redistributeInstance)
	require.Equal(t, ^uint32(0), allocator.next)
}

// TestChildSARouteSourceRekeyRefusesForeignSuccessor keeps an independently
// assigned child from adopting another original's source identity.
func TestChildSARouteSourceRekeyRefusesForeignSuccessor(t *testing.T) {
	allocator := childRouteSourceAllocator{}
	first, second := &ChildSA{}, &ChildSA{}
	firstID, err := allocator.instance(first)
	require.NoError(t, err)
	secondID, err := allocator.instance(second)
	require.NoError(t, err)
	require.NotEqual(t, firstID, secondID)
	_, err = allocator.inherit(first, second)
	require.ErrorIs(t, err, errChildRouteSourceAssigned)
	require.Equal(t, secondID, second.redistributeInstance)
	replacement := &ChildSA{}
	inherited, err := allocator.inherit(first, replacement)
	require.NoError(t, err)
	require.Equal(t, firstID, inherited)
}

// TestChildSARouteSourceFailureRemainsVisible checks the actual producer's
// bounded sticky finding before and after a later successful source lookup.
// A successful different publication cannot reconstruct the lost output.
func TestChildSARouteSourceFailureRemainsVisible(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	allocator := childRouteSourceAllocator{}
	require.Empty(t, allocator.findings())
	first, second := &ChildSA{}, &ChildSA{}
	_, err := allocator.instance(first)
	require.NoError(t, err)
	_, err = allocator.instance(second)
	require.NoError(t, err)
	_, err = allocator.inherit(first, second)
	require.ErrorIs(t, err, errChildRouteSourceAssigned)
	findings := allocator.findings()
	require.Len(t, findings, 1)
	require.Equal(t, "doctor-ipsec-redistribution-source", findings[0].Code)
	require.Equal(t, "sudo systemctl restart ze", findings[0].Fix)
	_, err = allocator.instance(first)
	require.NoError(t, err)
	require.Equal(t, findings, allocator.findings())

	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
	otherAlloc := childRouteSourceAllocator{}
	a, b := &ChildSA{}, &ChildSA{}
	_, err = otherAlloc.instance(a)
	require.NoError(t, err)
	_, err = otherAlloc.instance(b)
	require.NoError(t, err)
	_, err = otherAlloc.inherit(a, b)
	require.ErrorIs(t, err, errChildRouteSourceAssigned)
	otherFindings := otherAlloc.findings()
	require.Len(t, otherFindings, 1)
	require.NotContains(t, otherFindings[0].Fix, "systemctl")
	require.Contains(t, otherFindings[0].Fix, "ze signal stop")
}
