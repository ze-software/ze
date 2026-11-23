// Design: docs/architecture/fib/fib-depth-4-srv6.md -- VPP SRv6 store-unavailable degradation and doctor check tests
// Related: doctor_srv6.go -- fib-vpp-srv6 runtime check and codeSRv6Refused
// Related: srv6_state.go -- errSRv6StoreUnavailable and isSRv6StoreUnavailable
package fibvpp

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/sr"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// errInjectionSRv6Store returns err on every store call while err is non-nil;
// when err is nil it delegates to the underlying store.
type errInjectionSRv6Store struct {
	err   error
	inner srv6StateStore
}

func (s *errInjectionSRv6Store) ReadKey(key string) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.inner.ReadKey(key)
}

func (s *errInjectionSRv6Store) WriteKey(key string, data []byte) error {
	if s.err != nil {
		return s.err
	}
	return s.inner.WriteKey(key, data)
}

func (s *errInjectionSRv6Store) RemoveKey(key string) error {
	if s.err != nil {
		return s.err
	}
	return s.inner.RemoveKey(key)
}

func (s *errInjectionSRv6Store) ListKeys(prefix string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.inner.ListKeys(prefix)
}

// TestIsSRv6StoreUnavailableClassifiesTypedErrors proves only the typed
// store-unavailable errors allow plain routes to proceed, while corrupt,
// unreadable, and arbitrary errors stay fail-closed.
//
// VALIDATES: typed classification in isSRv6StoreUnavailable.
// PREVENTS: string matching or treating corrupt state as unavailable.
func TestIsSRv6StoreUnavailableClassifiesTypedErrors(t *testing.T) {
	assert.True(t, isSRv6StoreUnavailable(errSRv6StoreUnavailable))
	assert.True(t, isSRv6StoreUnavailable(statestore.ErrUnavailable))
	assert.True(t, isSRv6StoreUnavailable(&rpc.StateError{Status: rpc.StateUnavailable, Message: "no store"}))

	assert.False(t, isSRv6StoreUnavailable(nil))
	assert.False(t, isSRv6StoreUnavailable(&rpc.StateError{Status: rpc.StateCorrupt, Message: "corrupt"}))
	assert.False(t, isSRv6StoreUnavailable(&rpc.StateError{Status: rpc.StateReadFailed, Message: "io"}))
	assert.False(t, isSRv6StoreUnavailable(&rpc.StateError{Status: rpc.StatePersistFailed, Message: "disk full"}))
	assert.False(t, isSRv6StoreUnavailable(errors.New("unconfirmed IP mutation")))
}

// TestFibVPPPlainAndMPLSRoutesSucceedWithoutStore proves plain IP (add,
// replace, remove), MPLS push (add, remove), and flushRoutes all succeed when
// durable daemon state is unavailable, and fib-vpp-srv6 reports no finding.
//
// VALIDATES: journal row read-fails-on-a-fact-the-caller-does-not-need.md
// (2026-11-12) -- routes without an SRv6 SID program without the SRv6
// ownership list.
// PREVENTS: an absent persistent store blocking every IP and MPLS route.
func TestFibVPPPlainAndMPLSRoutesSucceedWithoutStore(t *testing.T) {
	stores := []struct {
		name  string
		store srv6StateStore
	}{
		{name: "nil store", store: nil},
		{
			name:  "rpc StateUnavailable",
			store: &errInjectionSRv6Store{err: &rpc.StateError{Status: rpc.StateUnavailable, Message: "no persistent store"}},
		},
		{
			name:  "statestore ErrUnavailable",
			store: &errInjectionSRv6Store{err: statestore.ErrUnavailable},
		},
	}

	for _, tc := range stores {
		t.Run(tc.name, func(t *testing.T) {
			ip := &mockBackend{}
			mpls := &mockMPLSBackend{}
			ch := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
			f := newFibVPP(ip)
			f.mplsBackend = mpls
			f.srv6Backend = newGovppSRv6Backend(ch, 0, tc.store)

			p1 := netip.MustParsePrefix("10.20.0.0/24")
			p2 := netip.MustParsePrefix("10.20.1.0/24")
			pMPLS := netip.MustParsePrefix("10.30.0.0/24")
			nh1 := netip.MustParseAddr("192.0.2.1")
			nh2 := netip.MustParseAddr("192.0.2.2")

			// Plain IP add, replace, remove.
			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Add, Prefix: p1, NextHop: nh1},
				{Action: routeaction.Add, Prefix: p2, NextHop: nh1},
			}})
			require.Len(t, ip.adds, 2)

			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Update, Prefix: p1, NextHop: nh2},
			}})
			require.Len(t, ip.replaces, 1)

			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Withdraw, Prefix: p2},
			}})
			require.Len(t, ip.dels, 1)
			assert.Equal(t, p2, ip.dels[0])

			// MPLS push add and remove.
			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Add, Prefix: pMPLS, NextHop: nh1, Labels: []uint32{100}},
			}})
			require.Len(t, mpls.pushes, 1)
			assert.Equal(t, pMPLS, mpls.pushes[0].prefix)

			// Re-add another MPLS route so flushRoutes removes both plain IP and MPLS.
			pMPLS2 := netip.MustParsePrefix("10.30.1.0/24")
			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Withdraw, Prefix: pMPLS, Labels: []uint32{100}},
				{Action: routeaction.Add, Prefix: pMPLS2, NextHop: nh1, Labels: []uint32{200}},
			}})
			require.Len(t, mpls.delPushes, 1)
			assert.Equal(t, pMPLS, mpls.delPushes[0])
			require.Len(t, mpls.pushes, 2)

			// Doctor check stays silent when no SRv6 route was requested.
			findings, err := answerDoctorCheck(srv6CheckName, f, nil)
			require.NoError(t, err)
			assert.Empty(t, findings)

			// flushRoutes removes remaining plain IP and MPLS routes.
			f.flushRoutes()
			require.Len(t, ip.dels, 2)
			assert.Equal(t, p1, ip.dels[1])
			require.Len(t, mpls.delPushes, 2)
			assert.Equal(t, pMPLS2, mpls.delPushes[1])
			assert.Empty(t, f.installed)
			assert.Empty(t, f.mplsInstalled)
			assert.Empty(t, ch.requests)
		})
	}
}

// TestFibVPPMixedBatchRefusesSRv6AndProgramsPlainIP proves a mixed batch with
// an unavailable store refuses the SRv6 route without panicking, programs the
// plain IP route in the same batch, reports the refusal to fib-vpp-srv6 with
// the ze init fix, and clears the finding on withdrawal or plain IP replace.
//
// VALIDATES: mixed SRv6 + plain IP batch under unavailable store.
// PREVENTS: SRv6 refusal poisoning sibling routes or panicking on nil store.
func TestFibVPPMixedBatchRefusesSRv6AndProgramsPlainIP(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	stores := []struct {
		name  string
		store srv6StateStore
	}{
		{name: "nil store", store: nil},
		{
			name:  "rpc StateUnavailable",
			store: &errInjectionSRv6Store{err: &rpc.StateError{Status: rpc.StateUnavailable, Message: "no persistent store"}},
		},
	}

	for _, tc := range stores {
		t.Run(tc.name, func(t *testing.T) {
			ip := &mockBackend{}
			ch := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
			f := newFibVPP(ip)
			f.srv6Backend = newGovppSRv6Backend(ch, 0, tc.store)

			srv6Prefix := netip.MustParsePrefix("198.51.100.0/24")
			plainPrefix := netip.MustParsePrefix("10.20.0.0/24")
			sid := netip.MustParseAddr("2001:db8:1::a")
			nh := netip.MustParseAddr("192.0.2.1")

			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Add, Prefix: srv6Prefix, SRv6SID: sid, TableID: 10},
				{Action: routeaction.Add, Prefix: plainPrefix, NextHop: nh},
			}})

			require.Len(t, ip.adds, 1, "plain IP route in same batch must still be programmed")
			assert.Equal(t, plainPrefix, ip.adds[0].prefix)
			assert.Empty(t, ch.requests, "refused SRv6 route must not touch VPP")

			findings, err := answerDoctorCheck(srv6CheckName, f, nil)
			require.NoError(t, err)
			require.Len(t, findings, 1)
			assert.Equal(t, codeSRv6Refused, findings[0].Code)
			assert.Equal(t, string(diagnostic.SeverityError), findings[0].Severity)
			assert.Contains(t, findings[0].Message, srv6Prefix.String())
			assert.Contains(t, findings[0].Message, sid.String())
			assert.Equal(t, "ze init, then sudo systemctl restart ze", findings[0].Fix)

			// Withdrawing the SRv6 route clears the doctor finding.
			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Withdraw, Prefix: srv6Prefix, SRv6SID: sid, TableID: 10},
			}})
			findings, err = answerDoctorCheck(srv6CheckName, f, nil)
			require.NoError(t, err)
			assert.Empty(t, findings)

			// Refuse again, then replace with a plain IP route: clears the finding.
			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Add, Prefix: srv6Prefix, SRv6SID: sid},
			}})
			findings, err = answerDoctorCheck(srv6CheckName, f, nil)
			require.NoError(t, err)
			require.Len(t, findings, 1)

			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Update, Prefix: srv6Prefix, NextHop: nh},
			}})
			findings, err = answerDoctorCheck(srv6CheckName, f, nil)
			require.NoError(t, err)
			assert.Empty(t, findings)

			// Refuse again, then withdraw without SID: also clears the finding.
			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Update, Prefix: srv6Prefix, SRv6SID: sid, TableID: 10},
			}})
			findings, err = answerDoctorCheck(srv6CheckName, f, nil)
			require.NoError(t, err)
			require.Len(t, findings, 1)

			f.processEvent(&incomingBatch{Changes: []incomingChange{
				{Action: routeaction.Withdraw, Prefix: srv6Prefix},
			}})
			findings, err = answerDoctorCheck(srv6CheckName, f, nil)
			require.NoError(t, err)
			assert.Empty(t, findings)
		})
	}
}

// TestFibVPPPlainToSRv6UpgradeWithNilStoreDoesNotPanic proves upgrading an
// existing ordinary IP route to SRv6 when store == nil skips
// checkpointSRv6Fallback, preserves the ordinary route, and reports the refusal
// to doctor.
//
// VALIDATES: IP-to-SRv6 upgrade with nil store refuses cleanly before b.store.
// PREVENTS: nil pointer dereference in checkpointSRv6Fallback -> saveFallback.
func TestFibVPPPlainToSRv6UpgradeWithNilStoreDoesNotPanic(t *testing.T) {
	ip := &srIPModelBackend{routes: make(map[srv6RouteKey]vppRichRoute)}
	ch := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	f := newFibVPP(ip)
	f.srv6Backend = newGovppSRv6Backend(ch, 0, nil)

	prefix := netip.MustParsePrefix("192.0.2.0/24")
	nh := netip.MustParseAddr("198.51.100.1")
	sid := netip.MustParseAddr("2001:db8:1::a")

	f.processEvent(&incomingBatch{Changes: []incomingChange{
		{Action: routeaction.Add, Prefix: prefix, NextHop: nh},
	}})
	require.Len(t, ip.routes, 1)

	require.NotPanics(t, func() {
		f.processEvent(&incomingBatch{Changes: []incomingChange{
			{Action: routeaction.Update, Prefix: prefix, NextHop: nh, SRv6SID: sid},
		}})
	})
	assert.Len(t, ip.routes, 1, "existing ordinary route must remain intact")
	assert.Empty(t, ch.requests)

	findings, err := answerDoctorCheck(srv6CheckName, f, nil)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, codeSRv6Refused, findings[0].Code)
	assert.Contains(t, findings[0].Fix, "ze init")

	// Withdrawing the route with the SID still set removes the ordinary route
	// and clears the doctor finding.
	f.processEvent(&incomingBatch{Changes: []incomingChange{
		{Action: routeaction.Withdraw, Prefix: prefix, SRv6SID: sid},
	}})
	assert.Empty(t, ip.routes, "ordinary route must be withdrawn")
	findings, err = answerDoctorCheck(srv6CheckName, f, nil)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// TestFibVPPSRv6CorruptAndUnconfirmedStillBlockMutations proves corrupt state
// and unconfirmed IP mutations keep blocking both processEvent and flushRoutes
// to preserve forwarding, and report doctor-fib-vpp-srv6-refused with the
// `ze show data check` fix.
//
// VALIDATES: fail-closed preservation on non-unavailable restore errors.
// PREVENTS: relaxing store-unavailable checks from weakening corrupt/ambiguous
// state protection.
func TestFibVPPSRv6CorruptAndUnconfirmedStillBlockMutations(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))

	t.Run("StateCorrupt", func(t *testing.T) {
		backing := &srRPCBoundedStore{files: make(map[string][]byte)}
		store := &errInjectionSRv6Store{
			err:   &rpc.StateError{Status: rpc.StateCorrupt, Message: "checksum mismatch"},
			inner: backing,
		}
		ip := &mockBackend{}
		ch := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
		f := newFibVPP(ip)
		f.srv6Backend = newGovppSRv6Backend(ch, 0, store)
		existingKey := srv6RouteKey{prefix: netip.MustParsePrefix("10.0.0.0/24")}
		f.installed[existingKey] = installedRoute{nextHop: "192.0.2.1"}

		f.processEvent(&incomingBatch{Changes: []incomingChange{
			{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.20.0.0/24"), NextHop: netip.MustParseAddr("192.0.2.2")},
		}})
		assert.Empty(t, ip.adds, "corrupt state must block plain IP mutations")

		f.flushRoutes()
		assert.Empty(t, ip.dels, "corrupt state must block flushRoutes")
		assert.Contains(t, f.installed, existingKey)

		findings, err := answerDoctorCheck(srv6CheckName, f, nil)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		assert.Equal(t, codeSRv6Refused, findings[0].Code)
		assert.Equal(t, string(diagnostic.SeverityError), findings[0].Severity)
		assert.Contains(t, findings[0].Message, "checksum mismatch")
		assert.Equal(t, "ze show data check, then sudo systemctl restart ze", findings[0].Fix)

		// Once the store recovers, the next event clears srv6RestoreErr.
		store.err = nil
		f.processEvent(&incomingBatch{Changes: []incomingChange{
			{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.20.0.0/24"), NextHop: netip.MustParseAddr("192.0.2.2")},
		}})
		require.Len(t, ip.adds, 1)
		findings, err = answerDoctorCheck(srv6CheckName, f, nil)
		require.NoError(t, err)
		assert.Empty(t, findings)
	})

	t.Run("unconfirmed IP mutation", func(t *testing.T) {
		store := &srRPCBoundedStore{files: make(map[string][]byte)}
		ch := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
		seed := newGovppSRv6Backend(ch, 0, store)
		require.NoError(t, seed.restore())
		prefix := netip.MustParsePrefix("192.0.2.0/24")
		require.NoError(t, seed.saveFallback(&srv6Fallback{
			Prefix:    prefix,
			Table:     10,
			NextHop:   "198.51.100.1",
			Confirmed: false,
		}))

		ip := &mockBackend{}
		f := newFibVPP(ip)
		f.srv6Backend = newGovppSRv6Backend(ch, 0, store)
		f.installed[srv6RouteKey{prefix: prefix, table: 10}] = installedRoute{nextHop: "198.51.100.1"}

		f.processEvent(&incomingBatch{Changes: []incomingChange{
			{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.20.0.0/24"), NextHop: netip.MustParseAddr("192.0.2.2")},
		}})
		assert.Empty(t, ip.adds, "unconfirmed IP mutation must block mutations")

		f.flushRoutes()
		assert.Empty(t, ip.richDels, "unconfirmed IP mutation must block flushRoutes")

		findings, err := answerDoctorCheck(srv6CheckName, f, nil)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		assert.Equal(t, codeSRv6Refused, findings[0].Code)
		assert.Contains(t, findings[0].Message, "unconfirmed IP mutation")
		assert.Equal(t, "ze show data check, then sudo systemctl restart ze", findings[0].Fix)
	})
}

// TestFibVPPSRv6DoctorBoundsAndSortsRefusals proves fib-vpp-srv6 bounds its
// output at labelFindingsMax plus one overflow line, ordered by table then
// prefix.
//
// VALIDATES: deterministic ordering and bounded doctor output for srv6Refusals.
// PREVENTS: unbounded doctor responses on large BGP SRv6 tables.
func TestFibVPPSRv6DoctorBoundsAndSortsRefusals(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	ch := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	f := newFibVPP(&mockBackend{})
	f.srv6Backend = newGovppSRv6Backend(ch, 0, nil)
	sid := netip.MustParseAddr("2001:db8:1::a")

	const extra = 5
	total := labelFindingsMax + extra
	changes := make([]incomingChange, 0, total)
	// Insert in reverse table and prefix order to verify deterministic sorting.
	for i := total - 1; i >= 0; i-- {
		table := uint32(10)
		if i < 2 {
			table = 1
		}
		addr := netip.AddrFrom4([4]byte{10, 0, byte(i >> 8), byte(i)})
		changes = append(changes, incomingChange{
			Action:  routeaction.Add,
			Prefix:  netip.PrefixFrom(addr, 32),
			TableID: table,
			SRv6SID: sid,
		})
	}
	f.processEvent(&incomingBatch{Changes: changes})

	findings, err := answerDoctorCheck(srv6CheckName, f, nil)
	require.NoError(t, err)
	require.Len(t, findings, labelFindingsMax+1)
	assert.Contains(t, findings[0].Message, "10.0.0.0/32 in IP table 1")
	assert.Contains(t, findings[1].Message, "10.0.0.1/32 in IP table 1")
	assert.Contains(t, findings[2].Message, "10.0.0.2/32 in IP table 10")
	last := findings[labelFindingsMax]
	assert.Equal(t, codeSRv6Refused, last.Code)
	assert.Contains(t, last.Message, "5 more SRv6 routes were refused")
	assert.Equal(t, "ze init, then sudo systemctl restart ze", last.Fix)
}

// TestFibVPPSRv6DoctorWiringAndRuntimeRefusal proves fib-vpp-srv6 answers empty
// when no writer exists, registers its code metadata with diagnostic.Lookup,
// records a VPP steering error with the restart fix, and clears it on success.
//
// VALIDATES: runtime check wiring, code metadata registration, and steering
// refusal lifecycle.
// PREVENTS: unregistered doctor code or sticky refusal after retry succeeds.
func TestFibVPPSRv6DoctorWiringAndRuntimeRefusal(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	assert.NotNil(t, diagnostic.Lookup(codeSRv6Refused), "ze show diagnostic describes codeSRv6Refused")

	findings, err := answerDoctorCheck(srv6CheckName, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, findings, "no writer holds no SRv6 state")

	store := &srRPCBoundedStore{files: make(map[string][]byte)}
	ch := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	f := newFibVPP(&mockBackend{})
	f.srv6Backend = newGovppSRv6Backend(ch, 0, store)

	prefix := netip.MustParsePrefix("198.51.100.0/24")
	sid := netip.MustParseAddr("2001:db8:1::a")
	ch.fail = "sr_steering_add_del"
	f.processEvent(&incomingBatch{Changes: []incomingChange{
		{Action: routeaction.Add, Prefix: prefix, SRv6SID: sid},
	}})

	findings, err = answerDoctorCheck(srv6CheckName, f, nil)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, codeSRv6Refused, findings[0].Code)
	assert.Equal(t, "sudo systemctl restart ze", findings[0].Fix)

	// Retry succeeds and clears the finding.
	f.processEvent(&incomingBatch{Changes: []incomingChange{
		{Action: routeaction.Add, Prefix: prefix, SRv6SID: sid},
	}})
	findings, err = answerDoctorCheck(srv6CheckName, f, nil)
	require.NoError(t, err)
	assert.Empty(t, findings)
}
