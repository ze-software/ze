// VALIDATES: spec-ospf-10 RFC 2328 sec 12.4.4 `default-information originate` -- the
// `always` form originates a Type 5 default (0.0.0.0/0) unconditionally; the bare form
// originates only while a NON-OSPF default exists in the Loc-RIB (self-exclusion avoids
// a feedback loop); the default is withdrawn when the condition lapses; the Loc-RIB
// watcher re-evaluates live when a default appears or disappears.
// PREVENTS: regressions where conditional origination ignores the RIB, OSPF's own
// default satisfies its own condition, `off` still originates, a lapsed condition
// leaves a stale Type 5, or the watcher is dead/unwired.
package ospf

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
)

var testDefaultRoute = netip.MustParsePrefix("0.0.0.0/0")

// installRIBDefault inserts a non-OSPF default route into the global Loc-RIB so a
// conditional `default-information originate` sees a real default, and registers its
// removal. The returned func removes it early (to exercise the withdraw path).
func installRIBDefault(t *testing.T) func() {
	t.Helper()
	loc := locrib.Default()
	staticID := redistevents.RegisterProtocol("static")
	loc.InsertForward(family.IPv4Unicast, testDefaultRoute,
		locrib.Path{Source: staticID, NextHop: netip.MustParseAddr("10.0.0.254"), AdminDistance: 1, Metric: 1}, nil)
	removed := false
	remove := func() {
		if removed {
			return
		}
		loc.Remove(family.IPv4Unicast, testDefaultRoute, staticID, 0)
		removed = true
	}
	t.Cleanup(remove)
	return remove
}

func TestOSPFDefaultInformationOriginate(t *testing.T) {
	t.Run("always_unconditional", func(t *testing.T) {
		eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1","default-information":{"originate":true,"always":true,"metric":"5","metric-type":"type-1"}}}`)
		eng.applyDefaultInformation()
		require.Equal(t, 1, eng.lsdb.SelfExternalCount(rid), "always originates a Type 5 default with no RIB default present")
		body, ok := externalBody(t, eng, rid, "0.0.0.0/0")
		require.True(t, ok)
		assert.Equal(t, uint32(5), body.Metric, "metric from default-information config")
		assert.False(t, body.ExternalType2, "metric-type type-1 -> E1")
		assert.Equal(t, [4]byte{}, body.NetworkMask, "default route mask is 0.0.0.0")
	})

	t.Run("conditional_with_rib_default", func(t *testing.T) {
		installRIBDefault(t)
		eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.2","default-information":{"originate":true}}}`)
		eng.applyDefaultInformation()
		require.Equal(t, 1, eng.lsdb.SelfExternalCount(rid), "conditional originates when a non-OSPF default exists in the RIB")
		body, ok := externalBody(t, eng, rid, "0.0.0.0/0")
		require.True(t, ok)
		assert.Equal(t, DefaultDefaultMetric, body.Metric, "default metric")
		assert.True(t, body.ExternalType2, "default metric-type type-2 -> E2")
	})

	t.Run("conditional_without_rib_default", func(t *testing.T) {
		eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.3","default-information":{"originate":true}}}`)
		eng.applyDefaultInformation()
		assert.Equal(t, 0, eng.lsdb.SelfExternalCount(rid), "conditional does NOT originate without a RIB default")
	})

	t.Run("self_default_does_not_satisfy_condition", func(t *testing.T) {
		// An OSPF-sourced default in the RIB must NOT satisfy OSPF's own condition
		// (no feedback loop). Tag the inserted default with OSPF's own ProtocolID.
		loc := locrib.Default()
		loc.InsertForward(family.IPv4Unicast, testDefaultRoute,
			locrib.Path{Source: ospfspf.ProtocolID(), NextHop: netip.MustParseAddr("10.0.0.9"), AdminDistance: 110, Metric: 1}, nil)
		t.Cleanup(func() { loc.Remove(family.IPv4Unicast, testDefaultRoute, ospfspf.ProtocolID(), 0) })
		eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.4","default-information":{"originate":true}}}`)
		eng.applyDefaultInformation()
		assert.Equal(t, 0, eng.lsdb.SelfExternalCount(rid), "OSPF's own default does not satisfy its own originate condition")
	})

	t.Run("off_no_origination", func(t *testing.T) {
		installRIBDefault(t) // a default exists, but originate is not configured
		eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.5"}}`)
		eng.applyDefaultInformation()
		assert.Equal(t, 0, eng.lsdb.SelfExternalCount(rid), "no default-information originate -> no Type 5")
	})

	t.Run("withdraw_when_condition_lapses", func(t *testing.T) {
		remove := installRIBDefault(t)
		eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.6","default-information":{"originate":true}}}`)
		eng.applyDefaultInformation()
		require.Equal(t, 1, eng.lsdb.SelfExternalCount(rid), "originated while the RIB default exists")
		remove() // RIB default goes away
		eng.applyDefaultInformation()
		assert.Equal(t, 0, eng.lsdb.SelfExternalCount(rid), "Type 5 default withdrawn when the condition lapses")
	})
}

// TestOSPFDefaultInformationWatcher proves the Loc-RIB watcher wires the conditional
// default to live RIB best-path changes: a non-OSPF default appearing originates the
// Type 5, and its removal withdraws it, with no OSPF topology change in between.
func TestOSPFDefaultInformationWatcher(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.7","default-information":{"originate":true}}}`)
	eng.watchDefaultRoute()
	defer eng.shutdown()

	require.Equal(t, 0, eng.lsdb.SelfExternalCount(rid), "no RIB default yet -> nothing originated")

	remove := installRIBDefault(t)
	require.Eventually(t, func() bool { return eng.lsdb.SelfExternalCount(rid) == 1 }, 2*time.Second, 5*time.Millisecond,
		"watcher originates the default when a non-OSPF default appears in the RIB")

	remove()
	require.Eventually(t, func() bool { return eng.lsdb.SelfExternalCount(rid) == 0 }, 2*time.Second, 5*time.Millisecond,
		"watcher withdraws the default when the RIB default disappears")
}

// TestOSPFDefaultRouteSharedWithRedistribute covers the shared 0.0.0.0/0 Type 5 key:
// default-information and a redistributed default both want it, and a withdraw from one
// intent must NOT drop a default the other still wants.
func TestOSPFDefaultRouteSharedWithRedistribute(t *testing.T) {
	t.Run("redist_withdraw_keeps_default_information_default", func(t *testing.T) {
		eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.1.1","default-information":{"originate":true,"always":true}}}`)
		eng.applyDefaultInformation()
		require.Equal(t, 1, eng.lsdb.SelfExternalCount(rid))
		require.NoError(t, eng.InjectExternal(testDefaultRoute, "static"))
		require.Equal(t, 1, eng.lsdb.SelfExternalCount(rid), "one shared Type 5 default for both intents")

		removed, err := eng.WithdrawExternal(testDefaultRoute)
		require.NoError(t, err)
		assert.True(t, removed, "the redistribution claim existed")
		assert.Equal(t, 1, eng.lsdb.SelfExternalCount(rid), "default-information (always) keeps the default LSA alive")
	})

	t.Run("default_information_off_keeps_redistributed_default", func(t *testing.T) {
		eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.1.2","default-information":{"originate":true,"always":true}}}`)
		eng.applyDefaultInformation()
		require.NoError(t, eng.InjectExternal(testDefaultRoute, "static"))
		require.Equal(t, 1, eng.lsdb.SelfExternalCount(rid))

		// default-information disabled, redistribute still injects the default.
		offCfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.1.2"}}`), nil)
		require.NoError(t, err)
		eng.setConfig(offCfg)
		eng.applyDefaultInformation()
		assert.Equal(t, 1, eng.lsdb.SelfExternalCount(rid), "redistribute keeps the default after default-information is disabled")

		// redistribute withdraws too -> neither intent wants it -> default purged.
		_, err = eng.WithdrawExternal(testDefaultRoute)
		require.NoError(t, err)
		assert.Equal(t, 0, eng.lsdb.SelfExternalCount(rid), "default withdrawn once neither intent wants it")
	})
}

// TestOSPFDefaultInformationConcurrent hammers applyDefaultInformation (the reconcile
// caller) and the Loc-RIB watcher worker while a writer flaps 0.0.0.0/0 in place. It
// exists to trip, under -race, the Lookup-off-lock Paths race and the reconcile/worker
// interleave if either regresses. Asserts only that the run converges without a race.
func TestOSPFDefaultInformationConcurrent(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.2.1","default-information":{"originate":true}}}`)
	eng.watchDefaultRoute()
	defer eng.shutdown()

	loc := locrib.Default()
	staticID := redistevents.RegisterProtocol("static")
	t.Cleanup(func() { loc.Remove(family.IPv4Unicast, testDefaultRoute, staticID, 0) })

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 300 {
			loc.InsertForward(family.IPv4Unicast, testDefaultRoute,
				locrib.Path{Source: staticID, NextHop: netip.MustParseAddr("10.0.0.254"), AdminDistance: 1, Metric: uint32(i)}, nil)
		}
		loc.Remove(family.IPv4Unicast, testDefaultRoute, staticID, 0)
	}()
	go func() {
		defer wg.Done()
		for range 300 {
			eng.applyDefaultInformation()
		}
	}()
	wg.Wait()

	// Converge to a known state: no RIB default -> conditional must not originate.
	eng.applyDefaultInformation()
	assert.Equal(t, 0, eng.lsdb.SelfExternalCount(rid), "no RIB default after the flap -> conditional originates nothing")
}
