package iface

import (
	"errors"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/ndp"
)

// withCleanDelegations gives a test an empty delegation table and restores
// the process table after it, so no test leaks a lease into the next.
func withCleanDelegations(t *testing.T) {
	t.Helper()
	previous := heldDelegations
	heldDelegations = &delegations{
		held:  make(map[string]map[netip.Prefix]heldDelegation),
		inUse: make(map[string]map[netip.Prefix]LifetimeDeadline),
	}
	t.Cleanup(func() { heldDelegations = previous })
}

// delegatedTestUnit is a LAN unit numbering subnet id 1 from pppoe0.
func delegatedTestUnit() *unitEntry {
	return &unitEntry{
		Label: "0",
		IPv6: &ipv6Settings{DelegatedPrefixes: []delegatedPrefixConfig{{
			Source:      "pppoe0",
			SubnetID:    1,
			InterfaceID: netip.MustParseAddr("::1"),
		}}},
	}
}

// VALIDATES: RFC 9915 Section 6.3 (RFC9915-6.3-1), spec AC-7 and AC-10. The
// lifetimes a unit's address takes are what remains of the delegation at the
// moment of use, counted from the Reply's receive time, the preferred one
// never above the valid one; a renew with shorter lifetimes shrinks them.
// METHOD: record a delegation at a fixed receive time, then read the unit's
// /64 and its lifetimes at chosen later moments.
// PREVENTS: an address or advertisement carrying the lease's full lifetime
// after part of it has run, which outlives the delegation.
func TestRFC9915DelegatedLifetimeBound(t *testing.T) {
	received := time.Unix(1_800_000_000, 0)
	delegated := netip.MustParsePrefix("2001:db8:aa00::/56")

	t.Run("lifetimes count down from the receive time", func(t *testing.T) {
		withCleanDelegations(t)
		heldDelegations.record("pppoe0", delegated, received, 3600, 1800)

		now := received.Add(1000*time.Second + 400*time.Millisecond)
		adverts, refusals := claimDelegatedAdverts(delegatedTestUnit(), now)
		require.Empty(t, refusals)
		require.Len(t, adverts, 1)
		assert.Equal(t, netip.MustParsePrefix("2001:db8:aa00:1::1/64"), adverts[0].Address)
		assert.Equal(t, delegated, adverts[0].Delegated)
		valid, preferred := adverts[0].Lifetimes(now)
		assert.Equal(t, uint32(2599), valid, "rounded down, never above what remains")
		assert.Equal(t, uint32(799), preferred)
	})

	t.Run("a preferred lifetime above the valid one is cut to it", func(t *testing.T) {
		withCleanDelegations(t)
		heldDelegations.record("pppoe0", delegated, received, 600, 7200)
		adverts, _ := claimDelegatedAdverts(delegatedTestUnit(), received)
		require.Len(t, adverts, 1)
		valid, preferred := adverts[0].Lifetimes(received)
		assert.Equal(t, uint32(600), valid)
		assert.Equal(t, uint32(600), preferred)
	})

	t.Run("an infinite delegation stays infinite", func(t *testing.T) {
		withCleanDelegations(t)
		heldDelegations.record("pppoe0", delegated, received, lifetimeInfiniteSeconds, lifetimeInfiniteSeconds)
		adverts, _ := claimDelegatedAdverts(delegatedTestUnit(), received.Add(1000*time.Hour))
		require.Len(t, adverts, 1)
		valid, preferred := adverts[0].Lifetimes(received.Add(1000 * time.Hour))
		assert.Equal(t, ndp.LifetimeInfinity, valid)
		assert.Equal(t, ndp.LifetimeInfinity, preferred)
	})

	t.Run("a renew with shorter lifetimes shrinks them", func(t *testing.T) {
		withCleanDelegations(t)
		heldDelegations.record("pppoe0", delegated, received, 7200, 3600)
		renewed := received.Add(100 * time.Second)
		heldDelegations.record("pppoe0", delegated, renewed, 900, 300)
		adverts, _ := claimDelegatedAdverts(delegatedTestUnit(), renewed)
		require.Len(t, adverts, 1)
		valid, preferred := adverts[0].Lifetimes(renewed)
		assert.Equal(t, uint32(900), valid)
		assert.Equal(t, uint32(300), preferred)
	})

	t.Run("an expired delegation numbers nothing", func(t *testing.T) {
		withCleanDelegations(t)
		heldDelegations.record("pppoe0", delegated, received, 60, 30)
		adverts, _ := claimDelegatedAdverts(delegatedTestUnit(), received.Add(61*time.Second))
		assert.Empty(t, adverts)
	})

	t.Run("a delegation too small for the subnet id is refused", func(t *testing.T) {
		withCleanDelegations(t)
		heldDelegations.record("pppoe0", netip.MustParsePrefix("2001:db8:aa00:5::/64"), received, 3600, 1800)
		adverts, refusals := claimDelegatedAdverts(delegatedTestUnit(), received)
		assert.Empty(t, adverts)
		require.Len(t, refusals, 1)
		assert.True(t, errors.Is(refusals[0].err, errDelegationTooSmall))
	})
}

// VALIDATES: RFC 9915 Section 6.3 (RFC9915-6.3-3), spec AC-12, on the iface
// side: a delegation a unit took space from is in use until it expires or
// its lease reports it gone, and one no unit took is not.
// PREVENTS: the DHCPv6 client releasing a prefix the LAN still numbers from.
func TestDelegatedPrefixInUse(t *testing.T) {
	received := time.Unix(1_800_000_000, 0)
	delegated := netip.MustParsePrefix("2001:db8:aa00::/56")

	withCleanDelegations(t)
	heldDelegations.record("pppoe0", delegated, received, 3600, 1800)
	assert.False(t, heldDelegations.claimed("pppoe0", delegated, received), "no unit took space yet")

	_, _ = claimDelegatedAdverts(delegatedTestUnit(), received)
	assert.True(t, heldDelegations.claimed("pppoe0", delegated, received))
	assert.False(t, heldDelegations.claimed("pppoe1", delegated, received), "another source holds nothing")
	assert.False(t, heldDelegations.claimed("pppoe0", delegated, received.Add(3601*time.Second)), "an expired delegation is outstanding nowhere")

	heldDelegations.record("pppoe0", delegated, received, 0, 0)
	assert.False(t, heldDelegations.claimed("pppoe0", delegated, received), "the lease's expiry ends the claim")
}

// VALIDATES: the bus handler turns a published DHCPv6 lease event into a
// delegation, keyed by source and masked prefix, and ignores an address
// lease. METHOD: push the JSON the DHCPv6 client publishes.
// PREVENTS: a field rename between the publisher and this consumer that
// silently numbers no LAN unit.
func TestDelegationQueueReadsLeaseEvents(t *testing.T) {
	withCleanDelegations(t)
	log := slog.New(slog.DiscardHandler)
	q := newDelegationQueue()

	q.pushLease(`{"name":"pppoe0","unit":"0","address":"2001:db8:aa00::","prefix-length":56,"lease-time":3600,"delegated":true,"preferred-lifetime":1800,"received":1800000000}`, log)
	q.pushLease(`{"name":"pppoe0","unit":"0","address":"2001:db8:ff::5","prefix-length":128,"lease-time":3600,"received":1800000000}`, log)
	recordDelegationEvents(q.take(), log)

	d, ok := heldDelegations.chosen("pppoe0", time.Unix(1_800_000_000, 0))
	require.True(t, ok)
	assert.Equal(t, netip.MustParsePrefix("2001:db8:aa00::/56"), d.prefix)
	assert.Equal(t, uint32(3600), d.valid.Remaining(time.Unix(1_800_000_000, 0)))
	assert.Len(t, heldDelegations.held["pppoe0"], 1, "an address lease is not a delegation")
}
