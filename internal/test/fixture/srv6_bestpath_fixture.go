// Design: docs/architecture/route-selection.md -- SRv6 Service SID resolvability before best-path selection.
// Related: register_srv6_bestpath.go -- fixture registrations.
// Related: rfc4271_next_hop_daemon_fixture.go -- real BGP socket and kernel FIB verification helpers.
// RFC: rfc/short/rfc9252.md -- Sections 5 and 6.
package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	srv6BestPathSRSourcePeer  = "127.0.0.2"
	srv6BestPathFallbackPeer  = "127.0.0.3"
	srv6BestPathReceiverPeer  = "127.0.0.4"
	srv6BestPathSRNextHop     = "10.251.1.1"
	srv6BestPathFallbackNH    = "10.251.2.1"
	srv6BestPathV6NextHop     = "2001:db8:fe::1"
	srv6BestPathLocatorPrefix = "fc00:0:99::/48"
	srv6BestPathServiceSID    = "fc00:0:99::1"
	srv6BestPathSubjectPfx    = "198.19.160.0/24"
	srv6BestPathResolvPfxA    = "198.19.161.0/24"
	srv6BestPathResolvPfxB    = "198.19.162.0/24"
	srv6BestPathStaticPfx     = "198.19.163.0/24"
	srv6BestPathStaticCtrlPfx = "198.19.164.0/24"
	srv6BestPathBypassV4Pfx   = "198.19.165.0/24"
	srv6BestPathBypassV6Pfx   = "2001:db8:165::/48"
	srv6BestPathStaticLink    = "zesrv6st"
	srv6BestPathStaticConn    = "198.18.231.0/24"
	srv6BestPathStaticGW      = "198.18.231.1"
	srv6BestPathStaticBGPNH   = "198.18.231.2"

	srv6BestPathDoctorCode    = "doctor-bgp-srv6-sid-unresolvable"
	srv6BestPathDoctorFix     = "ze show bgp rib | match srv6-sid-unresolvable"
	srv6BestPathIneligibleKey = "srv6-sid-unresolvable"
	srv6BestPathMatchCmd      = "show bgp rib | match srv6-sid-unresolvable"
	srv6BestPathReceivedCmd   = "show bgp rib | received"
	srv6BestPathDoctorCmd     = "show doctor"
	srv6BestPathExclMetric    = `ze_rib_srv6_sid_unresolvable_exclusions_total{family="ipv4/unicast"}`

	srv6BestPathPollAttempts = 80
	srv6BestPathPollDelay    = 100 * time.Millisecond

	srv6BestPathLocalASN    uint16 = 65001
	srv6BestPathReceiverASN uint16 = 65002
	srv6BestPathSRASN       uint16 = 65101
	srv6BestPathFallbackASN uint16 = 65102
)

// srv6RIBRow models the operator-facing fields inspected on show bgp rib rows.
type srv6RIBRow struct {
	Peer              string `json:"peer"`
	Family            string `json:"family"`
	Prefix            string `json:"prefix"`
	Best              *bool  `json:"best"`
	SRv6SIDResolvable *bool  `json:"srv6-sid-resolvable"`
	IneligibleReason  string `json:"ineligible-reason"`
}

// srv6RIBAnswer models the JSON envelope returned by show bgp rib commands.
type srv6RIBAnswer struct {
	Routes []srv6RIBRow `json:"routes"`
}

// srv6DoctorFinding models one diagnostic object returned by show doctor.
type srv6DoctorFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Fix      string `json:"fix"`
}

// srv6DoctorAnswer models the JSON envelope returned by show doctor.
type srv6DoctorAnswer struct {
	Diagnostics []srv6DoctorFinding `json:"diagnostics"`
}

// srv6WireExpectation defines expected IPv4 /24 wire announcements and
// withdrawals on a receiver BGP socket without assuming packet ordering.
type srv6WireExpectation struct {
	announced map[string][]byte // prefix -> expected AS_PATH wire bytes
	withdrawn map[string]bool   // prefix -> expected withdrawal
}

// srv6BestPathUnresolvableDriver proves AC-1 and AC-2 end-to-end: an
// unresolvable SRv6 Service SID path at higher LOCAL_PREF is excluded before
// best-path selection, a reachable lower-LOCAL_PREF runner-up wins and is
// advertised to a downstream peer, show bgp rib marks the retained SRv6 row
// ineligible, show doctor surfaces doctor-bgp-srv6-sid-unresolvable, and the
// Prometheus exclusion counter increments.
func srv6BestPathUnresolvableDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("srv6-bestpath-unresolvable requires <BGP listen address>")
	}
	listenAddr := args[0]
	return Observe(ctx, "srv6-bestpath-unresolvable", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		srConn, fallbackConn, recvConn, err := srv6ConnectThreePeers(ctx, plugin, listenAddr)
		if err != nil {
			return err
		}
		defer srConn.Close()       //nolint:errcheck // Socket closes at fixture exit.
		defer fallbackConn.Close() //nolint:errcheck // Socket closes at fixture exit.
		defer recvConn.Close()     //nolint:errcheck // Socket closes at fixture exit.

		nhCovers := terminalCovers{
			label:     "srv6-unresolvable-nh",
			device:    "srv6nh0",
			instance:  9252,
			addresses: []string{srv6BestPathSRNextHop, srv6BestPathFallbackNH},
		}
		if err := nhCovers.install(ctx, plugin); err != nil {
			return err
		}

		sid := netip.MustParseAddr(srv6BestPathServiceSID)
		if err := srv6SendIPv4Update(srConn, srv6BestPathSubjectPfx, srv6BestPathSRNextHop, srv6BestPathSRASN, 200, &sid); err != nil {
			return err
		}
		if err := srv6SendIPv4Update(fallbackConn, srv6BestPathSubjectPfx, srv6BestPathFallbackNH, srv6BestPathFallbackASN, 100, nil); err != nil {
			return err
		}

		if err := srv6WaitReceivedCount(ctx, plugin, srv6BestPathSubjectPfx, 2); err != nil {
			return err
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathSubjectPfx, srv6BestPathFallbackPeer); err != nil {
			return fmt.Errorf("runner-up did not win over unresolvable SRv6 path: %w", err)
		}

		wantPath := srv6TwoOctetASPath(srv6BestPathLocalASN, srv6BestPathFallbackASN)
		if err := srv6WaitWireEvents(recvConn, srv6WireExpectation{
			announced: map[string][]byte{srv6BestPathSubjectPfx: wantPath},
		}); err != nil {
			return fmt.Errorf("receiver did not get reachable runner-up advertisement: %w", err)
		}

		if err := srv6WaitRowMarkedIneligible(ctx, plugin, srv6BestPathSRSourcePeer, srv6BestPathSubjectPfx, true); err != nil {
			return err
		}
		if err := srv6WaitRowMarkedIneligible(ctx, plugin, srv6BestPathFallbackPeer, srv6BestPathSubjectPfx, false); err != nil {
			return err
		}
		if err := srv6VerifyMatchPipe(ctx, plugin, srv6BestPathSubjectPfx, true); err != nil {
			return err
		}
		if err := srv6WaitDoctorFinding(ctx, plugin, true, srv6BestPathSubjectPfx, srv6BestPathServiceSID); err != nil {
			return err
		}
		if err := srv6WaitExclusionMetric(ctx, plugin); err != nil {
			return err
		}
		return importSay("srv6-bestpath-unresolvable: unresolvable SRv6 path excluded, runner-up advertised, show bgp rib and show doctor reported exclusion")
	})
}

// srv6BestPathResolvableDriver proves AC-3 and AC-4 end-to-end: adding a
// covering IPv6 locator route promotes retained excluded SRv6 paths (both with
// and without a runner-up) without any new BGP UPDATE, and removing that cover
// fails back / withdraws automatically while retaining the paths in Adj-RIB-In.
func srv6BestPathResolvableDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("srv6-bestpath-resolvable requires <BGP listen address>")
	}
	listenAddr := args[0]
	return Observe(ctx, "srv6-bestpath-resolvable", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		srConn, fallbackConn, recvConn, err := srv6ConnectThreePeers(ctx, plugin, listenAddr)
		if err != nil {
			return err
		}
		defer srConn.Close()       //nolint:errcheck // Socket closes at fixture exit.
		defer fallbackConn.Close() //nolint:errcheck // Socket closes at fixture exit.
		defer recvConn.Close()     //nolint:errcheck // Socket closes at fixture exit.

		nhCovers := terminalCovers{
			label:     "srv6-resolvable-nh",
			device:    "srv6nh0",
			instance:  9252,
			addresses: []string{srv6BestPathSRNextHop, srv6BestPathFallbackNH},
		}
		if err := nhCovers.install(ctx, plugin); err != nil {
			return err
		}

		sid := netip.MustParseAddr(srv6BestPathServiceSID)
		if err := srv6SendIPv4Update(srConn, srv6BestPathResolvPfxA, srv6BestPathSRNextHop, srv6BestPathSRASN, 200, &sid); err != nil {
			return err
		}
		if err := srv6SendIPv4Update(srConn, srv6BestPathResolvPfxB, srv6BestPathSRNextHop, srv6BestPathSRASN, 200, &sid); err != nil {
			return err
		}
		if err := srv6SendIPv4Update(fallbackConn, srv6BestPathResolvPfxA, srv6BestPathFallbackNH, srv6BestPathFallbackASN, 100, nil); err != nil {
			return err
		}

		if err := srv6WaitReceivedCount(ctx, plugin, srv6BestPathResolvPfxA, 2); err != nil {
			return err
		}
		if err := srv6WaitReceivedCount(ctx, plugin, srv6BestPathResolvPfxB, 1); err != nil {
			return err
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathResolvPfxA, srv6BestPathFallbackPeer); err != nil {
			return err
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathResolvPfxB, ""); err != nil {
			return err
		}

		fallbackPath := srv6TwoOctetASPath(srv6BestPathLocalASN, srv6BestPathFallbackASN)
		srPath := srv6TwoOctetASPath(srv6BestPathLocalASN, srv6BestPathSRASN)
		if err := srv6WaitWireEvents(recvConn, srv6WireExpectation{
			announced: map[string][]byte{srv6BestPathResolvPfxA: fallbackPath},
		}); err != nil {
			return fmt.Errorf("initial fallback wire advertisement: %w", err)
		}

		beforeUpdates, err := peerCounter03(ctx, plugin, srv6BestPathSRSourcePeer, "updates-received")
		if err != nil {
			return err
		}

		locatorEntry := rpc.RouteInstallEntry{
			Protocol:  protocolStatic,
			Instance:  9252,
			AFI:       2,
			SAFI:      1,
			Prefix:    srv6BestPathLocatorPrefix,
			Interface: "srv6loc0",
			RouteType: 1,
		}
		if err := sysribInstall(ctx, plugin, locatorEntry); err != nil {
			return fmt.Errorf("install IPv6 locator cover: %w", err)
		}

		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathResolvPfxA, srv6BestPathSRSourcePeer); err != nil {
			return fmt.Errorf("locator add did not promote SRv6 path A: %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathResolvPfxB, srv6BestPathSRSourcePeer); err != nil {
			return fmt.Errorf("locator add did not promote solo excluded SRv6 path B: %w", err)
		}
		if err := srv6WaitWireEvents(recvConn, srv6WireExpectation{
			announced: map[string][]byte{
				srv6BestPathResolvPfxA: srPath,
				srv6BestPathResolvPfxB: srPath,
			},
		}); err != nil {
			return fmt.Errorf("locator add did not advertise promoted SRv6 paths: %w", err)
		}
		if err := srv6WaitRowMarkedIneligible(ctx, plugin, srv6BestPathSRSourcePeer, srv6BestPathResolvPfxA, false); err != nil {
			return err
		}
		if err := srv6WaitRowMarkedIneligible(ctx, plugin, srv6BestPathSRSourcePeer, srv6BestPathResolvPfxB, false); err != nil {
			return err
		}
		if err := srv6WaitDoctorFinding(ctx, plugin, false, "", ""); err != nil {
			return err
		}

		afterPromote, err := peerCounter03(ctx, plugin, srv6BestPathSRSourcePeer, "updates-received")
		if err != nil {
			return err
		}
		if afterPromote != beforeUpdates {
			return fmt.Errorf("expected zero new BGP UPDATEs during locator promotion: before=%d after=%d", beforeUpdates, afterPromote)
		}

		if err := sysribRemove(ctx, plugin, locatorEntry); err != nil {
			return fmt.Errorf("remove IPv6 locator cover: %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathResolvPfxA, srv6BestPathFallbackPeer); err != nil {
			return fmt.Errorf("locator removal did not fail back prefix A: %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathResolvPfxB, ""); err != nil {
			return fmt.Errorf("locator removal did not withdraw solo prefix B: %w", err)
		}
		if err := srv6WaitWireEvents(recvConn, srv6WireExpectation{
			announced: map[string][]byte{srv6BestPathResolvPfxA: fallbackPath},
			withdrawn: map[string]bool{srv6BestPathResolvPfxB: true},
		}); err != nil {
			return fmt.Errorf("locator removal wire failover/withdrawal: %w", err)
		}
		if err := srv6WaitRowMarkedIneligible(ctx, plugin, srv6BestPathSRSourcePeer, srv6BestPathResolvPfxA, true); err != nil {
			return err
		}
		if err := srv6WaitRowMarkedIneligible(ctx, plugin, srv6BestPathSRSourcePeer, srv6BestPathResolvPfxB, true); err != nil {
			return err
		}
		if err := srv6WaitDoctorFinding(ctx, plugin, true, srv6BestPathResolvPfxA, srv6BestPathServiceSID); err != nil {
			return err
		}

		afterDemote, err := peerCounter03(ctx, plugin, srv6BestPathSRSourcePeer, "updates-received")
		if err != nil {
			return err
		}
		if afterDemote != beforeUpdates {
			return fmt.Errorf("expected zero new BGP UPDATEs during locator removal: before=%d after=%d", beforeUpdates, afterDemote)
		}
		return importSay("srv6-bestpath-resolvable: locator add and remove promoted and demoted retained SRv6 paths with zero new BGP UPDATEs")
	})
}

// srv6BestPathStaticWinsDriver proves AC-7 against the real Linux kernel FIB:
// when a competing iBGP path (AD 200) carries an unreachable SRv6 Service SID,
// excluding it before best-path selection allows the static backup (worse AD
// 210) to win Loc-RIB arbitration and remain programmed in the Linux kernel FIB.
func srv6BestPathStaticWinsDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("srv6-bestpath-static-wins requires <BGP listen address>")
	}
	listenAddr := args[0]
	return Observe(ctx, "srv6-bestpath-static-wins", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := waitRIBWinner(ctx, plugin, srv6BestPathStaticConn, "connected"); err != nil {
			return fmt.Errorf("connected subnet on %s never entered Loc-RIB: %w", srv6BestPathStaticLink, err)
		}
		if err := waitRIBWinner(ctx, plugin, srv6BestPathStaticPfx, protocolStatic); err != nil {
			return fmt.Errorf("initial static backup never entered Loc-RIB: %w", err)
		}
		if err := importKernel(ctx, srv6BestPathStaticPfx, 1, "via "+srv6BestPathStaticGW+" ", "dev "+srv6BestPathStaticLink+" ", "proto 250 "); err != nil {
			return fmt.Errorf("initial static backup not installed in kernel FIB: %w", err)
		}

		srConn, err := atomicAggregatePolicyConnect(ctx, listenAddr, netip.MustParseAddr(srv6BestPathSRSourcePeer), srv6BestPathLocalASN)
		if err != nil {
			return err
		}
		defer srConn.Close() //nolint:errcheck // Socket closes at fixture exit.
		if err := p12WaitPeersEOR(ctx, plugin, "sr-source"); err != nil {
			return err
		}

		sid := netip.MustParseAddr(srv6BestPathServiceSID)
		if err := srv6SendIPv4Update(srConn, srv6BestPathStaticPfx, srv6BestPathStaticBGPNH, srv6BestPathSRASN, 200, &sid); err != nil {
			return err
		}
		if err := srv6SendIPv4Update(srConn, srv6BestPathStaticCtrlPfx, srv6BestPathStaticBGPNH, srv6BestPathSRASN, 100, nil); err != nil {
			return err
		}

		if err := srv6WaitReceivedCount(ctx, plugin, srv6BestPathStaticPfx, 1); err != nil {
			return err
		}
		if err := srv6WaitReceivedCount(ctx, plugin, srv6BestPathStaticCtrlPfx, 1); err != nil {
			return err
		}

		// Positive control: reachable non-SRv6 BGP route installs into kernel FIB.
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathStaticCtrlPfx, srv6BestPathSRSourcePeer); err != nil {
			return fmt.Errorf("control BGP route not elected best: %w", err)
		}
		if err := waitRIBWinner(ctx, plugin, srv6BestPathStaticCtrlPfx, protocolBGP); err != nil {
			return fmt.Errorf("control BGP route not selected in Loc-RIB: %w", err)
		}
		if err := importKernel(ctx, srv6BestPathStaticCtrlPfx, 1, "via "+srv6BestPathStaticBGPNH+" ", "dev "+srv6BestPathStaticLink+" ", "proto 250 "); err != nil {
			return fmt.Errorf("control BGP route not installed in kernel FIB: %w", err)
		}

		// Subject: unresolvable SRv6 BGP path stays excluded from BGP best-path
		// and Loc-RIB so static backup at worse AD 210 wins and stays in kernel FIB.
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathStaticPfx, ""); err != nil {
			return fmt.Errorf("unresolvable SRv6 BGP path unexpectedly won best-path: %w", err)
		}
		if err := srv6WaitRowMarkedIneligible(ctx, plugin, srv6BestPathSRSourcePeer, srv6BestPathStaticPfx, true); err != nil {
			return err
		}
		if _, err := waitCandidates(ctx, plugin, srv6BestPathStaticPfx, func(row candidateRow) bool {
			if _, hasBGP := candidatePathOf(row, protocolBGP); hasBGP {
				return false
			}
			staticPath, hasStatic := candidatePathOf(row, protocolStatic)
			if !hasStatic {
				return false
			}
			return staticPath.Selected
		}); err != nil {
			return fmt.Errorf("static backup did not remain sole Loc-RIB winner: %w", err)
		}
		if err := importKernel(ctx, srv6BestPathStaticPfx, 1, "via "+srv6BestPathStaticGW+" ", "dev "+srv6BestPathStaticLink+" ", "proto 250 "); err != nil {
			return fmt.Errorf("static backup route missing from Linux kernel FIB: %w", err)
		}
		return importSay("srv6-bestpath-static-wins: static backup at worse AD won Loc-RIB and remained programmed in Linux kernel FIB")
	})
}

// srv6BestPathStaticWinsEmptyDriver verifies that the kernel FIB holds none of
// the static-wins fixture prefixes before start and after daemon exit.
func srv6BestPathStaticWinsEmptyDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("srv6-bestpath-static-wins-empty takes no arguments")
	}
	for _, prefix := range []string{srv6BestPathStaticPfx, srv6BestPathStaticCtrlPfx} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("srv6-bestpath-static-wins prefixes are absent from every kernel table")
}

// srv6BestPathBypassConfigDriver proves AC-11 end-to-end: per-peer and
// per-family srv6-sid-resolvability { ignore true|false; } bypasses BGP
// pre-selection exclusion, live reload and rollback re-evaluate retained
// routes immediately with zero new BGP UPDATEs, and an invalid reload is
// refused while keeping the active selection intact.
func srv6BestPathBypassConfigDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("srv6-bestpath-bypass-config requires <BGP listen address> <active config file>")
	}
	listenAddr := args[0]
	activeConfig := args[1]
	return Observe(ctx, "srv6-bestpath-bypass-config", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		conn, err := srv6ConnectDualStack(ctx, listenAddr, netip.MustParseAddr(srv6BestPathSRSourcePeer), srv6BestPathLocalASN)
		if err != nil {
			return err
		}
		defer conn.Close() //nolint:errcheck // Socket closes at fixture exit.
		if err := p12WaitPeersEOR(ctx, plugin, "sr-peer"); err != nil {
			return err
		}

		nhCovers := terminalCovers{
			label:     "srv6-bypass-nh",
			device:    "srv6nh0",
			instance:  9252,
			addresses: []string{srv6BestPathSRNextHop, srv6BestPathV6NextHop},
		}
		if err := nhCovers.install(ctx, plugin); err != nil {
			return err
		}

		sid := netip.MustParseAddr(srv6BestPathServiceSID)
		if err := srv6SendIPv4Update(conn, srv6BestPathBypassV4Pfx, srv6BestPathSRNextHop, srv6BestPathSRASN, 200, &sid); err != nil {
			return err
		}
		if err := srv6SendIPv6Update(conn, srv6BestPathBypassV6Pfx, srv6BestPathV6NextHop, srv6BestPathSRASN, 200, sid); err != nil {
			return err
		}

		if err := srv6WaitReceivedCount(ctx, plugin, srv6BestPathBypassV4Pfx, 1); err != nil {
			return err
		}
		if err := srv6WaitReceivedCount(ctx, plugin, srv6BestPathBypassV6Pfx, 1); err != nil {
			return err
		}

		// Strict default: both IPv4 and IPv6 SRv6 paths are excluded.
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV4Pfx, ""); err != nil {
			return fmt.Errorf("strict default did not exclude IPv4 SRv6 route: %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV6Pfx, ""); err != nil {
			return fmt.Errorf("strict default did not exclude IPv6 SRv6 route: %w", err)
		}

		beforeUpdates, err := peerCounter03(ctx, plugin, srv6BestPathSRSourcePeer, "updates-received")
		if err != nil {
			return err
		}

		baseBytes, err := os.ReadFile(activeConfig) //nolint:gosec // Active config file named by this .ci case.
		if err != nil {
			return err
		}
		baseText := string(baseBytes)
		schema, err := config.YANGSchema()
		if err != nil {
			return err
		}

		// Reload 1: per-family bypass for ipv4/unicast on sr-peer.
		v4OnlyText, err := srv6WithPeerBypass(schema, baseText, `srv6-sid-resolvability { family ipv4/unicast { ignore true; } }`)
		if err != nil {
			return err
		}
		if err := bgpBestRouteReload(ctx, plugin, activeConfig, v4OnlyText); err != nil {
			return fmt.Errorf("reload 1 (ipv4/unicast bypass): %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV4Pfx, srv6BestPathSRSourcePeer); err != nil {
			return fmt.Errorf("reload 1 did not admit ipv4/unicast SRv6 path: %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV6Pfx, ""); err != nil {
			return fmt.Errorf("reload 1 unexpectedly admitted ipv6/unicast SRv6 path: %w", err)
		}

		// Reload 2: peer ignore true with family ipv4/unicast ignore false.
		peerIgnoreText, err := srv6WithPeerBypass(schema, baseText, `srv6-sid-resolvability { ignore true; family ipv4/unicast { ignore false; } }`)
		if err != nil {
			return err
		}
		if err := bgpBestRouteReload(ctx, plugin, activeConfig, peerIgnoreText); err != nil {
			return fmt.Errorf("reload 2 (peer ignore + ipv4 strict override): %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV6Pfx, srv6BestPathSRSourcePeer); err != nil {
			return fmt.Errorf("reload 2 did not admit ipv6/unicast SRv6 path: %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV4Pfx, ""); err != nil {
			return fmt.Errorf("reload 2 did not re-exclude ipv4/unicast SRv6 path: %w", err)
		}

		// Reload 3: invalid family name must be refused and keep reload 2 state.
		invalidText := strings.Replace(peerIgnoreText, "ipv4/unicast", "ipv4/bogus", 1)
		if invalidText == peerIgnoreText {
			return errors.New("failed to construct invalid reload config")
		}
		if err := os.WriteFile(activeConfig, []byte(invalidText), 0o600); err != nil {
			return err
		}
		status, _, reloadErr := command02(ctx, plugin, "request reload")
		if reloadErr == nil && status != statusError {
			return fmt.Errorf("invalid srv6-sid-resolvability reload unexpectedly succeeded (status=%s)", status)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV6Pfx, srv6BestPathSRSourcePeer); err != nil {
			return fmt.Errorf("failed reload disturbed active ipv6/unicast best path: %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV4Pfx, ""); err != nil {
			return fmt.Errorf("failed reload disturbed active ipv4/unicast exclusion: %w", err)
		}

		// Reload 4: restore original strict config (rollback).
		if err := bgpBestRouteReload(ctx, plugin, activeConfig, baseText); err != nil {
			return fmt.Errorf("reload 4 (restore strict default): %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV4Pfx, ""); err != nil {
			return fmt.Errorf("rollback did not re-exclude ipv4/unicast SRv6 path: %w", err)
		}
		if err := rfc4271NextHopBest(ctx, plugin, srv6BestPathBypassV6Pfx, ""); err != nil {
			return fmt.Errorf("rollback did not re-exclude ipv6/unicast SRv6 path: %w", err)
		}

		afterUpdates, err := peerCounter03(ctx, plugin, srv6BestPathSRSourcePeer, "updates-received")
		if err != nil {
			return err
		}
		if afterUpdates != beforeUpdates {
			return fmt.Errorf("expected zero new BGP UPDATEs across bypass reloads: before=%d after=%d", beforeUpdates, afterUpdates)
		}
		return importSay("srv6-bestpath-bypass-config: per-peer and per-family bypass, invalid refusal, and rollback reselection verified")
	})
}

func srv6ConnectThreePeers(ctx context.Context, plugin *sdk.Plugin, listenAddr string) (srConn, fallbackConn, recvConn net.Conn, err error) {
	srConn, err = atomicAggregatePolicyConnect(ctx, listenAddr, netip.MustParseAddr(srv6BestPathSRSourcePeer), srv6BestPathLocalASN)
	if err != nil {
		return nil, nil, nil, err
	}
	fallbackConn, err = atomicAggregatePolicyConnect(ctx, listenAddr, netip.MustParseAddr(srv6BestPathFallbackPeer), srv6BestPathLocalASN)
	if err != nil {
		_ = srConn.Close() //nolint:errcheck // Cleanup on partial connect failure.
		return nil, nil, nil, err
	}
	recvConn, err = atomicAggregatePolicyConnect(ctx, listenAddr, netip.MustParseAddr(srv6BestPathReceiverPeer), srv6BestPathReceiverASN)
	if err != nil {
		_ = srConn.Close()       //nolint:errcheck // Cleanup on partial connect failure.
		_ = fallbackConn.Close() //nolint:errcheck // Cleanup on partial connect failure.
		return nil, nil, nil, err
	}
	if err = p12WaitPeersEOR(ctx, plugin, "sr-source", "fallback", "receiver"); err != nil {
		_ = srConn.Close()       //nolint:errcheck // Cleanup on handshake failure.
		_ = fallbackConn.Close() //nolint:errcheck // Cleanup on handshake failure.
		_ = recvConn.Close()     //nolint:errcheck // Cleanup on handshake failure.
		return nil, nil, nil, err
	}
	if err = bgpBestRouteExpect(recvConn, nil, nil, nil); err != nil {
		_ = srConn.Close()       //nolint:errcheck // Cleanup on EOR failure.
		_ = fallbackConn.Close() //nolint:errcheck // Cleanup on EOR failure.
		_ = recvConn.Close()     //nolint:errcheck // Cleanup on EOR failure.
		return nil, nil, nil, fmt.Errorf("receiver initial End-of-RIB: %w", err)
	}
	return srConn, fallbackConn, recvConn, nil
}

func srv6ConnectDualStack(ctx context.Context, address string, source netip.Addr, asn uint16) (net.Conn, error) {
	dialer := net.Dialer{Timeout: time.Second, LocalAddr: net.TCPAddrFromAddrPort(netip.AddrPortFrom(source, 0))}
	var conn net.Conn
	var dialErr error
	if !Poll(ctx, 60, 100*time.Millisecond, func() bool {
		conn, dialErr = dialer.DialContext(ctx, "tcp4", address)
		// refuse-by-default: guards Poll -- false retries readiness; dialErr is retained and returned if the barrier fails.
		return dialErr == nil
	}) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("srv6 dual-stack listener %s did not accept %s: %w", address, source, dialErr)
	}
	failed := true
	defer func() {
		if failed {
			_ = conn.Close() //nolint:errcheck // Failed handshake owns this socket.
		}
	}()
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, err
	}
	id := source.As4()
	open := &message.Open{
		Version:        4,
		MyAS:           asn,
		HoldTime:       0,
		BGPIdentifier:  binary.BigEndian.Uint32(id[:]),
		OptionalParams: []byte{2, 12, 1, 4, 0, 1, 0, 1, 1, 4, 0, 2, 0, 1},
	}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
		return nil, err
	}
	if err := delayOpenFixtureRead(conn, 1); err != nil {
		return nil, err
	}
	if err := delayOpenFixtureRead(conn, 4); err != nil {
		return nil, err
	}
	if _, err := conn.Write(message.PackTo(&message.Keepalive{}, nil)); err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	failed = false
	return conn, nil
}

// srv6BuildPrefixSIDAttr encodes a BGP Prefix-SID attribute (code 40) carrying
// one SRv6 L3 Service TLV (type 5) with one SRv6 SID Information Sub-TLV.
func srv6BuildPrefixSIDAttr(sid netip.Addr) []byte {
	rawSID := sid.As16()
	sidInfo := make([]byte, 0, 24)
	sidInfo = append(sidInfo, 1, 0, 21, 0) // Sub-TLV Type=1, Len=21, Reserved=0
	sidInfo = append(sidInfo, rawSID[:]...)
	sidInfo = append(sidInfo, 0, 0, 0, 0) // Flags=0, Behavior=0x0000, Reserved=0

	serviceTLV := make([]byte, 0, 4+len(sidInfo))
	serviceTLV = append(serviceTLV, 5, 0, byte(1+len(sidInfo)), 0) // Type=5, Len=25, Reserved=0
	serviceTLV = append(serviceTLV, sidInfo...)

	attr := make([]byte, 0, 3+len(serviceTLV))
	attr = append(attr, 0xC0, byte(attribute.AttrPrefixSID), byte(len(serviceTLV)))
	attr = append(attr, serviceTLV...)
	return attr
}

// srv6BuildIPv4Update constructs a 2-octet-ASN BGP UPDATE announcing one IPv4
// /24 prefix with optional SRv6 L3 Service SID Prefix-SID attribute.
func srv6BuildIPv4Update(prefix, nextHop string, originASN uint16, localPref uint32, sid *netip.Addr) ([]byte, error) {
	pfx, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, err
	}
	hop, err := netip.ParseAddr(nextHop)
	if err != nil {
		return nil, err
	}
	if !pfx.Addr().Is4() {
		return nil, errors.New("srv6 IPv4 update requires IPv4 prefix")
	}
	if pfx.Bits() != 24 {
		return nil, errors.New("srv6 IPv4 update requires /24 prefix")
	}
	if !hop.Is4() {
		return nil, errors.New("srv6 IPv4 update requires IPv4 next hop")
	}
	nh := hop.As4()
	dst := pfx.Addr().As4()
	attrs := []byte{
		0x40, 1, 1, 0, // ORIGIN: IGP
		0x40, 2, 4, 2, 1, byte(originASN >> 8), byte(originASN), // AS_PATH: [originASN]
		0x40, 3, 4, nh[0], nh[1], nh[2], nh[3], // NEXT_HOP
		0x40, 5, 4, // LOCAL_PREF
	}
	attrs = binary.BigEndian.AppendUint32(attrs, localPref)
	if sid != nil {
		attrs = append(attrs, srv6BuildPrefixSIDAttr(*sid)...)
	}
	update := &message.Update{
		PathAttributes: attrs,
		NLRI:           []byte{24, dst[0], dst[1], dst[2]},
	}
	return message.PackTo(update, nil), nil
}

// srv6BuildIPv6Update constructs a 2-octet-ASN BGP UPDATE announcing one IPv6
// /48 prefix via MP_REACH_NLRI with an SRv6 L3 Service SID Prefix-SID attribute.
func srv6BuildIPv6Update(prefix, nextHop string, originASN uint16, localPref uint32, sid netip.Addr) ([]byte, error) {
	pfx, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, err
	}
	hop, err := netip.ParseAddr(nextHop)
	if err != nil {
		return nil, err
	}
	if !pfx.Addr().Is6() {
		return nil, errors.New("srv6 IPv6 update requires IPv6 prefix")
	}
	if pfx.Bits() != 48 {
		return nil, errors.New("srv6 IPv6 update requires /48 prefix")
	}
	if !hop.Is6() {
		return nil, errors.New("srv6 IPv6 update requires IPv6 next hop")
	}
	nh := hop.As16()
	dst := pfx.Addr().As16()
	mpReach := make([]byte, 0, 28)
	mpReach = append(mpReach, 0x00, 0x02, 0x01, 16) // AFI=2, SAFI=1, NH Len=16
	mpReach = append(mpReach, nh[:]...)
	mpReach = append(mpReach, 0x00) // Reserved
	mpReach = append(mpReach, 48, dst[0], dst[1], dst[2], dst[3], dst[4], dst[5])

	attrs := []byte{
		0x40, 1, 1, 0, // ORIGIN: IGP
		0x40, 2, 4, 2, 1, byte(originASN >> 8), byte(originASN), // AS_PATH
		0x40, 5, 4, // LOCAL_PREF
	}
	attrs = binary.BigEndian.AppendUint32(attrs, localPref)
	attrs = append(attrs, 0x80, byte(attribute.AttrMPReachNLRI), byte(len(mpReach)))
	attrs = append(attrs, mpReach...)
	attrs = append(attrs, srv6BuildPrefixSIDAttr(sid)...)

	update := &message.Update{PathAttributes: attrs}
	return message.PackTo(update, nil), nil
}

func srv6SendIPv4Update(conn net.Conn, prefix, nextHop string, originASN uint16, localPref uint32, sid *netip.Addr) error {
	packet, err := srv6BuildIPv4Update(prefix, nextHop, originASN, localPref, sid)
	if err != nil {
		return err
	}
	return srv6WritePacket(conn, packet)
}

func srv6SendIPv6Update(conn net.Conn, prefix, nextHop string, originASN uint16, localPref uint32, sid netip.Addr) error {
	packet, err := srv6BuildIPv6Update(prefix, nextHop, originASN, localPref, sid)
	if err != nil {
		return err
	}
	return srv6WritePacket(conn, packet)
}

func srv6WritePacket(conn net.Conn, packet []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	n, err := conn.Write(packet)
	if err != nil {
		return err
	}
	if n != len(packet) {
		return io.ErrShortWrite
	}
	return conn.SetWriteDeadline(time.Time{})
}

func srv6TwoOctetASPath(asns ...uint16) []byte {
	out := make([]byte, 0, 2+2*len(asns))
	out = append(out, 2, byte(len(asns)))
	for _, asn := range asns {
		out = append(out, byte(asn>>8), byte(asn))
	}
	return out
}

// srv6DecodeIPv4Prefixes splits wire-encoded IPv4 CIDR NLRIs into canonical
// prefix strings.
func srv6DecodeIPv4Prefixes(wire []byte) ([]string, error) {
	var prefixes []string
	for off := 0; off < len(wire); {
		bits := int(wire[off])
		off++
		if bits > 32 {
			return nil, fmt.Errorf("invalid IPv4 prefix length %d", bits)
		}
		octets := (bits + 7) / 8
		if off+octets > len(wire) {
			return nil, fmt.Errorf("truncated IPv4 NLRI at offset %d", off)
		}
		var raw [4]byte
		copy(raw[:octets], wire[off:off+octets])
		off += octets
		pfx := netip.PrefixFrom(netip.AddrFrom4(raw), bits).Masked()
		prefixes = append(prefixes, pfx.String())
	}
	return prefixes, nil
}

// srv6WaitWireEvents reads BGP UPDATEs from conn until every expected
// announcement (with exact AS_PATH) and withdrawal has been observed.
func srv6WaitWireEvents(conn net.Conn, want srv6WireExpectation) error {
	pendingAnn := make(map[string][]byte, len(want.announced))
	for k, v := range want.announced {
		pendingAnn[k] = v
	}
	pendingWdr := make(map[string]bool, len(want.withdrawn))
	for k, v := range want.withdrawn {
		if v {
			pendingWdr[k] = true
		}
	}
	for len(pendingAnn) > 0 || len(pendingWdr) > 0 {
		update, err := atomicAggregatePolicyRead(conn)
		if err != nil {
			return fmt.Errorf("wire events incomplete (pending announce=%v withdraw=%v): %w", pendingAnn, pendingWdr, err)
		}
		if len(update.WithdrawnRoutes) > 0 {
			wdrList, err := srv6DecodeIPv4Prefixes(update.WithdrawnRoutes)
			if err != nil {
				return err
			}
			for _, pfx := range wdrList {
				delete(pendingWdr, pfx)
			}
		}
		if len(update.NLRI) > 0 {
			annList, err := srv6DecodeIPv4Prefixes(update.NLRI)
			if err != nil {
				return err
			}
			_, _, path, found := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
			if !found {
				return errors.New("wire UPDATE omitted AS_PATH")
			}
			for _, pfx := range annList {
				expectedPath, expected := pendingAnn[pfx]
				if !expected {
					continue
				}
				if !bytes.Equal(path, expectedPath) {
					return fmt.Errorf("prefix %s advertised with AS_PATH %x, want %x", pfx, path, expectedPath)
				}
				delete(pendingAnn, pfx)
			}
		}
	}
	return nil
}

func srv6WaitReceivedCount(ctx context.Context, plugin *sdk.Plugin, prefix string, want int) error {
	var last string
	if Poll(ctx, srv6BestPathPollAttempts, srv6BestPathPollDelay, func() bool {
		raw, err := requireDone02(ctx, plugin, srv6BestPathReceivedCmd)
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(raw)
		var answer srv6RIBAnswer
		if err := decode02(raw, &answer); err != nil {
			last = err.Error()
			return false
		}
		return srv6CountPrefixRows(answer.Routes, prefix) >= want
	}) {
		return nil
	}
	return fmt.Errorf("show bgp rib | received never held %d rows for %s: %s", want, prefix, last)
}

func srv6CountPrefixRows(rows []srv6RIBRow, prefix string) int {
	count := 0
	for _, row := range rows {
		if row.Prefix == prefix {
			count++
		}
	}
	return count
}

func srv6RowMatchesIneligible(rows []srv6RIBRow, peer, prefix string, wantIneligible bool) bool {
	for _, row := range rows {
		if row.Peer != peer {
			continue
		}
		if row.Prefix != prefix {
			continue
		}
		if wantIneligible {
			if row.SRv6SIDResolvable == nil {
				return false
			}
			if *row.SRv6SIDResolvable {
				return false
			}
			return row.IneligibleReason == srv6BestPathIneligibleKey
		}
		if row.SRv6SIDResolvable != nil {
			return false
		}
		return row.IneligibleReason != srv6BestPathIneligibleKey
	}
	return false
}

func srv6WaitRowMarkedIneligible(ctx context.Context, plugin *sdk.Plugin, peer, prefix string, wantIneligible bool) error {
	var last string
	if Poll(ctx, srv6BestPathPollAttempts, srv6BestPathPollDelay, func() bool {
		raw, err := requireDone02(ctx, plugin, srv6BestPathReceivedCmd)
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(raw)
		var answer srv6RIBAnswer
		if err := decode02(raw, &answer); err != nil {
			last = err.Error()
			return false
		}
		return srv6RowMatchesIneligible(answer.Routes, peer, prefix, wantIneligible)
	}) {
		return nil
	}
	return fmt.Errorf("show bgp rib row (%s, %s) ineligible=%t not reached: %s", peer, prefix, wantIneligible, last)
}

func srv6VerifyMatchPipe(ctx context.Context, plugin *sdk.Plugin, prefix string, wantPresent bool) error {
	var last string
	if Poll(ctx, srv6BestPathPollAttempts, srv6BestPathPollDelay, func() bool {
		raw, err := requireDone02(ctx, plugin, srv6BestPathMatchCmd)
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(raw)
		hasPrefix := strings.Contains(last, prefix)
		hasReason := strings.Contains(last, srv6BestPathIneligibleKey)
		if wantPresent {
			return hasPrefix && hasReason
		}
		return !hasPrefix
	}) {
		return nil
	}
	return fmt.Errorf("%s presence=%t for %s not satisfied: %s", srv6BestPathMatchCmd, wantPresent, prefix, last)
}

func srv6DoctorHasSRv6Finding(answer srv6DoctorAnswer, wantPrefix, wantSID string) bool {
	for _, diag := range answer.Diagnostics {
		if diag.Code != srv6BestPathDoctorCode {
			continue
		}
		if diag.Severity != "warning" {
			continue
		}
		if diag.Fix != srv6BestPathDoctorFix {
			continue
		}
		if wantPrefix != "" && !strings.Contains(diag.Message, wantPrefix) {
			continue
		}
		if wantSID != "" && !strings.Contains(diag.Message, wantSID) {
			continue
		}
		return true
	}
	return false
}

func srv6DoctorHasCode(answer srv6DoctorAnswer, code string) bool {
	for _, diag := range answer.Diagnostics {
		if diag.Code == code {
			return true
		}
	}
	return false
}

func srv6WaitDoctorFinding(ctx context.Context, plugin *sdk.Plugin, wantPresent bool, wantPrefix, wantSID string) error {
	var last string
	if Poll(ctx, srv6BestPathPollAttempts, srv6BestPathPollDelay, func() bool {
		raw, err := requireDone02(ctx, plugin, srv6BestPathDoctorCmd)
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(raw)
		var answer srv6DoctorAnswer
		if err := decode02(raw, &answer); err != nil {
			last = err.Error()
			return false
		}
		if wantPresent {
			return srv6DoctorHasSRv6Finding(answer, wantPrefix, wantSID)
		}
		return !srv6DoctorHasCode(answer, srv6BestPathDoctorCode)
	}) {
		return nil
	}
	return fmt.Errorf("show doctor %s present=%t not reached: %s", srv6BestPathDoctorCode, wantPresent, last)
}

func srv6WaitExclusionMetric(ctx context.Context, plugin *sdk.Plugin) error {
	var last string
	if Poll(ctx, srv6BestPathPollAttempts, srv6BestPathPollDelay, func() bool {
		res := command07(ctx, plugin, "show metrics values")
		if res.status != statusDone {
			var tb textbuf.Buffer
			tb.Str("status=").Str(res.status)
			if res.err != nil {
				tb.Str(" err=").Str(res.err.Error())
			}
			last = tb.String()
			return false
		}
		metricsText := metricsText07(res.data)
		last = metricsText
		val, ok := counter07(metricsText, srv6BestPathExclMetric)
		if !ok {
			return false
		}
		return val >= 1
	}) {
		return nil
	}
	return fmt.Errorf("metric %s >= 1 not observed: %.600s", srv6BestPathExclMetric, last)
}

func srv6WithPeerBypass(schema *config.Schema, baseText, bypassBlock string) (string, error) {
	tree, err := config.NewParser(schema).Parse(baseText)
	if err != nil {
		return "", err
	}
	var snippet textbuf.Buffer
	snippet.Str("bgp { peer sr-peer { ").Str(bypassBlock).Str(" } }")
	addition, err := config.NewParser(schema).Parse(snippet.String())
	if err != nil {
		return "", err
	}
	bgpNode := tree.GetContainer("bgp")
	if bgpNode == nil {
		return "", errors.New("bypass fixture config missing bgp container")
	}
	peerNode := bgpNode.GetList("peer")["sr-peer"]
	if peerNode == nil {
		return "", errors.New("bypass fixture config missing peer sr-peer")
	}
	addPeer := addition.GetContainer("bgp").GetList("peer")["sr-peer"]
	peerNode.SetContainer("srv6-sid-resolvability", addPeer.GetContainer("srv6-sid-resolvability"))
	return config.Serialize(tree, schema), nil
}
