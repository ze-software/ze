// Design: docs/architecture/route-selection.md -- next-hop admission runtime doctor checks and metric failure coalescing
// Related: rib_next_hop_doctor.go, doctor_codes.go, rib_next_hop.go, rib_remote.go
package rib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// syncLogBuffer captures structured JSON slog output safely across goroutines.
type syncLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncLogBuffer) count(t *testing.T, level, code string) int {
	t.Helper()
	b.mu.Lock()
	raw := bytes.Clone(b.buf.Bytes())
	b.mu.Unlock()

	total := 0
	for line := range bytes.SplitSeq(raw, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		require.NoError(t, json.Unmarshal(line, &rec))
		if rec["level"] != level {
			continue
		}
		if rec["code"] != code {
			continue
		}
		total++
	}
	return total
}

func captureRIBLogs(t *testing.T) *syncLogBuffer {
	t.Helper()
	buf := &syncLogBuffer{}
	prev := logger()
	SetLogger(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { SetLogger(prev) })
	return buf
}

// TestRIBNextHopDoctorCodesRegistered verifies AC-5: both next-hop doctor codes
// are registered with explanation metadata and declared in runtime checks.
//
// VALIDATES: doctor-bgp-next-hop-unresolvable and doctor-bgp-next-hop-metrics
// exist in diagnostic.Lookup with non-empty Title, Description, and Examples,
// and their Stage 1 declarations target DoctorPhaseRuntime.
// PREVENTS: unregistered doctor codes, duplicate init panics, or missing fixes.
func TestRIBNextHopDoctorCodesRegistered(t *testing.T) {
	unresolvable := diagnostic.Lookup(codeDoctorBGPNextHopUnresolvable)
	require.NotNil(t, unresolvable, "doctor-bgp-next-hop-unresolvable must be registered")
	require.NotEmpty(t, unresolvable.Title)
	require.NotEmpty(t, unresolvable.Description)
	require.Contains(t, unresolvable.Description, "RFC 4271")
	require.Contains(t, unresolvable.Description, "Adj-RIB-In")
	require.Contains(t, unresolvable.Description, "ze show bgp rib | match next-hop-unresolvable")
	require.Contains(t, unresolvable.Description, "ze show rib")
	require.NotEmpty(t, unresolvable.Examples)
	require.Contains(t, unresolvable.RelatedCodes, codeDoctorBGPNextHopMetrics)

	metricsMeta := diagnostic.Lookup(codeDoctorBGPNextHopMetrics)
	require.NotNil(t, metricsMeta, "doctor-bgp-next-hop-metrics must remain registered")
	require.NotEmpty(t, metricsMeta.Title)
	require.NotEmpty(t, metricsMeta.Description)
	require.NotEmpty(t, metricsMeta.Examples)
	require.Contains(t, diagnostic.EventCodes(), codeDoctorBGPNextHopMetrics)

	unresolvableDecl := unresolvedNextHopDoctorDeclaration()
	require.Equal(t, unresolvedNextHopDoctorName, unresolvableDecl.Name)
	require.Equal(t, rpc.DoctorPhaseRuntime, unresolvableDecl.Phase)
	require.Contains(t, unresolvableDecl.Codes, codeDoctorBGPNextHopUnresolvable)

	metricsDecl := nextHopMetricsDoctorDeclaration()
	require.Equal(t, nextHopMetricsDoctorName, metricsDecl.Name)
	require.Equal(t, rpc.DoctorPhaseRuntime, metricsDecl.Phase)
	require.Contains(t, metricsDecl.Codes, codeDoctorBGPNextHopMetrics)

	r := newTestRIBManager(t)
	_, err := r.doctorCheck("not-a-rib-doctor-check")
	require.ErrorContains(t, err, "not-a-rib-doctor-check")
}

// TestUnresolvedNextHopDoctorCheckLifecycle verifies AC-3 and tracker bounds:
// unresolved received routes raise doctor-bgp-next-hop-unresolvable warnings,
// clear on resolution, withdrawal, and owner-scoped peer DOWN, and obey the
// 5-sample display cap and 4096-entry tracker cap.
//
// VALIDATES: runtime doctor check lifecycle, same-address owner isolation,
// recovery-admission exclusion, 5-sample cap, and 4096-entry tracker bound.
// PREVENTS: silent next-hop exclusion, stale doctor warnings after cleanup,
// cross-owner clobbering, and unbounded memory growth under unreachable churn.
func TestUnresolvedNextHopDoctorCheckLifecycle(t *testing.T) {
	t.Run("resolve_clears_finding", func(t *testing.T) {
		r, loc := nextHopReachabilityRIB(t)

		findings, err := r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings)

		wirePrefix := ipv4Prefix(24, 10, 50, 0)
		nextHop := netip.MustParseAddr("198.51.100.129")
		nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 100, wirePrefix)

		r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Equal(t, codeDoctorBGPNextHopUnresolvable, findings[0].Code)
		require.Equal(t, "warning", findings[0].Severity)
		require.Contains(t, findings[0].Message, "1")
		require.Contains(t, findings[0].Message, "10.50.0.0/24")
		require.Contains(t, findings[0].Message, "198.51.100.129")
		require.Contains(t, findings[0].Fix, unresolvedNextHopDoctorFix)

		loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.128/25"), igpRoute(0))
		r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings, "resolving the covering route must clear the doctor warning")
	})

	t.Run("withdraw_clears_finding", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)

		wirePrefix := ipv4Prefix(24, 10, 51, 0)
		nextHop := netip.MustParseAddr("198.51.100.130")
		routes := nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 100, wirePrefix)

		r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)

		findings, err := r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)

		routes.Remove(family.IPv4Unicast, wirePrefix)
		r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings, "withdrawing the excluded prefix must clear the doctor warning")
	})

	t.Run("peer_down_purge_isolates_same_address_sibling_owner", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)

		sharedPeer := netip.MustParseAddr("192.0.2.188")
		const (
			ownerA uint64 = 1701
			ownerB uint64 = 1702
		)
		wireA := ipv4Prefix(24, 10, 52, 1)
		wireB := ipv4Prefix(24, 10, 52, 2)
		nextHopA := netip.MustParseAddr("198.51.100.131")
		nextHopB := netip.MustParseAddr("198.51.100.132")
		nextHopReachabilityPath(t, r, ownerA, sharedPeer, nextHopA, 100, wireA)
		nextHopReachabilityPath(t, r, ownerB, sharedPeer, nextHopB, 100, wireB)

		r.checkBestPathChange(family.IPv4Unicast, wireA, false, nil)
		r.checkBestPathChange(family.IPv4Unicast, wireB, false, nil)

		findings, err := r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Contains(t, findings[0].Message, "10.52.1.0/24")
		require.Contains(t, findings[0].Message, "10.52.2.0/24")

		r.peerMu.Lock()
		delete(r.bgpPeers, ownerA)
		r.peerMu.Unlock()
		r.purgeBestPrevForOwner(bestPeer{owner: ownerA, address: sharedPeer.String()})

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1, "purging ownerA must preserve same-address sibling ownerB")
		require.NotContains(t, findings[0].Message, "10.52.1.0/24")
		require.NotContains(t, findings[0].Message, "198.51.100.131")
		require.Contains(t, findings[0].Message, "10.52.2.0/24")
		require.Contains(t, findings[0].Message, "198.51.100.132")

		r.peerMu.Lock()
		delete(r.bgpPeers, ownerB)
		r.peerMu.Unlock()
		r.purgeBestPrevForPeer(sharedPeer.String())

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings, "purging remaining peer address must clear all unresolved findings")
	})

	t.Run("recovery_admission_does_not_raise_finding", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)

		prefix := netip.MustParsePrefix("10.53.0.0/24")
		wirePrefix := ipv4Prefix(24, 10, 53, 0)
		nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("198.51.100.133"), 100, wirePrefix)

		r.peerMu.RLock()
		candidates := r.gatherPrefixCandidatesForLocked(family.IPv4Unicast, prefix, candidateAdmissionRecovery)
		r.peerMu.RUnlock()
		releaseCandidates(candidates)

		findings, err := r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings, "private recovery admission must never raise the unresolved next-hop doctor warning")
	})

	t.Run("sample_cap_five_and_more_summary", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)

		nextHop := netip.MustParseAddr("198.51.100.200")
		attrs := aigpSelectionAttrs(nextHop.As4(), nil, 100, 1)
		firstWire := ipv4Prefix(24, 10, 60, 0)
		routes := nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 100, firstWire)
		r.checkBestPathChange(family.IPv4Unicast, firstWire, false, nil)

		for i := byte(1); i < 5; i++ {
			wire := ipv4Prefix(24, 10, 60, i)
			routes.Insert(family.IPv4Unicast, attrs, wire)
			r.checkBestPathChange(family.IPv4Unicast, wire, false, nil)
		}

		findings, err := r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Contains(t, findings[0].Message, "5")
		require.NotContains(t, findings[0].Message, "more)", "5 entries sit at the sample cap with no overflow suffix")

		for i := byte(5); i < 7; i++ {
			wire := ipv4Prefix(24, 10, 60, i)
			routes.Insert(family.IPv4Unicast, attrs, wire)
			r.checkBestPathChange(family.IPv4Unicast, wire, false, nil)
		}

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Contains(t, findings[0].Message, "7")
		require.Contains(t, findings[0].Message, "(+2 more)")
	})

	t.Run("tracker_cap_4096_and_overflow_count", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)

		peerAddr := netip.MustParseAddr("192.0.2.9")
		nextHop := netip.MustParseAddr("198.51.100.201")
		attrs := aigpSelectionAttrs(nextHop.As4(), nil, 100, 1)
		firstWire := ipv4Prefix(24, 10, 64, 0)
		routes := nextHopReachabilityPath(t, r, 9, peerAddr, nextHop, 100, firstWire)
		r.checkBestPathChange(family.IPv4Unicast, firstWire, false, nil)

		const (
			trackerCap = 4096
			overflow   = 5
		)
		for i := 1; i < trackerCap+overflow; i++ {
			wire := ipv4Prefix(24, 10, 64+byte(i>>8), byte(i&0xff))
			routes.Insert(family.IPv4Unicast, attrs, wire)
			r.checkBestPathChange(family.IPv4Unicast, wire, false, nil)
		}

		findings, err := r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Equal(t, codeDoctorBGPNextHopUnresolvable, findings[0].Code)
		require.Equal(t, "warning", findings[0].Severity)
		require.Contains(t, findings[0].Message, "4101", "summary must include tracked entries plus overflow")
		require.Contains(t, findings[0].Message, "4096", "summary must state the tracking cap when overflowed")
		require.Contains(t, findings[0].Message, "(+4096 more)")
		require.Contains(t, findings[0].Fix, unresolvedNextHopDoctorFix)

		r.peerMu.Lock()
		delete(r.bgpPeers, 9)
		r.peerMu.Unlock()
		r.purgeBestPrevForOwner(bestPeer{owner: 9, address: peerAddr.String()})

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings, "purging the owner must clear tracked and overflow state")
	})
}

// TestRemoteMetricFailureCoalescesLogAndReportsDoctor verifies AC-4: repeated
// forked route-metrics failures remain fail-closed uncached, log once per
// healthy->failed transition, surface doctor-bgp-next-hop-metrics via
// OnDoctorCheck, and clear + re-arm after recovery.
//
// VALIDATES: error-log coalescing across prospectiveDistance and igpDistance,
// uncached fail-closed lookup behavior, and live OnDoctorCheck reporting.
// PREVENTS: error-log flooding during engine metric outages and missing live
// doctor visibility for forked route-metrics failures.
func TestRemoteMetricFailureCoalescesLogAndReportsDoctor(t *testing.T) {
	logs := captureRIBLogs(t)

	var failLookups atomic.Bool
	var rpcCalls atomic.Int64

	clientSide, engineSide := net.Pipe()
	bridge := rpc.NewDirectBridge()
	bridge.SetDeliverEvents(func([]string) error { return nil })
	bridge.SetDispatchRPC(func(_ context.Context, method string, body json.RawMessage) (json.RawMessage, error) {
		if method != rpc.MethodRouteMetrics {
			return nil, errors.New("unexpected engine RPC method: " + method)
		}
		var input rpc.RouteMetricsInput
		if err := json.Unmarshal(body, &input); err != nil {
			return nil, err
		}
		rpcCalls.Add(1)
		if failLookups.Load() {
			return nil, errors.New("controlled engine route-metrics failure")
		}
		distances := make([]rpc.RouteMetric, len(input.Addresses))
		for i := range distances {
			distances[i] = rpc.RouteMetric{Resolved: true, Cost: 10}
		}
		return json.Marshal(rpc.RouteMetricsOutput{Revision: 1, Distances: distances})
	})
	bridge.SetReady()

	client := sdk.NewWithConn("bgp-rib", rpc.NewBridgedConn(clientSide, bridge))
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		require.NoError(t, engineSide.Close())
	})

	r := newRIBManager(client)
	r.forkRIB = &remoteRIB{
		client: client,
		costs:  make(map[netip.Addr]igpcost.Distance),
	}

	// Healthy state emits no doctor finding.
	findings, err := r.doctorCheck(nextHopMetricsDoctorName)
	require.NoError(t, err)
	require.Empty(t, findings)

	// Episode 1: repeated prospectiveDistance and igpDistance failures stay
	// fail-closed uncached and emit one coalesced error log.
	failLookups.Store(true)
	path := locrib.Path{
		Source:   bgpProtocolID,
		Instance: bgpLocRIBInstance,
		IsBGP:    true,
		NextHop:  netip.MustParseAddr("198.51.100.1"),
	}
	prefix := netip.MustParsePrefix("10.70.0.0/24")
	for range 3 {
		d := r.forkRIB.prospectiveDistance(family.IPv4Unicast, prefix, path)
		require.False(t, d.Resolved)
	}
	for range 2 {
		d := r.igpDistance(netip.MustParseAddr("198.51.100.2"))
		require.False(t, d.Resolved)
	}
	require.Equal(t, int64(5), rpcCalls.Load(), "failed lookups must never be cached")
	require.Equal(t, 1, logs.count(t, "ERROR", codeDoctorBGPNextHopMetrics),
		"repeated failures in one outage episode must log ERROR only once")

	findings, err = r.doctorCheck(nextHopMetricsDoctorName)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	require.Equal(t, codeDoctorBGPNextHopMetrics, findings[0].Code)
	require.Equal(t, "warning", findings[0].Severity)
	require.NotEmpty(t, findings[0].Message)
	require.Equal(t, remoteMetricFix, findings[0].Fix)

	// Recovery clears the doctor warning and re-arms log coalescing.
	failLookups.Store(false)
	recovered := r.forkRIB.prospectiveDistance(family.IPv4Unicast, prefix, path)
	require.True(t, recovered.Resolved, "recovery must succeed immediately because failures were not cached")
	require.Equal(t, 1, logs.count(t, "ERROR", codeDoctorBGPNextHopMetrics))

	findings, err = r.doctorCheck(nextHopMetricsDoctorName)
	require.NoError(t, err)
	require.Empty(t, findings, "recovery must clear doctor-bgp-next-hop-metrics")

	// Episode 2: a fresh failure episode starting from igpDistance logs once
	// more and re-raises the runtime doctor warning.
	failLookups.Store(true)
	callsBeforeEpisode2 := rpcCalls.Load()
	for range 3 {
		d := r.igpDistance(netip.MustParseAddr("198.51.100.3"))
		require.False(t, d.Resolved)
	}
	require.Equal(t, callsBeforeEpisode2+3, rpcCalls.Load(), "second-episode failures must also remain uncached")
	require.Equal(t, 2, logs.count(t, "ERROR", codeDoctorBGPNextHopMetrics),
		"recovery must re-arm error logging so a new outage logs once")

	findings, err = r.doctorCheck(nextHopMetricsDoctorName)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	require.Equal(t, codeDoctorBGPNextHopMetrics, findings[0].Code)
	require.Equal(t, "warning", findings[0].Severity)
	require.Equal(t, remoteMetricFix, findings[0].Fix)

	// Recovery via igpDistance clears the finding again.
	failLookups.Store(false)
	recoveredIGP := r.igpDistance(netip.MustParseAddr("198.51.100.3"))
	require.True(t, recoveredIGP.Resolved)

	findings, err = r.doctorCheck(nextHopMetricsDoctorName)
	require.NoError(t, err)
	require.Empty(t, findings)
}

// TestUnresolvedNextHopOverflowDeduplicationAndEmptyCleanup verifies bounded
// overflow accounting under repeated re-announcements, zero-sample overflow
// formatting, individual withdrawal to an empty live peer, and manual peer
// teardown via inboundEmpty.
//
// VALIDATES: re-announcing an overflowed prefix does not inflate the doctor
// count; zero-sample overflow avoids "(+N more)" formatting; withdrawing all
// routes individually while a peer stays UP clears overflow and the doctor
// warning; inboundEmpty purges manual-peer overflow.
// PREVENTS: unbounded overflow counter inflation under flapping prefixes and
// stuck doctor-bgp-next-hop-unresolvable warnings after routes are gone.
func TestUnresolvedNextHopOverflowDeduplicationAndEmptyCleanup(t *testing.T) {
	t.Run("reannounce_overflowed_prefix_does_not_inflate_count", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)

		peerAddr := netip.MustParseAddr("192.0.2.11")
		nextHop := netip.MustParseAddr("198.51.100.211")
		attrs := aigpSelectionAttrs(nextHop.As4(), nil, 100, 1)
		firstWire := ipv4Prefix(24, 10, 64, 0)
		routes := nextHopReachabilityPath(t, r, 11, peerAddr, nextHop, 100, firstWire)
		r.checkBestPathChange(family.IPv4Unicast, firstWire, false, nil)

		const (
			trackerCap = 4096
			overflow   = 5
		)
		var overflowWires [][]byte
		for i := 1; i < trackerCap+overflow; i++ {
			wire := ipv4Prefix(24, 10, 64+byte(i>>8), byte(i&0xff))
			routes.Insert(family.IPv4Unicast, attrs, wire)
			r.checkBestPathChange(family.IPv4Unicast, wire, false, nil)
			if i >= trackerCap {
				overflowWires = append(overflowWires, wire)
			}
		}

		findings, err := r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Contains(t, findings[0].Message, "4101")

		// Re-announcing the same overflowed prefixes repeatedly must not inflate
		// the reported count.
		updatedAttrs := aigpSelectionAttrs(nextHop.As4(), nil, 200, 1)
		for range 10 {
			for _, wire := range overflowWires {
				routes.Insert(family.IPv4Unicast, updatedAttrs, wire)
				r.checkBestPathChange(family.IPv4Unicast, wire, false, nil)
			}
		}

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Contains(t, findings[0].Message, "4101", "re-announcing overflowed prefixes must not inflate count")
		require.Contains(t, findings[0].Message, "(+4096 more)")
	})

	t.Run("individual_withdraw_all_routes_clears_overflow_while_peer_up", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)

		peerAddr := netip.MustParseAddr("192.0.2.12")
		nextHop := netip.MustParseAddr("198.51.100.212")
		attrs := aigpSelectionAttrs(nextHop.As4(), nil, 100, 1)
		firstWire := ipv4Prefix(24, 10, 64, 0)
		routes := nextHopReachabilityPath(t, r, 12, peerAddr, nextHop, 100, firstWire)
		r.checkBestPathChange(family.IPv4Unicast, firstWire, false, nil)

		const (
			trackerCap = 4096
			overflow   = 5
		)
		wires := make([][]byte, trackerCap+overflow)
		wires[0] = firstWire
		for i := 1; i < trackerCap+overflow; i++ {
			wire := ipv4Prefix(24, 10, 64+byte(i>>8), byte(i&0xff))
			wires[i] = wire
			routes.Insert(family.IPv4Unicast, attrs, wire)
			r.checkBestPathChange(family.IPv4Unicast, wire, false, nil)
		}

		// Withdraw all 4096 tracked entries first while leaving the 5 overflowed
		// routes in the peer's RIB to exercise zero-sample overflow formatting.
		for i := range trackerCap {
			routes.Remove(family.IPv4Unicast, wires[i])
			r.checkBestPathChange(family.IPv4Unicast, wires[i], false, nil)
		}

		findings, err := r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Contains(t, findings[0].Message, "5 BGP routes")
		require.Contains(t, findings[0].Message, "beyond the 4096-entry sample cap")
		require.NotContains(t, findings[0].Message, "more)", "zero-sample overflow must not print (+N more)")

		// Withdrawing the remaining 5 overflowed routes empties the live peer's
		// RIB and must clear overflow and the doctor finding while the peer stays UP.
		for i := trackerCap; i < trackerCap+overflow; i++ {
			routes.Remove(family.IPv4Unicast, wires[i])
			r.checkBestPathChange(family.IPv4Unicast, wires[i], false, nil)
		}

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings, "emptying a live peer via individual withdrawals must clear overflow")
		require.Zero(t, r.unresolvedNextHops.total.Load())
		require.Zero(t, r.unresolvedNextHops.overflow.Load())
	})

	t.Run("inbound_empty_manual_peer_clears_overflow", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)

		peerAddr := netip.MustParseAddr("192.0.2.13")
		nextHop := netip.MustParseAddr("198.51.100.213")
		attrs := aigpSelectionAttrs(nextHop.As4(), nil, 100, 1)
		routes, err := storage.NewAddressPeerRIB(peerAddr)
		require.NoError(t, err)
		r.peerMu.Lock()
		if r.ribInPool[bgpProtocolID] == nil {
			r.ribInPool[bgpProtocolID] = make(map[string]*storage.PeerRIB)
		}
		r.ribInPool[bgpProtocolID][peerAddr.String()] = routes
		r.peerMu.Unlock()

		const (
			trackerCap = 4096
			overflow   = 5
		)
		for i := range trackerCap + overflow {
			wire := ipv4Prefix(24, 10, 64+byte(i>>8), byte(i&0xff))
			routes.Insert(family.IPv4Unicast, attrs, wire)
			r.checkBestPathChange(family.IPv4Unicast, wire, false, nil)
		}

		findings, err := r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Contains(t, findings[0].Message, "4101")

		r.inboundEmpty(peerAddr.String())

		findings, err = r.doctorCheck(unresolvedNextHopDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings, "inboundEmpty on manual peer must purge tracked and overflow state")
		require.Zero(t, r.unresolvedNextHops.total.Load())
		require.Zero(t, r.unresolvedNextHops.overflow.Load())
	})
}
