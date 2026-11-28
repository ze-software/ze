// Design: docs/architecture/route-selection.md -- SRv6 Service SID resolvability before best-path selection
// RFC: rfc/short/rfc9252.md -- Sections 5 and 6 SRv6 Service SID resolvability and local-policy bypass
// Related: rib_next_hop.go, rib_srv6_resolvability_config.go, rib_pipeline_bestfact.go, rib_next_hop_doctor.go

package rib

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// srv6PrefixSIDValue builds a Prefix-SID attribute value carrying one SRv6
// Service TLV (type 5 for L3, type 6 for L2) with a single SID Info Sub-TLV
// and optional SID Structure Sub-Sub-TLV bytes.
func srv6PrefixSIDValue(serviceTLVType byte, sid netip.Addr, sidStructure []byte) []byte {
	sidInfoVal := make([]byte, 0, 21+len(sidStructure))
	sidInfoVal = append(sidInfoVal, 0) // Reserved
	sidInfoVal = append(sidInfoVal, sid.AsSlice()...)
	sidInfoVal = append(sidInfoVal, 0, 0, 0, 0) // Flags, Endpoint Behavior(2), Reserved
	sidInfoVal = append(sidInfoVal, sidStructure...)
	sidInfo := append([]byte{1, byte(len(sidInfoVal) >> 8), byte(len(sidInfoVal))}, sidInfoVal...)

	serviceVal := append([]byte{0}, sidInfo...)
	return append([]byte{serviceTLVType, byte(len(serviceVal) >> 8), byte(len(serviceVal))}, serviceVal...)
}

// appendPrefixSIDAttr appends a BGP Prefix-SID path attribute (code 40) onto
// wire attributes bytes.
func appendPrefixSIDAttr(attrs, prefixSIDVal []byte) []byte {
	out := make([]byte, 0, len(attrs)+3+len(prefixSIDVal))
	out = append(out, attrs...)
	out = append(out, 0xC0, byte(attribute.AttrPrefixSID), byte(len(prefixSIDVal)))
	out = append(out, prefixSIDVal...)
	return out
}

// ipv4SRv6Attrs builds standard iBGP IPv4 unicast wire attributes carrying the
// given IPv4 NEXT_HOP, LOCAL_PREF, and an SRv6 L3 Service SID.
func ipv4SRv6Attrs(nextHop [4]byte, localPref uint32, sid netip.Addr) []byte {
	base := aigpSelectionAttrs(nextHop, nil, localPref, 1)
	return appendPrefixSIDAttr(base, srv6PrefixSIDValue(5, sid, nil))
}

// ipv6IGPRoute builds a terminal IPv6 interior route suitable for resolving
// IPv6 SRv6 locators in a test Loc-RIB.
func ipv6IGPRoute(metric uint32) locrib.Path {
	return locrib.Path{
		Source:    igpTestSource,
		Instance:  1,
		Metric:    metric,
		NextHop:   netip.MustParseAddr("fe80::1"),
		Interface: "eth0",
		RouteType: routetype.Unicast,
	}
}

// ipv6DirectRoute builds a directly connected IPv6 route (zero NextHop) that
// terminates both igpcost.Resolve and sysrib's nhResolver.
func ipv6DirectRoute(metric uint32) locrib.Path {
	return locrib.Path{
		Source:    igpTestSource,
		Instance:  1,
		Metric:    metric,
		Interface: "eth0",
		RouteType: routetype.Unicast,
	}
}

// srv6IPv4Path inserts one IPv4 unicast route carrying an SRv6 Service SID into
// a test peer's Adj-RIB-In.
func srv6IPv4Path(t *testing.T, r *RIBManager, peerOwner uint64, peer, nextHop netip.Addr, localPref uint32, sid netip.Addr, wirePrefix []byte) *storage.PeerRIB {
	t.Helper()
	routes := r.bgpPeers[peerOwner]
	if routes == nil {
		routes = storage.NewPeerRIB(peer.String())
		r.bgpPeers[peerOwner] = routes
		t.Cleanup(routes.Release)
	}
	if r.peerMeta[peerOwner] == nil {
		r.peerMeta[peerOwner] = &peerMetadata{Address: peer, PeerASN: 65001, LocalASN: 65001}
	}
	routes.Insert(family.IPv4Unicast, ipv4SRv6Attrs(nextHop.As4(), localPref, sid), wirePrefix)
	return routes
}

// gatherPrefixCandidatesWithLock calls gatherPrefixCandidatesForLocked under a
// panic-safe r.peerMu.RLock so any panic inside candidate admission releases
// peerMu before t.Cleanup runs.
func gatherPrefixCandidatesWithLock(r *RIBManager, fam family.Family, pfx netip.Prefix, admission candidateAdmission) []*Candidate {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()
	return r.gatherPrefixCandidatesForLocked(fam, pfx, admission)
}

// gatherKeyCandidatesWithLock calls gatherKeyCandidatesForLocked under a
// panic-safe r.peerMu.RLock so any panic inside candidate admission releases
// peerMu before t.Cleanup runs.
func gatherKeyCandidatesWithLock(r *RIBManager, fam family.Family, routeKey []byte, admission candidateAdmission) []*Candidate {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()
	return r.gatherKeyCandidatesForLocked(fam, routeKey, admission)
}

// ipv6UnicastSRv6Update builds a BGP UPDATE body for IPv6 unicast carrying
// MP_REACH_NLRI with an IPv6 next hop and an SRv6 L3 Service SID.
func ipv6UnicastSRv6Update(t *testing.T, nextHop netip.Addr, localPref uint32, sid netip.Addr, prefix string) (body, nlri []byte) {
	t.Helper()
	var err error
	nlri, err = prefixToWire("ipv6/unicast", prefix, 0, false)
	require.NoError(t, err)

	mpReach := []byte{0x00, 0x02, 0x01, 16}
	mpReach = append(mpReach, nextHop.AsSlice()...)
	mpReach = append(mpReach, 0x00)
	mpReach = append(mpReach, nlri...)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x05, 0x04,
	}
	attrs = binary.BigEndian.AppendUint32(attrs, localPref)
	attrs = append(attrs, 0x80, byte(attribute.AttrMPReachNLRI), byte(len(mpReach)))
	attrs = append(attrs, mpReach...)
	attrs = appendPrefixSIDAttr(attrs, srv6PrefixSIDValue(5, sid, nil))

	body = []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))}
	return append(body, attrs...), nlri
}

// vpnv4TransposedSRv6Update builds a VPNv4 BGP UPDATE announcing 10.92.0.0/16
// whose SRv6 SID Structure transposes transposLen bits at transposOffset into
// the 20-bit VPNv4 label field.
func vpnv4TransposedSRv6Update(nextHop [4]byte, partialSID netip.Addr, label uint32, lbl, lnl, fl, al, transposLen, transposOffset byte) (body, nlri []byte) {
	sidStructure := []byte{
		1, 0, 6,
		lbl, lnl, fl, al, transposLen, transposOffset,
	}
	prefixSID := srv6PrefixSIDValue(5, partialSID, sidStructure)

	nlri = []byte{
		24 + 64 + 16, // label(24) + RD(64) + prefix(16)
		byte(label >> 12), byte(label >> 4), byte(label<<4) | 0x01,
		0, 0, 0, 0, 0, 0, 0, 7, // RD 0:7
		10, 92, // 10.92.0.0/16
	}

	mpReach := []byte{
		0x00, 0x01, 0x80, 12,
		0, 0, 0, 0, 0, 0, 0, 0, nextHop[0], nextHop[1], nextHop[2], nextHop[3],
		0x00,
	}
	mpReach = append(mpReach, nlri...)

	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00}
	attrs = append(attrs, 0x80, byte(attribute.AttrMPReachNLRI), byte(len(mpReach)))
	attrs = append(attrs, mpReach...)
	attrs = appendPrefixSIDAttr(attrs, prefixSID)

	body = []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))}
	return append(body, attrs...), nlri
}

// evpnSRv6Update builds an EVPN Type-2 BGP UPDATE carrying an SRv6 L2 Service
// TLV (type 6) with non-transposed Service SID.
func evpnSRv6Update(nextHop [4]byte, sid netip.Addr, nlri []byte) []byte {
	mpReach := []byte{0x00, 0x19, 0x46, 4, nextHop[0], nextHop[1], nextHop[2], nextHop[3], 0x00}
	mpReach = append(mpReach, nlri...)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9, // AS_PATH = [65001]
	}
	attrs = append(attrs, 0x80, byte(attribute.AttrMPReachNLRI), byte(len(mpReach)))
	attrs = append(attrs, mpReach...)
	attrs = appendPrefixSIDAttr(attrs, srv6PrefixSIDValue(6, sid, nil))

	body := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))}
	return append(body, attrs...)
}

// TestGatherCandidatesSkipsUnresolvableSRv6SID proves AC-1: a path whose
// reconstructed SRv6 Service SID has no resolving route in the Loc-RIB is
// excluded before Phase 2 ranking, stays retained in Adj-RIB-In, stays visible
// to recovery admission, and increments the Prometheus exclusion counter.
//
// VALIDATES: RFC 9252 Section 5 -- candidateSRv6SIDResolved excludes a path
// with an unreachable SRv6 Service SID even when its BGP NEXT_HOP resolves.
// PREVENTS: selecting and installing an unprogrammable SRv6 path as BGP best.
// MUTATION: make candidateSRv6SIDResolved return (sid, true) unconditionally.
//
// RFC requirement: RFC9252-5-2 negative -- a path whose SRv6 Service SID has no resolving route in the Loc-RIB is excluded from best-path candidate selection
func TestGatherCandidatesSkipsUnresolvableSRv6SID(t *testing.T) {
	oldMetrics := metricsPtr.Load()
	defer metricsPtr.Store(oldMetrics)
	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)

	r, loc := nextHopReachabilityRIB(t)
	nextHop := netip.MustParseAddr("198.51.100.1")
	sid := netip.MustParseAddr("fc00:10::1")
	prefix := netip.MustParsePrefix("10.92.1.0/24")
	wirePrefix := ipv4Prefix(24, 10, 92, 1)

	// BGP NEXT_HOP resolves, while the SRv6 Service SID has no IPv6 cover.
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))
	require.True(t, igpcost.Resolve(loc, nextHop).Resolved)
	require.False(t, igpcost.Resolve(loc, sid).Resolved)

	routes := srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 200, sid, wirePrefix)

	forwarding := gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, prefix, candidateAdmissionForwarding)
	recovery := gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, prefix, candidateAdmissionRecovery)
	defer releaseCandidates(forwarding)
	defer releaseCandidates(recovery)

	require.Empty(t, forwarding, "unresolvable SRv6 Service SID must be excluded from forwarding candidacy")
	require.Len(t, recovery, 1, "recovery admission must preserve retained paths without local forwarding resolution")

	_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
	require.False(t, changed, "excluded SRv6 path must not produce a best-path change")
	_, found := loc.Best(family.IPv4Unicast, prefix)
	require.False(t, found, "excluded SRv6 path must not enter Loc-RIB")
	require.Equal(t, 1, routes.Len(), "excluded SRv6 path must stay retained in Adj-RIB-In")

	scraped := scrapeRIBMetrics(t, reg)
	require.Contains(t, scraped, `ze_rib_srv6_sid_unresolvable_exclusions_total{family="ipv4/unicast"} `)
}

// TestGatherCandidatesAdmitsResolvableSRv6SID proves AC-5: a path carrying a
// valid SRv6 Service SID whose address resolves in the Loc-RIB is admitted to
// Phase 2 best-path selection and installs with its Service SID.
//
// VALIDATES: RFC 9252 Section 5 -- a resolvable SRv6 Service SID passes
// candidateSRv6SIDResolved and enters Loc-RIB as the winning BGP path.
// PREVENTS: overly aggressive SRv6 filtering dropping valid reachable paths.
// MUTATION: make candidateSRv6SIDResolved return (sid, false) for all SRv6 paths.
//
// RFC requirement: RFC9252-5-2 positive -- a path whose SRv6 Service SID resolves in the Loc-RIB is admitted to best-path candidate selection
func TestGatherCandidatesAdmitsResolvableSRv6SID(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	nextHop := netip.MustParseAddr("198.51.100.1")
	sid := netip.MustParseAddr("fc00:11::1")
	prefix := netip.MustParsePrefix("10.92.11.0/24")
	wirePrefix := ipv4Prefix(24, 10, 92, 11)

	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))
	loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:11::/48"), ipv6IGPRoute(10))
	require.True(t, igpcost.Resolve(loc, nextHop).Resolved)
	require.True(t, igpcost.Resolve(loc, sid).Resolved)

	srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 150, sid, wirePrefix)

	candidates := gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, prefix, candidateAdmissionForwarding)
	defer releaseCandidates(candidates)
	require.Len(t, candidates, 1, "resolvable SRv6 Service SID must be admitted to candidacy")

	_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
	require.True(t, changed)

	best, found := loc.Best(family.IPv4Unicast, prefix)
	require.True(t, found)
	require.True(t, best.IsBGP)
	require.Equal(t, nextHop, best.NextHop)
	require.Equal(t, sid, best.SRv6SID)
}

// TestUnresolvableSRv6LosesToNonSRv6Path proves AC-2: an unresolvable SRv6 path
// with higher LOCAL_PREF (200) loses to a reachable lower-LOCAL_PREF (100)
// non-SRv6 runner-up, and only the runner-up enters Loc-RIB and advertise.
//
// VALIDATES: pre-ranking exclusion allows reachable fallback BGP paths to win
// and be advertised instead of leaving the prefix dark.
// PREVENTS: unresolvable higher-LOCAL_PREF SRv6 paths shadowing reachable paths.
func TestUnresolvableSRv6LosesToNonSRv6Path(t *testing.T) {
	r, calls := sourceLeaseTestRIB(t, true)
	calls.mu.Lock()
	calls.issued = map[uint64]uint64{1: 1, 2: 2}
	calls.mu.Unlock()
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	igpcost.Set(func(addr netip.Addr) igpcost.Distance { return igpcost.Resolve(loc, addr) })
	t.Cleanup(func() { igpcost.Set(nil) })

	nhSRv6 := netip.MustParseAddr("198.51.100.1")
	nhFallback := netip.MustParseAddr("198.51.100.2")
	sid := netip.MustParseAddr("fc00:20::1")
	prefix := netip.MustParsePrefix("10.92.20.0/24")
	wirePrefix := ipv4Prefix(24, 10, 92, 20)

	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))
	require.False(t, igpcost.Resolve(loc, sid).Resolved)

	srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nhSRv6, 200, sid, wirePrefix)
	r.peerUp[1] = true
	advertiseCaptureReceipt(t, r, 1, family.IPv4Unicast, wirePrefix, 1, 41)
	nextHopReachabilityPath(t, r, 2, netip.MustParseAddr("192.0.2.2"), nhFallback, 100, wirePrefix)
	r.peerUp[2] = true
	advertiseCaptureReceipt(t, r, 2, family.IPv4Unicast, wirePrefix, 2, 42)

	change, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
	require.True(t, changed)
	require.Equal(t, nhFallback, change.NextHop)
	require.False(t, change.SRv6SID.IsValid())

	best, found := loc.Best(family.IPv4Unicast, prefix)
	require.True(t, found)
	require.True(t, best.IsBGP)
	require.Equal(t, nhFallback, best.NextHop)
	require.False(t, best.SRv6SID.IsValid())

	adv, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, adv, 1)
	require.Equal(t, prefix, adv[0].key.prefix)
	require.Equal(t, netip.MustParseAddr("192.0.2.2"), adv[0].route.Source)
}

// TestLocRIBChangeReevaluatesSRv6Prefix proves AC-3: adding a covering IPv6
// locator route to the Loc-RIB triggers runAIGPSelection -> reselectAIGPRoutes
// and promotes a previously excluded retained SRv6 path with no new BGP UPDATE.
//
// VALIDATES: recovery from full exclusion (no prior bestPrev record) via the
// production Loc-RIB OnChange worker without receiving a new BGP UPDATE.
// PREVENTS: retained SRv6 paths remaining permanently dark after locator up.
func TestLocRIBChangeReevaluatesSRv6Prefix(t *testing.T) {
	registerBuiltinCommands()
	r, loc := nextHopReachabilityRIB(t)
	nextHop := netip.MustParseAddr("198.51.100.1")
	sid := netip.MustParseAddr("fc00:30::1")
	prefix := netip.MustParsePrefix("10.92.30.0/24")
	wirePrefix := ipv4Prefix(24, 10, 92, 30)

	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))
	routes := srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 200, sid, wirePrefix)
	startNextHopReachabilitySelection(t, r)

	// Initial worker scan excludes the unresolvable SRv6 path and records doctor state.
	require.Eventually(t, func() bool {
		findings, err := r.doctorCheck(unresolvedSRv6SIDDoctorName)
		return err == nil && len(findings) == 1
	}, 5*time.Second, 10*time.Millisecond)
	_, found := loc.Best(family.IPv4Unicast, prefix)
	require.False(t, found)
	require.Equal(t, 1, routes.Len())

	// Add the covering IPv6 locator route with no new BGP UPDATE.
	loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:30::/48"), ipv6IGPRoute(10))

	require.Eventually(t, func() bool {
		best, ok := loc.Best(family.IPv4Unicast, prefix)
		if !ok {
			return false
		}
		if !best.IsBGP {
			return false
		}
		return best.SRv6SID == sid
	}, 5*time.Second, 10*time.Millisecond, "adding covering IPv6 locator must promote retained SRv6 path without UPDATE")

	findings, err := r.doctorCheck(unresolvedSRv6SIDDoctorName)
	require.NoError(t, err)
	require.Empty(t, findings, "locator restoration must clear doctor warning")

	rows := showBGPRIBRows(t, r, "received")
	row := findRowByPrefix(t, rows, prefix.String())
	_, hasResolvable := row[rowKeySRv6SIDResolvable]
	require.False(t, hasResolvable)
	_, hasReason := row[rowKeyIneligibleReason]
	require.False(t, hasReason)
}

// TestLocRIBRemovalReevaluatesSRv6Prefix proves AC-4: removing the covering
// IPv6 locator route withdraws a lone winning SRv6 path from Loc-RIB and fails
// over a multi-path prefix to the reachable runner-up with no BGP UPDATE.
//
// VALIDATES: Loc-RIB locator removal triggers automatic SRv6 failover and
// withdrawal while retaining the Adj-RIB-In entry for future recovery.
// PREVENTS: stale unprogrammable SRv6 routes staying selected after locator loss.
func TestLocRIBRemovalReevaluatesSRv6Prefix(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	nhSRv6 := netip.MustParseAddr("198.51.100.1")
	nhFallback := netip.MustParseAddr("198.51.100.2")
	sid := netip.MustParseAddr("fc00:40::1")
	cover := netip.MustParsePrefix("fc00:40::/48")

	failoverPrefix := netip.MustParsePrefix("10.92.40.0/24")
	failoverWire := ipv4Prefix(24, 10, 92, 40)
	lonePrefix := netip.MustParsePrefix("10.92.41.0/24")
	loneWire := ipv4Prefix(24, 10, 92, 41)

	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))
	loc.Insert(family.IPv6Unicast, cover, ipv6IGPRoute(10))

	srv6Routes := srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nhSRv6, 200, sid, failoverWire)
	srv6Routes.Insert(family.IPv4Unicast, ipv4SRv6Attrs(nhSRv6.As4(), 200, sid), loneWire)
	nextHopReachabilityPath(t, r, 2, netip.MustParseAddr("192.0.2.2"), nhFallback, 100, failoverWire)

	startNextHopReachabilitySelection(t, r)

	require.Eventually(t, func() bool {
		b1, ok1 := loc.Best(family.IPv4Unicast, failoverPrefix)
		if !ok1 {
			return false
		}
		b2, ok2 := loc.Best(family.IPv4Unicast, lonePrefix)
		if !ok2 {
			return false
		}
		if b1.SRv6SID != sid {
			return false
		}
		return b2.SRv6SID == sid
	}, 5*time.Second, 10*time.Millisecond, "both SRv6 routes must initially win while locator resolves")

	_, removed := loc.Remove(family.IPv6Unicast, cover, igpTestSource, 1)
	require.True(t, removed)

	require.Eventually(t, func() bool {
		b1, ok1 := loc.Best(family.IPv4Unicast, failoverPrefix)
		if !ok1 {
			return false
		}
		if b1.NextHop != nhFallback {
			return false
		}
		if b1.SRv6SID.IsValid() {
			return false
		}
		_, ok2 := loc.Best(family.IPv4Unicast, lonePrefix)
		return !ok2
	}, 5*time.Second, 10*time.Millisecond, "removing locator must fail over multi-path prefix and withdraw lone SRv6 prefix")

	require.Equal(t, 2, srv6Routes.Len(), "withdrawn SRv6 paths must stay retained in Adj-RIB-In")
}

// TestSRv6ResolvabilityProspectiveSelfCaptureAndCrossFamily proves A-4, AC-1,
// and AC-5: an IPv6 unicast route whose own prefix would capture its SRv6
// Service SID is excluded until a more-specific cover protects the SID, while
// an IPv4 unicast route carrying an IPv6 SRv6 SID resolves cleanly against the
// IPv6 Loc-RIB without IPv4 prospective overlay interference.
//
// VALIDATES: prospective self-capture detection for IPv6 unicast SRv6 routes
// and cross-family IPv4-over-SRv6 resolution through ResolveProspective.
// PREVENTS: IPv6 SRv6 encapsulation loops and cross-family overlay breakage.
// RFC requirement: RFC9252-5-2 negative -- an IPv6 unicast SRv6 path whose own prefix would prospectively capture its Service SID is excluded before best-path per RFC 4271
// RFC requirement: RFC9252-5-2 positive -- the same IPv6 unicast SRv6 path is admitted once a more-specific cover protects its Service SID, and cross-family IPv4-over-SRv6 resolves against IPv6 Loc-RIB
func TestSRv6ResolvabilityProspectiveSelfCaptureAndCrossFamily(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	// Case 1: IPv6 unicast route fc00:50:1::/48 with SID fc00:50:1::100.
	// Less-specific cover fc00:50::/32 exists, plus reachable BGP NEXT_HOP.
	bgpNHv6 := netip.MustParseAddr("2001:db8:99::1")
	loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:99::/48"), ipv6IGPRoute(10))
	loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:50::/32"), ipv6IGPRoute(10))

	selfSID := netip.MustParseAddr("fc00:50:1::100")
	selfPrefix := netip.MustParsePrefix("fc00:50:1::/48")
	body, selfNLRI := ipv6UnicastSRv6Update(t, bgpNHv6, 150, selfSID, selfPrefix.String())
	feedReceived(r, 1, netip.MustParseAddr("2001:db8::1"), ctxID, body)

	_, found := loc.Best(family.IPv6Unicast, selfPrefix)
	require.False(t, found, "IPv6 route that would capture its own SRv6 Service SID must be excluded")

	// Adding a more-specific IGP cover protects the Service SID from capture.
	loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:50:1::100/128"), ipv6IGPRoute(5))
	_, changed := r.checkBestPathChange(family.IPv6Unicast, selfNLRI, false, nil)
	require.True(t, changed)
	bestV6, found := loc.Best(family.IPv6Unicast, selfPrefix)
	require.True(t, found)
	require.Equal(t, selfSID, bestV6.SRv6SID)

	// Case 2: Cross-family IPv4 unicast carrying IPv6 SRv6 Service SID.
	nhV4 := netip.MustParseAddr("198.51.100.1")
	crossSID := netip.MustParseAddr("fc00:51::1")
	crossPrefix := netip.MustParsePrefix("10.92.51.0/24")
	crossWire := ipv4Prefix(24, 10, 92, 51)
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))
	loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:51::/48"), ipv6IGPRoute(10))

	srv6IPv4Path(t, r, 2, netip.MustParseAddr("192.0.2.2"), nhV4, 150, crossSID, crossWire)
	_, changed = r.checkBestPathChange(family.IPv4Unicast, crossWire, false, nil)
	require.True(t, changed)
	bestV4, found := loc.Best(family.IPv4Unicast, crossPrefix)
	require.True(t, found)
	require.Equal(t, crossSID, bestV4.SRv6SID)
}

// TestResolvabilityUsesMergedTransposedSID proves AC-8 and A-2: when a VPNv4
// SRv6 transposition window overlaps locator bits, resolvability checks the
// MERGED Service SID reconstructed by entrySRv6SID, never the partial wire SID.
//
// VALIDATES: candidateSRv6SIDResolved checks the merged transposed SID rather
// than the partial Prefix-SID wire address.
// PREVENTS: false admission or false exclusion when transposition touches
// locator node bits (RFC 9252 Section 3.2.1).
// RFC requirement: RFC9252-5-2 negative -- a transposed VPNv4 SRv6 path whose partial wire SID has a cover but whose reconstructed merged Service SID has no cover is excluded in gatherKeyCandidatesForLocked
// RFC requirement: RFC9252-5-2 positive -- the same transposed VPNv4 SRv6 path is admitted in gatherKeyCandidatesForLocked once a cover resolves the reconstructed merged Service SID
func TestResolvabilityUsesMergedTransposedSID(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	peer := netip.MustParseAddr("192.0.2.6")
	nhV4 := [4]byte{198, 51, 100, 6}

	// LBL=32, LNL=16, FL=16, AL=0; TransposOffset=32 overlaps Locator Node bits!
	// Partial wire SID: fc00:60:0000::1; label carries 0x0042 in high 16 of 20 bits.
	// Reconstructed merged SID: fc00:60:0042::1.
	partialSID := netip.MustParseAddr("fc00:60:0000::1")
	mergedSID := netip.MustParseAddr("fc00:60:0042::1")
	const transposedNode uint32 = 0x0042
	label := transposedNode << 4

	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))
	// Cover ONLY the partial wire SID prefix, NOT the merged SID prefix.
	partialCover := netip.MustParsePrefix("fc00:60:0000::/48")
	mergedCover := netip.MustParsePrefix("fc00:60:0042::/48")
	loc.Insert(family.IPv6Unicast, partialCover, ipv6IGPRoute(10))

	body, nlri := vpnv4TransposedSRv6Update(nhV4, partialSID, label, 32, 16, 16, 0, 16, 32)
	feedReceived(r, 1, peer, ctxID, body)

	require.Equal(t, mergedSID, storedSRv6SID(t, r, ipv4VPN, nlri, 1, peer),
		"entrySRv6SID must reconstruct the merged SID with locator node bits restored")

	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	routeKey, ok := routeIdentity(ipv4VPN, nlri, false, false, scratch[:])
	require.True(t, ok)

	excluded := gatherKeyCandidatesWithLock(r, ipv4VPN, routeKey, candidateAdmissionForwarding)
	releaseCandidates(excluded)
	require.Empty(t, excluded, "covering only the partial wire SID must NOT admit the transposed VPNv4 path")

	// Replace with cover for ONLY the merged SID prefix.
	_, removed := loc.Remove(family.IPv6Unicast, partialCover, igpTestSource, 1)
	require.True(t, removed)
	loc.Insert(family.IPv6Unicast, mergedCover, ipv6IGPRoute(10))

	admitted := gatherKeyCandidatesWithLock(r, ipv4VPN, routeKey, candidateAdmissionForwarding)
	defer releaseCandidates(admitted)
	require.Len(t, admitted, 1, "covering the merged transposed SID must admit the VPNv4 path")
}

// TestEVPNUnresolvableSRv6SIDIneligible proves AC-10 and A-6: an EVPN
// (l2vpn/evpn) path carrying a non-transposed SRv6 Service SID (L2 Service TLV
// type 6) is excluded before best-path selection when its SID is unreachable
// and recovers once a covering IPv6 locator route resolves it.
//
// VALIDATES: RFC 9252 Section 6 -- EVPN over SRv6 enforces Service SID
// resolvability in gatherKeyCandidatesForLocked.
// PREVENTS: non-CIDR EVPN routes bypassing RFC 9252 Section 6 resolvability.
func TestEVPNUnresolvableSRv6SIDIneligible(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	peer := netip.MustParseAddr("192.0.2.7")
	nhV4 := [4]byte{198, 51, 100, 7}
	sid := netip.MustParseAddr("fc00:70::1")

	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))

	nlri := evpnMACRoute(1, 100)
	body := evpnSRv6Update(nhV4, sid, nlri)
	feedReceived(r, 1, peer, ctxID, body)

	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	routeKey, ok := routeIdentity(evpnFamily, nlri, false, false, scratch[:])
	require.True(t, ok)

	excluded := gatherKeyCandidatesWithLock(r, evpnFamily, routeKey, candidateAdmissionForwarding)
	releaseCandidates(excluded)
	require.Empty(t, excluded, "EVPN route with unresolvable SRv6 Service SID must be excluded (RFC 9252 Section 6)")

	findings, err := r.doctorCheck(unresolvedSRv6SIDDoctorName)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	require.Contains(t, findings[0].Message, sid.String())

	// Install covering IPv6 locator and verify recovery.
	loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:70::/48"), ipv6IGPRoute(10))
	_, changed := r.checkBestPathChange(evpnFamily, nlri, false, nil)
	require.True(t, changed, "EVPN SRv6 route must enter selection once its locator resolves")

	admitted := gatherKeyCandidatesWithLock(r, evpnFamily, routeKey, candidateAdmissionForwarding)
	defer releaseCandidates(admitted)
	require.Len(t, admitted, 1)

	findings, err = r.doctorCheck(unresolvedSRv6SIDDoctorName)
	require.NoError(t, err)
	require.Empty(t, findings)
}

// TestSRv6ResolvabilityRecursionDepthAndDiscard proves A-1 and the recursion
// depth boundary: an 8-hop recursive IPv6 locator chain resolves, a 9-hop chain
// fails, discard routes (blackhole/unreachable/prohibit) fail, and self/mutual
// loops fail — matching sysrib's 8-hop nhResolver semantics.
//
// VALIDATES: numeric boundary at 8 vs 9 recursion hops plus discard and loop
// rejection for SRv6 Service SID resolvability.
// PREVENTS: infinite recursion or disagreement between bgp-rib and sysrib.
// RFC requirement: RFC9252-5-2 positive -- an 8-hop recursive IPv6 locator chain resolves and admits the SRv6 path
// RFC requirement: RFC9252-5-2 negative -- a 9-hop chain, discard covers (blackhole/unreachable/prohibit), and self/mutual recursion loops exclude the SRv6 path
func TestSRv6ResolvabilityRecursionDepthAndDiscard(t *testing.T) {
	setupChain := func(t *testing.T, hops int) (*RIBManager, *locrib.RIB, netip.Prefix, netip.Addr) {
		t.Helper()
		r, loc := nextHopReachabilityRIB(t)
		nhV4 := netip.MustParseAddr("198.51.100.1")
		loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))

		// Build a chain of `hops` LPM lookups: hop 0 covers the SID, hops
		// 0..hops-2 recurse to hop i+1, and hop hops-1 is terminal direct.
		hopAddr := func(idx int) netip.Addr {
			var b [16]byte
			b[0], b[1] = 0xfc, 0x00
			b[2], b[3] = 0x00, 0x80
			b[4], b[5] = 0x00, byte(idx+1)
			b[15] = 1
			return netip.AddrFrom16(b)
		}
		for i := range hops {
			pfx := netip.PrefixFrom(hopAddr(i), 64).Masked()
			if i == hops-1 {
				loc.Insert(family.IPv6Unicast, pfx, ipv6DirectRoute(1))
			} else {
				rec := ipv6DirectRoute(1)
				rec.MetricRecursive = true
				rec.NextHop = hopAddr(i + 1)
				loc.Insert(family.IPv6Unicast, pfx, rec)
			}
		}

		sid := hopAddr(0)
		prefix := netip.MustParsePrefix("10.92.80.0/24")
		wirePrefix := ipv4Prefix(24, 10, 92, 80)
		srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nhV4, 100, sid, wirePrefix)
		return r, loc, prefix, sid
	}

	t.Run("depth_8_resolves", func(t *testing.T) {
		r, loc, prefix, sid := setupChain(t, 8)
		require.True(t, igpcost.Resolve(loc, sid).Resolved)

		cands := gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, prefix, candidateAdmissionForwarding)
		defer releaseCandidates(cands)
		require.Len(t, cands, 1, "8-hop recursive IPv6 locator chain must resolve and admit SRv6 path")
	})

	t.Run("depth_9_fails", func(t *testing.T) {
		r, loc, prefix, sid := setupChain(t, 9)
		require.False(t, igpcost.Resolve(loc, sid).Resolved)

		cands := gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, prefix, candidateAdmissionForwarding)
		releaseCandidates(cands)
		require.Empty(t, cands, "9-hop recursive IPv6 locator chain exceeds 8-hop cap and must exclude SRv6 path")
	})

	for _, rt := range []routetype.Type{routetype.Blackhole, routetype.Unreachable, routetype.Prohibit} {
		t.Run("discard_"+rt.String(), func(t *testing.T) {
			r, loc := nextHopReachabilityRIB(t)
			nhV4 := netip.MustParseAddr("198.51.100.1")
			sid := netip.MustParseAddr("fc00:81::1")
			prefix := netip.MustParsePrefix("10.92.81.0/24")
			wirePrefix := ipv4Prefix(24, 10, 92, 81)

			loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))
			discard := ipv6DirectRoute(0)
			discard.RouteType = rt
			loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:81::/48"), discard)

			srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nhV4, 100, sid, wirePrefix)
			cands := gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, prefix, candidateAdmissionForwarding)
			releaseCandidates(cands)
			require.Empty(t, cands, "discard cover %s must not resolve SRv6 Service SID", rt)
		})
	}

	t.Run("self_loop_and_mutual_loop_fail", func(t *testing.T) {
		for _, mutual := range []bool{false, true} {
			r, loc := nextHopReachabilityRIB(t)
			nhV4 := netip.MustParseAddr("198.51.100.1")
			sid := netip.MustParseAddr("fc00:82::1")
			prefix := netip.MustParsePrefix("10.92.82.0/24")
			wirePrefix := ipv4Prefix(24, 10, 92, 82)
			loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))

			rec := ipv6DirectRoute(0)
			rec.MetricRecursive = true
			if mutual {
				peerHop := netip.MustParseAddr("fc00:83::1")
				rec.NextHop = peerHop
				back := rec
				back.NextHop = sid
				loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:83::/48"), back)
			} else {
				rec.NextHop = sid
			}
			loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:82::/48"), rec)

			srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nhV4, 100, sid, wirePrefix)
			cands := gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, prefix, candidateAdmissionForwarding)
			releaseCandidates(cands)
			require.Empty(t, cands, "looping IPv6 locator (mutual=%v) must exclude SRv6 path", mutual)
		}
	})
}

// TestForkedRIBSRv6SIDResolvability proves AC-6 and R-3: in forked placement
// using engine route-metrics RPCs, SRv6 Service SID resolvability excludes
// unreachable SIDs, promotes on revision advance, fails closed on RPC error,
// and serves show bgp rib from cache during an outage with zero blocking RPCs.
//
// VALIDATES: forked bgp-rib parity for SRv6 SID admission, recovery, fail-closed
// RPC handling, and cached show bgp rib annotations during outages.
// PREVENTS: forked bgp-rib bypassing RFC 9252 Section 5 or stalling show bgp rib.
func TestForkedRIBSRv6SIDResolvability(t *testing.T) {
	registerBuiltinCommands()

	var sid1Resolved atomic.Bool
	var failLookups atomic.Bool
	var blockLookups atomic.Bool
	var rpcCalls atomic.Int64

	nhV4 := netip.MustParseAddr("198.51.100.1")
	sid1 := netip.MustParseAddr("fc00:90::1")
	sid2 := netip.MustParseAddr("fc00:90::2")

	clientSide, engineSide := net.Pipe()
	bridge := rpc.NewDirectBridge()
	bridge.SetDeliverEvents(func([]string) error { return nil })
	bridge.SetDispatchRPC(func(ctx context.Context, method string, body json.RawMessage) (json.RawMessage, error) {
		if method != rpc.MethodRouteMetrics {
			return nil, errors.New("unexpected engine RPC method: " + method)
		}
		rpcCalls.Add(1)
		if blockLookups.Load() {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		if failLookups.Load() {
			return nil, errors.New("controlled engine route-metrics failure")
		}
		var input rpc.RouteMetricsInput
		if err := json.Unmarshal(body, &input); err != nil {
			return nil, err
		}
		distances := make([]rpc.RouteMetric, len(input.Addresses))
		for i, addrText := range input.Addresses {
			addr := netip.MustParseAddr(addrText)
			switch addr {
			case nhV4:
				distances[i] = rpc.RouteMetric{Resolved: true, Cost: 10}
			case sid1:
				distances[i] = rpc.RouteMetric{Resolved: sid1Resolved.Load(), Cost: 10}
			default:
				distances[i] = rpc.RouteMetric{Resolved: false}
			}
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

	pfx1 := netip.MustParsePrefix("10.92.90.0/24")
	wire1 := ipv4Prefix(24, 10, 92, 90)
	wire2 := ipv4Prefix(24, 10, 92, 91)

	routes := srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nhV4, 100, sid1, wire1)
	routes.Insert(family.IPv4Unicast, ipv4SRv6Attrs(nhV4.As4(), 100, sid2), wire2)

	// Step 1: sid1 is initially unresolved -> excluded and annotated in show bgp rib.
	cands := gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, pfx1, candidateAdmissionForwarding)
	releaseCandidates(cands)
	require.Empty(t, cands, "forked RIB must exclude unresolvable SRv6 Service SID")

	rows := showBGPRIBRows(t, r, "received")
	row1 := findRowByPrefix(t, rows, "10.92.90.0/24")
	require.Equal(t, false, row1[rowKeySRv6SIDResolvable])
	require.Equal(t, reasonSRv6SIDUnresolvable, row1[rowKeyIneligibleReason])

	// Step 2: Engine resolves sid1; simulate runRemoteMetrics cache clear on revision advance.
	sid1Resolved.Store(true)
	r.forkRIB.mu.Lock()
	clear(r.forkRIB.costs)
	clear(r.forkRIB.prospective)
	r.forkRIB.mu.Unlock()

	cands = gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, pfx1, candidateAdmissionForwarding)
	require.Len(t, cands, 1, "forked RIB must admit SRv6 Service SID once engine resolves it")
	releaseCandidates(cands)

	// Warm ONLY the BGP NEXT_HOP cache for pfx2 while the engine is healthy so
	// Step 3 and Step 4 isolate the SRv6 Service SID RPC failure and cache miss.
	pfx2 := netip.MustParsePrefix("10.92.91.0/24")
	warmNHPfx2 := r.forkRIB.prospectiveDistance(family.IPv4Unicast, pfx2, locrib.Path{
		Source:   bgpProtocolID,
		Instance: bgpLocRIBInstance,
		NextHop:  nhV4,
		IsBGP:    true,
		IsEBGP:   false,
	})
	require.True(t, warmNHPfx2.Resolved)

	// Step 3: Trigger an RPC outage on sid2 -> fail-closed & metricFailureActive = true.
	failLookups.Store(true)
	cands2 := gatherPrefixCandidatesWithLock(r, family.IPv4Unicast, pfx2, candidateAdmissionForwarding)
	releaseCandidates(cands2)
	require.Empty(t, cands2, "forked RIB must fail closed when route-metrics RPC errors")
	require.True(t, r.forkRIB.metricFailureActive())

	// Step 4: Block RPCs and verify show bgp rib uses cache without issuing RPCs.
	blockLookups.Store(true)
	callsBeforeShow := rpcCalls.Load()
	rowsDuringOutage := showBGPRIBRows(t, r, "received")
	require.Equal(t, callsBeforeShow, rpcCalls.Load(),
		"show bgp rib must read cached SRv6 SID metrics without RPCs during an active outage")

	cachedRow := findRowByPrefix(t, rowsDuringOutage, "10.92.90.0/24")
	_, hasRes := cachedRow[rowKeySRv6SIDResolvable]
	require.False(t, hasRes, "cached resolvable SRv6 row must remain clean during outage")
	_, hasReason := cachedRow[rowKeyIneligibleReason]
	require.False(t, hasReason)

	uncachedRow := findRowByPrefix(t, rowsDuringOutage, "10.92.91.0/24")
	require.Equal(t, false, uncachedRow[rowKeySRv6SIDResolvable])
	require.Equal(t, reasonSRv6SIDUnresolvable, uncachedRow[rowKeyIneligibleReason])
}

// TestSRv6ResolvabilityBypassConfigAndReload proves AC-11, AC-9, and R-5:
// precedence across all 6 scopes, dynamic-group resolution, strict bool/family
// validation, ConfigTx reload and rollback reselection, and preservation of
// RFC 9252 Section 7 semantic validity (RFC9252-5-1) and RFC 4271 NEXT_HOP
// resolvability even when ignore is true.
//
// VALIDATES: srv6-sid-resolvability YANG config parsing, inheritance, reload,
// rollback, and non-bypass of RFC9252-5-1 and NEXT_HOP checks.
// PREVENTS: precedence inversion, invalid values silently accepted, stale
// selections across reload/rollback, or bypass leaking into other checks.
func TestSRv6ResolvabilityBypassConfigAndReload(t *testing.T) {
	family.RegisterTestFamilies()

	t.Run("precedence_and_dynamic_groups", func(t *testing.T) {
		r := newRIBManager(nil)
		tx := r.newConfigTx(false)

		// Default (unconfigured) enforces strict resolvability everywhere.
		r.peerMu.RLock()
		require.False(t, r.srv6SIDResolvabilityIgnoredLocked(1, "192.0.2.10", family.IPv4Unicast))
		r.peerMu.RUnlock()

		// Configure all 6 precedence levels plus a dynamic group, mixing JSON
		// bools and delivered string booleans ("true"/"false").
		bgpJSON := `
			"srv6-sid-resolvability": {
				"ignore": "false",
				"family": {
					"ipv4/mpls-vpn": {"ignore": "true"},
					"l2vpn/evpn": {"ignore": true}
				}
			},
			"group": {
				"core": {
					"srv6-sid-resolvability": {
						"ignore": false,
						"family": {
							"ipv6/unicast": {"ignore": "true"}
						}
					},
					"peer": {
						"pe1": {
							"connection": {"remote": {"ip": "192.0.2.10"}},
							"srv6-sid-resolvability": {
								"ignore": "true",
								"family": {
									"ipv4/unicast": {"ignore": "false"}
								}
							}
						},
						"pe2": {
							"connection": {"remote": {"ip": "192.0.2.11"}}
						}
					}
				},
				"dyn-ix": {
					"connection": {"remote": {"ip": "dynamic"}},
					"srv6-sid-resolvability": {
						"ignore": "true",
						"family": {
							"l2vpn/evpn": {"ignore": "false"}
						}
					}
				}
			},
			"peer": {
				"standalone": {
					"connection": {"remote": {"ip": "192.0.2.20"}}
				}
			}
		`
		require.NoError(t, tx.Configure(ribSections(bgpJSON)))

		r.peerMeta[1] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.10"), GroupName: "core"}
		r.peerMeta[2] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.11"), GroupName: "core"}
		r.peerMeta[3] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.99"), GroupName: "dyn-ix"}
		r.peerMeta[4] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.20")}

		r.peerMu.RLock()
		defer r.peerMu.RUnlock()

		// pe1: peer family (ipv4/unicast=false) beats peer (ignore=true);
		// peer (ignore=true) beats group (ignore=false) and bgp.
		require.False(t, r.srv6SIDResolvabilityIgnoredLocked(1, "192.0.2.10", family.IPv4Unicast))
		require.True(t, r.srv6SIDResolvabilityIgnoredLocked(1, "192.0.2.10", family.IPv6Unicast))
		require.True(t, r.srv6SIDResolvabilityIgnoredLocked(1, "192.0.2.10", ipv4VPN))

		// pe2: inherits group family (ipv6/unicast=true) > group (ignore=false) > bgp family (ipv4/mpls-vpn=true)!
		require.True(t, r.srv6SIDResolvabilityIgnoredLocked(2, "192.0.2.11", family.IPv6Unicast))
		require.False(t, r.srv6SIDResolvabilityIgnoredLocked(2, "192.0.2.11", ipv4VPN),
			"group ignore=false must beat global bgp family ignore=true")
		require.False(t, r.srv6SIDResolvabilityIgnoredLocked(2, "192.0.2.11", family.IPv4Unicast))

		// dynamic peer in dyn-ix: resolved via GroupName; group family (evpn=false) beats group (true).
		require.True(t, r.srv6SIDResolvabilityIgnoredLocked(3, "192.0.2.99", family.IPv4Unicast))
		require.False(t, r.srv6SIDResolvabilityIgnoredLocked(3, "192.0.2.99", evpnFamily))

		// standalone peer with no peer/group rule: falls through to bgp family > bgp.
		require.True(t, r.srv6SIDResolvabilityIgnoredLocked(4, "192.0.2.20", ipv4VPN))
		require.True(t, r.srv6SIDResolvabilityIgnoredLocked(4, "192.0.2.20", evpnFamily))
		require.False(t, r.srv6SIDResolvabilityIgnoredLocked(4, "192.0.2.20", family.IPv4Unicast))
	})

	t.Run("validation_rejects_invalid_bools_and_families", func(t *testing.T) {
		r := newRIBManager(nil)
		tx := r.newConfigTx(false)
		require.NoError(t, tx.Configure(ribSections(`"srv6-sid-resolvability":{"ignore":"true"}`)))

		invalidCases := []struct {
			name     string
			body     string
			wantText string
		}{
			{"non_bool_string", `"srv6-sid-resolvability":{"ignore":"yes"}`, "yes"},
			{"numeric_bool_string", `"srv6-sid-resolvability":{"ignore":"1"}`, "1"},
			{"numeric_json", `"srv6-sid-resolvability":{"ignore":1}`, "1"},
			{"unknown_family", `"srv6-sid-resolvability":{"family":{"ipv4/not-a-family":{"ignore":"true"}}}`, "ipv4/not-a-family"},
			{"missing_family_ignore", `"srv6-sid-resolvability":{"family":{"ipv4/unicast":{}}}`, "ignore"},
			{"invalid_family_bool", `"srv6-sid-resolvability":{"family":{"ipv4/unicast":{"ignore":"maybe"}}}`, "maybe"},
			{"unknown_container_key", `"srv6-sid-resolvability":{"bogus":"true"}`, "bogus"},
			{"empty_group_invalid", `"group":{"empty":{"srv6-sid-resolvability":{"ignore":"nope"}}}`, "nope"},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tx.Verify(ribSections(tc.body))
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantText)
				require.ErrorIs(t, tx.Apply(), sdk.ErrConfigApplyUnverified)
				// Running config stays intact after refused verify.
				require.True(t, r.srv6SIDResolvabilityIgnoredWithGroup("192.0.2.1", "", family.IPv4Unicast))
			})
		}
	})

	t.Run("reload_and_rollback_reselect_and_preserve_other_gates", func(t *testing.T) {
		r, loc := nextHopReachabilityRIB(t)
		tx := r.newConfigTx(false)
		require.NoError(t, tx.Configure(ribSections(``)))

		nhReachable := netip.MustParseAddr("198.51.100.1")
		nhUnreachable := netip.MustParseAddr("198.51.100.200")
		sid := netip.MustParseAddr("fc00:b0::1")
		loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/25"), igpRoute(10))

		pfxBypass := netip.MustParsePrefix("10.92.110.0/24")
		wireBypass := ipv4Prefix(24, 10, 92, 110)
		pfxBadNH := netip.MustParsePrefix("10.92.111.0/24")
		wireBadNH := ipv4Prefix(24, 10, 92, 111)
		pfxInvalidSID := netip.MustParsePrefix("10.92.112.0/24")
		wireInvalidSID := ipv4Prefix(24, 10, 92, 112)

		routes := srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nhReachable, 100, sid, wireBypass)
		// Route with unresolvable BGP NEXT_HOP and unresolvable SRv6 SID.
		routes.Insert(family.IPv4Unicast, ipv4SRv6Attrs(nhUnreachable.As4(), 100, sid), wireBadNH)
		// Route with semantically invalid SRv6 Prefix-SID (RFC9252-5-1) and reachable NEXT_HOP.
		invalidAttrs := appendPrefixSIDAttr(aigpSelectionAttrs(nhReachable.As4(), nil, 100, 1), buildInvalidSRv6PrefixSID())
		routes.Insert(family.IPv4Unicast, invalidAttrs, wireInvalidSID)

		r.reselectAIGPRoutes()
		_, found := loc.Best(family.IPv4Unicast, pfxBypass)
		require.False(t, found, "strict default excludes unresolvable SRv6 route")

		// Enable per-peer bypass via ConfigTx Verify + Apply -> reselects immediately.
		bypassCfg := `"peer":{"pe1":{"connection":{"remote":{"ip":"192.0.2.1"}},"srv6-sid-resolvability":{"ignore":"true"}}}`
		require.NoError(t, tx.Verify(ribSections(bypassCfg)))
		require.NoError(t, tx.Apply())

		best, found := loc.Best(family.IPv4Unicast, pfxBypass)
		require.True(t, found, "applying ignore=true must immediately admit retained SRv6 route")
		require.Equal(t, sid, best.SRv6SID)

		// Verify AC-9: bypass NEVER skips NEXT_HOP resolvability or RFC9252-5-1 semantic validity.
		_, foundBadNH := loc.Best(family.IPv4Unicast, pfxBadNH)
		require.False(t, foundBadNH, "srv6-sid-resolvability ignore must never bypass RFC 4271 NEXT_HOP resolvability")
		_, foundInvalid := loc.Best(family.IPv4Unicast, pfxInvalidSID)
		require.False(t, foundInvalid, "srv6-sid-resolvability ignore must never bypass RFC9252-5-1 semantic SID validity")

		// Rollback restores strict enforcement and withdraws immediately.
		require.NoError(t, tx.Rollback())
		_, found = loc.Best(family.IPv4Unicast, pfxBypass)
		require.False(t, found, "rolling back ignore=true must withdraw unresolvable SRv6 route immediately")
		require.Equal(t, 3, routes.Len(), "retained Adj-RIB-In routes remain intact after rollback")
	})
}

// TestSRv6UnresolvableVisibilityShowAndDoctor proves AC-1 and R-2: show bgp rib
// annotates unresolvable SRv6 CIDR and transposed VPNv4 rows while omitting
// exclusion keys for resolvable, bypassed, and non-SRv6 rows; runtime doctor
// check bgp-rib-unresolved-srv6-sid reports bounded samples, enforces the
// 4096-entry cap with overflow deduplication, and clears on resolution,
// withdrawal, and owner-scoped peer DOWN.
//
// VALIDATES: show bgp rib srv6-sid-resolvable / ineligible-reason JSON output,
// diagnostic.Lookup metadata, runtime doctor check lifecycle, and 4096 cap.
// PREVENTS: silent SRv6 exclusion, stale doctor findings, or unbounded tracker
// growth under unreachable locator churn.
func TestSRv6UnresolvableVisibilityShowAndDoctor(t *testing.T) {
	registerBuiltinCommands()

	t.Run("doctor_code_and_declaration", func(t *testing.T) {
		meta := diagnostic.Lookup(codeDoctorBGPSRv6SIDUnresolvable)
		require.NotNil(t, meta, "doctor-bgp-srv6-sid-unresolvable must be registered")
		require.NotEmpty(t, meta.Title)
		require.Contains(t, meta.Description, "RFC 9252")
		require.Contains(t, meta.Description, "Adj-RIB-In")
		require.Contains(t, meta.Description, unresolvedSRv6SIDDoctorFix)
		require.NotEmpty(t, meta.Examples)

		decl := unresolvedSRv6SIDDoctorDeclaration()
		require.Equal(t, unresolvedSRv6SIDDoctorName, decl.Name)
		require.Equal(t, rpc.DoctorPhaseRuntime, decl.Phase)
		require.Contains(t, decl.Codes, codeDoctorBGPSRv6SIDUnresolvable)
	})

	t.Run("show_bgp_rib_annotations_and_omissions", func(t *testing.T) {
		oldMetrics := metricsPtr.Load()
		defer metricsPtr.Store(oldMetrics)
		reg := metrics.NewPrometheusRegistry()
		SetMetricsRegistry(reg)

		family.RegisterTestFamilies()
		r, loc := nextHopReachabilityRIB(t)
		ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

		nhV4 := netip.MustParseAddr("198.51.100.1")
		loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))
		loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:c1::/48"), ipv6IGPRoute(10))

		unresSID := netip.MustParseAddr("fc00:c0::1")
		resSID := netip.MustParseAddr("fc00:c1::1")

		wireUnres := ipv4Prefix(24, 10, 92, 120)
		wireRes := ipv4Prefix(24, 10, 92, 121)
		wirePlain := ipv4Prefix(24, 10, 92, 122)
		wireBypass := ipv4Prefix(24, 10, 92, 123)

		// Peer 1: unresolvable SRv6, resolvable SRv6, plain non-SRv6, and transposed unresolvable VPNv4.
		routes1 := srv6IPv4Path(t, r, 1, netip.MustParseAddr("192.0.2.1"), nhV4, 100, unresSID, wireUnres)
		routes1.Insert(family.IPv4Unicast, ipv4SRv6Attrs(nhV4.As4(), 100, resSID), wireRes)
		routes1.Insert(family.IPv4Unicast, aigpSelectionAttrs(nhV4.As4(), nil, 100, 1), wirePlain)

		// Peer 2: bypassed via srv6-sid-resolvability { ignore true; }.
		srv6IPv4Path(t, r, 2, netip.MustParseAddr("192.0.2.2"), nhV4, 100, unresSID, wireBypass)
		tx := r.newConfigTx(false)
		require.NoError(t, tx.Configure(ribSections(`"peer":{"pe2":{"connection":{"remote":{"ip":"192.0.2.2"}},"srv6-sid-resolvability":{"ignore":"true"}}}`)))

		// Transposed VPNv4 route whose merged SID fc00:c2:0042::1 is unresolvable
		// even though its partial wire SID fc00:c2:0000::1 has a covering route.
		loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("fc00:c2:0000::/48"), ipv6IGPRoute(10))
		vpnBody, vpnNLRI := vpnv4TransposedSRv6Update(nhV4.As4(), netip.MustParseAddr("fc00:c2:0000::1"), 0x0042<<4, 32, 16, 16, 0, 16, 32)
		feedReceived(r, 1, netip.MustParseAddr("192.0.2.1"), ctxID, vpnBody)
		vpnDisplay := formatNLRIAsPrefix(ipv4VPN, vpnNLRI, false)

		r.checkBestPathChange(family.IPv4Unicast, wireUnres, false, nil)
		r.checkBestPathChange(family.IPv4Unicast, wireRes, false, nil)
		r.checkBestPathChange(family.IPv4Unicast, wirePlain, false, nil)
		r.checkBestPathChange(family.IPv4Unicast, wireBypass, false, nil)

		rows := showBGPRIBRows(t, r, "received")

		unresRow := findRowByPrefix(t, rows, "10.92.120.0/24")
		require.Equal(t, false, unresRow[rowKeySRv6SIDResolvable])
		require.Equal(t, reasonSRv6SIDUnresolvable, unresRow[rowKeyIneligibleReason])

		vpnRow := findRowByPrefix(t, rows, vpnDisplay)
		require.Equal(t, false, vpnRow[rowKeySRv6SIDResolvable])
		require.Equal(t, reasonSRv6SIDUnresolvable, vpnRow[rowKeyIneligibleReason])

		for _, cleanPfx := range []string{"10.92.121.0/24", "10.92.122.0/24", "10.92.123.0/24"} {
			row := findRowByPrefix(t, rows, cleanPfx)
			_, hasRes := row[rowKeySRv6SIDResolvable]
			require.False(t, hasRes, "clean/resolvable/bypassed prefix %s must omit srv6-sid-resolvable", cleanPfx)
			_, hasReason := row[rowKeyIneligibleReason]
			require.False(t, hasReason, "clean/resolvable/bypassed prefix %s must omit ineligible-reason", cleanPfx)
		}

		matched := showBGPRIBRows(t, r, "received", "match", reasonSRv6SIDUnresolvable)
		require.Len(t, matched, 2)

		// Read-only `show bgp rib best` must exclude unresolvable SRv6 routes
		// without incrementing ze_rib_srv6_sid_unresolvable_exclusions_total.
		// ipv4/unicast counted 2 exclusions (tx.Configure reselection + explicit checkBestPathChange);
		// ipv4/mpls-vpn counted 1 exclusion (feedReceived selection).
		metricsBeforeShowBest := scrapeRIBMetrics(t, reg)
		require.Contains(t, metricsBeforeShowBest, `ze_rib_srv6_sid_unresolvable_exclusions_total{family="ipv4/unicast"} 2`)
		require.Contains(t, metricsBeforeShowBest, `ze_rib_srv6_sid_unresolvable_exclusions_total{family="ipv4/mpls-vpn"} 1`)

		status, bestPayload, err := r.handleCommand(cmdShowRIBBest, "*", nil)
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		bestJSON, err := json.Marshal(bestPayload)
		require.NoError(t, err)
		bestText := string(bestJSON)
		require.NotContains(t, bestText, "10.92.120.0/24")
		require.NotContains(t, bestText, vpnDisplay)
		require.Contains(t, bestText, "10.92.121.0/24")
		require.Contains(t, bestText, "10.92.122.0/24")
		require.Contains(t, bestText, "10.92.123.0/24")

		metricsAfterShowBest := scrapeRIBMetrics(t, reg)
		require.Equal(t, metricsBeforeShowBest, metricsAfterShowBest,
			"read-only show bgp rib best must not increment ze_rib_srv6_sid_unresolvable_exclusions_total")
	})

	t.Run("doctor_lifecycle_owner_isolation_and_4096_cap", func(t *testing.T) {
		r, loc := nextHopReachabilityRIB(t)
		nhV4 := netip.MustParseAddr("198.51.100.1")
		loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(10))

		sharedPeer := netip.MustParseAddr("192.0.2.188")
		const (
			ownerA uint64 = 2701
			ownerB uint64 = 2702
		)
		sidA := netip.MustParseAddr("fc00:d1::1")
		sidB := netip.MustParseAddr("fc00:d2::1")
		wireA := ipv4Prefix(24, 10, 92, 131)
		wireB := ipv4Prefix(24, 10, 92, 132)

		srv6IPv4Path(t, r, ownerA, sharedPeer, nhV4, 100, sidA, wireA)
		srv6IPv4Path(t, r, ownerB, sharedPeer, nhV4, 100, sidB, wireB)
		r.checkBestPathChange(family.IPv4Unicast, wireA, false, nil)
		r.checkBestPathChange(family.IPv4Unicast, wireB, false, nil)

		findings, err := r.doctorCheck(unresolvedSRv6SIDDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Equal(t, codeDoctorBGPSRv6SIDUnresolvable, findings[0].Code)
		require.Equal(t, doctorSeverityWarning, findings[0].Severity)
		require.Equal(t, unresolvedSRv6SIDDoctorFix, findings[0].Fix)
		require.Contains(t, findings[0].Message, "10.92.131.0/24")
		require.Contains(t, findings[0].Message, sidA.String())
		require.Contains(t, findings[0].Message, "10.92.132.0/24")

		// Purging ownerA preserves same-address sibling ownerB.
		r.peerMu.Lock()
		delete(r.bgpPeers, ownerA)
		r.peerMu.Unlock()
		r.purgeBestPrevForOwner(bestPeer{owner: ownerA, address: sharedPeer.String()})

		findings, err = r.doctorCheck(unresolvedSRv6SIDDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.NotContains(t, findings[0].Message, "10.92.131.0/24")
		require.Contains(t, findings[0].Message, "10.92.132.0/24")

		// Purging ownerB clears findings completely.
		r.peerMu.Lock()
		delete(r.bgpPeers, ownerB)
		r.peerMu.Unlock()
		r.purgeBestPrevForOwner(bestPeer{owner: ownerB, address: sharedPeer.String()})

		findings, err = r.doctorCheck(unresolvedSRv6SIDDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings)

		// Exercise the 4096-entry tracker cap + overflow deduplication.
		peerCap := netip.MustParseAddr("192.0.2.199")
		sidCap := netip.MustParseAddr("fc00:d9::1")
		attrs := ipv4SRv6Attrs(nhV4.As4(), 100, sidCap)
		firstWire := ipv4Prefix(24, 10, 128, 0)
		routesCap := srv6IPv4Path(t, r, 2709, peerCap, nhV4, 100, sidCap, firstWire)
		r.checkBestPathChange(family.IPv4Unicast, firstWire, false, nil)

		const (
			trackerCap = 4096
			overflow   = 5
		)
		var overflowWires [][]byte
		for i := 1; i < trackerCap+overflow; i++ {
			wire := ipv4Prefix(24, 10, 128+byte(i>>8), byte(i&0xff))
			routesCap.Insert(family.IPv4Unicast, attrs, wire)
			r.checkBestPathChange(family.IPv4Unicast, wire, false, nil)
			if i >= trackerCap {
				overflowWires = append(overflowWires, wire)
			}
		}

		findings, err = r.doctorCheck(unresolvedSRv6SIDDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Contains(t, findings[0].Message, "4101")
		require.Contains(t, findings[0].Message, "4096")
		require.Contains(t, findings[0].Message, "(+4096 more)")

		// Re-announcing overflowed prefixes must not inflate the overflow count.
		updatedAttrs := ipv4SRv6Attrs(nhV4.As4(), 200, sidCap)
		for range 5 {
			for _, w := range overflowWires {
				routesCap.Insert(family.IPv4Unicast, updatedAttrs, w)
				r.checkBestPathChange(family.IPv4Unicast, w, false, nil)
			}
		}
		findings, err = r.doctorCheck(unresolvedSRv6SIDDoctorName)
		require.NoError(t, err)
		require.Len(t, findings, 1)
		require.Contains(t, findings[0].Message, "4101")

		// Purging owner clears tracked and overflow state cleanly.
		r.peerMu.Lock()
		delete(r.bgpPeers, 2709)
		r.peerMu.Unlock()
		r.purgeBestPrevForOwner(bestPeer{owner: 2709, address: peerCap.String()})

		findings, err = r.doctorCheck(unresolvedSRv6SIDDoctorName)
		require.NoError(t, err)
		require.Empty(t, findings)
		require.Zero(t, r.unresolvedSRv6SIDs.total.Load())
		require.Zero(t, r.unresolvedSRv6SIDs.overflow.Load())
	})
}
