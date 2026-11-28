// Design: docs/architecture/testing/interop.md -- RFC 9252 Section 5 SRv6 Service SID resolvability proof against FRR.
// Related: check_special.go -- specialCheckers entry for bgp-srv6-resolvability-frr.
// Related: check_srv6_resolvability_test.go -- pure JSON predicate unit tests.
// RFC: rfc/short/rfc9252.md -- Section 5 SRv6 Service SID resolvability before BGP best-path computation.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	srv6ResolvabilityScenario      = "bgp-srv6-resolvability-frr"
	srv6ResolvabilityControlPrefix = "10.92.52.0/24"
	srv6ResolvabilityVPNRD         = "65001:1"
	srv6ResolvabilityVPNFamily     = "ipv6/mpls-vpn"
	srv6ResolvabilityLocatorPrefix = "fc00:0:1::/48"
	srv6ResolvabilityIneligibleKey = "srv6-sid-unresolvable"
	srv6ResolvabilityDoctorCode    = "doctor-bgp-srv6-sid-unresolvable"
	srv6ResolvabilityASPath        = "65001"
	srv6ResolvabilityShowVPNJSON   = "show bgp ipv6 vpn rd all json"
	// Ze's `show bgp rib` currently renders ipv6/mpls-vpn wire NLRIs (3-byte
	// label + 8-byte RD 65001:1 + 8-byte IPv6 prefix 2001:db8:9252:1::/64) as a
	// 16-byte CIDR (checkers.go, silent-fall-through.md). Whatever label FRR
	// allocates, bytes 4..15 + /152 always end with ":fd:e900:0:120:10d:b892/152".
	srv6ResolvabilityRenderedVPNKey = ":fd:e900:0:120:10d:b892/152"
)

func srv6ResolvabilityMatchesVPNPrefix(prefix string) bool {
	return strings.Contains(prefix, srv6IneligiblePrefix) || strings.HasSuffix(prefix, srv6ResolvabilityRenderedVPNKey)
}

// srv6ResolvabilityRIBRow models one received row of Ze's `show bgp rib`.
type srv6ResolvabilityRIBRow struct {
	Peer              string `json:"peer"`
	Family            string `json:"family"`
	Prefix            string `json:"prefix"`
	SRv6SIDResolvable *bool  `json:"srv6-sid-resolvable"`
	IneligibleReason  string `json:"ineligible-reason"`
}

// srv6ResolvabilityBestRow models one winner row of Ze's `show bgp rib best`.
type srv6ResolvabilityBestRow struct {
	Family   string `json:"family"`
	Prefix   string `json:"prefix"`
	BestPeer string `json:"best-peer"`
}

// srv6ResolvabilityDoctorRow models one diagnostic row of Ze's `show doctor`.
type srv6ResolvabilityDoctorRow struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Fix      string `json:"fix"`
}

// checkSRv6ResolvabilityFRR proves RFC 9252 Section 5 end-to-end against FRR:
//  1. FRR exports IPv6 VPN 2001:db8:9252:1::/64 with an SRv6 Service SID from
//     LOC1 (fc00:0:1::/48) alongside non-SRv6 control 10.92.52.0/24.
//  2. While fc00:0:1::/48 has no route in Ze's Loc-RIB, Ze propagates the
//     control to frr-sink, retains the VPN route marked srv6-sid-unresolvable,
//     reports doctor-bgp-srv6-sid-unresolvable, excludes it from best-path,
//     and does NOT advertise it to frr-sink.
//  3. Once FRR advertises fc00:0:1::/48 over ipv6 unicast (no new VPN UPDATE),
//     Ze resolves the Service SID, elects the retained VPN route, clears the
//     ineligibility and doctor warning, and advertises it to frr-sink.
//
// RFC requirement: RFC9252-5-2 negative -- FRR-originated SRv6 IPv6 VPN route whose locator fc00:0:1::/48 is unreachable on Ze is excluded from Ze's best-path, marked srv6-sid-unresolvable, and withheld from downstream peer frr-sink while the non-SRv6 control route is propagated.
// RFC requirement: RFC9252-5-2 positive -- once covering locator fc00:0:1::/48 becomes reachable on Ze, the retained SRv6 IPv6 VPN route enters Ze's best-path, clears srv6-sid-unresolvable, and is advertised to frr-sink.
func checkSRv6ResolvabilityFRR(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	if !check.Network.IPv4.IsValid() {
		return errors.New("srv6 resolvability scenario has no selected IPv4 network")
	}
	assertion := 0
	defer func() {
		if resultErr != nil {
			resultErr = checkerFailure(ctx, check.Lab, check.Source.Name, assertion, resultErr)
		}
	}()

	zeAddr := networkHostAddress(check.Network, 2)
	frrAddr := networkHostAddress(check.Network, 3)
	var sessionCmd textbuf.Buffer
	frrSession := []string{cmdVtysh, "-c", sessionCmd.Str("show bgp neighbor ").Str(zeAddr).String()}

	// Assertion 1: both FRR source and FRR sink sessions are Established.
	assertion = 1
	if err := waitContains(ctx, check.Lab, peerFRR, frrSession, 90*time.Second, "BGP state = Established"); err != nil {
		return fmt.Errorf("source FRR session not Established: %w", err)
	}
	if err := waitContains(ctx, check.Lab, peerFRRSink, frrSession, 90*time.Second, "BGP state = Established"); err != nil {
		return fmt.Errorf("sink FRR session not Established: %w", err)
	}

	// Assertion 2: FRR allocates its SRv6 VPN SID from LOC1 and exports the VPN route.
	assertion = 2
	if err := waitContains(ctx, check.Lab, peerFRR, []string{cmdVtysh, "-c", srv6IneligibleBGPState}, 60*time.Second, srv6IneligibleLocatorHeld); err != nil {
		return fmt.Errorf("source FRR did not receive LOC1 from zebra: %w", err)
	}
	if _, err := check.Lab.Exec(ctx, peerFRR, srv6IneligibleReexport, nil); err != nil {
		return fmt.Errorf("source FRR first VPN re-export: %w", err)
	}
	if err := waitContains(ctx, check.Lab, peerFRR, []string{cmdVtysh, "-c", srv6IneligibleBGPState}, 60*time.Second, srv6IneligibleSIDHeld); err != nil {
		return fmt.Errorf("source FRR did not allocate SRv6 VPN SID from LOC1: %w", err)
	}
	if _, err := check.Lab.Exec(ctx, peerFRR, srv6IneligibleReexport, nil); err != nil {
		return fmt.Errorf("source FRR second VPN re-export: %w", err)
	}
	if err := waitContains(ctx, check.Lab, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv6 vpn"}, 60*time.Second, srv6IneligiblePrefix); err != nil {
		return fmt.Errorf("source FRR did not export SRv6 IPv6 VPN prefix: %w", err)
	}

	// Assertion 3: positive control proves Ze -> frr-sink forwarding is live.
	assertion = 3
	var ctrlCmd textbuf.Buffer
	sinkControl := []string{cmdVtysh, "-c", ctrlCmd.Str("show bgp ipv4 unicast ").Str(srv6ResolvabilityControlPrefix).Str(" json").String()}
	if err := waitContains(ctx, check.Lab, peerFRRSink, sinkControl, 60*time.Second, srv6ResolvabilityControlPrefix); err != nil {
		return fmt.Errorf("sink FRR did not receive non-SRv6 control route: %w", err)
	}

	// Assertion 4: Ze retains the SRv6 VPN route marked srv6-sid-unresolvable,
	// excludes it from best-path, reports doctor-bgp-srv6-sid-unresolvable, and
	// withholds it from frr-sink.
	assertion = 4
	showReceived := zeCommand("show bgp rib | received")
	if last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     60 * time.Second,
		Interval:    time.Second,
		Description: "Ze marks FRR SRv6 VPN route srv6-sid-unresolvable",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", showReceived, queryEnvironment("ze", showReceived))
	}, func(output string) bool {
		return srv6ResolvabilityRequireRowExcluded(output, frrAddr, srv6ResolvabilityVPNFamily, true) == nil
	}); err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("Ze did not mark SRv6 VPN route srv6-sid-unresolvable: %w", err), last)
	}

	showDoctor := zeCommand("show doctor")
	if last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "Ze reports doctor-bgp-srv6-sid-unresolvable",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", showDoctor, queryEnvironment("ze", showDoctor))
	}, func(output string) bool {
		return srv6ResolvabilityRequireDoctor(output, true) == nil
	}); err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("Ze show doctor missing %s: %w", srv6ResolvabilityDoctorCode, err), last)
	}

	showBest := zeCommand("show bgp rib best")
	bestOut, err := check.Lab.Query(ctx, "ze", showBest, queryEnvironment("ze", showBest))
	if err != nil {
		return fmt.Errorf("query Ze show bgp rib best: %w", err)
	}
	if err := srv6ResolvabilityRequireBestVPN(bestOut, frrAddr, false); err != nil {
		return interoplab.WithSearchedOutput(err, bestOut)
	}

	sinkVPNOut, err := check.Lab.Query(ctx, peerFRRSink, []string{cmdVtysh, "-c", srv6ResolvabilityShowVPNJSON}, nil)
	if err != nil {
		return fmt.Errorf("query sink FRR IPv6 VPN table: %w", err)
	}
	if err := srv6ResolvabilityRequireFRRVPN(sinkVPNOut, srv6ResolvabilityVPNRD, zeAddr, ""); err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("sink FRR unexpectedly held unresolvable SRv6 VPN route: %w", err), sinkVPNOut)
	}

	// Assertion 5: advertise covering IPv6 locator fc00:0:1::/48 from FRR over
	// ipv6 unicast (no new VPN UPDATE); Ze must elect the retained SRv6 VPN
	// route, clear ineligibility and doctor warnings, and advertise it to frr-sink.
	assertion = 5
	if _, err := check.Lab.Exec(ctx, peerFRR, []string{
		cmdVtysh, "-c", frrConfigureTerminal,
		"-c", "router bgp 65001",
		"-c", "address-family ipv6 unicast",
		"-c", "network " + srv6ResolvabilityLocatorPrefix,
	}, nil); err != nil {
		return fmt.Errorf("advertise covering locator %s from FRR: %w", srv6ResolvabilityLocatorPrefix, err)
	}

	if last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     60 * time.Second,
		Interval:    time.Second,
		Description: "Ze selects retained SRv6 VPN route after locator arrives",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", showBest, queryEnvironment("ze", showBest))
	}, func(output string) bool {
		return srv6ResolvabilityRequireBestVPN(output, frrAddr, true) == nil
	}); err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("Ze did not select retained SRv6 VPN route after locator resolved: %w", err), last)
	}

	if last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "Ze clears srv6-sid-unresolvable on retained VPN route",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", showReceived, queryEnvironment("ze", showReceived))
	}, func(output string) bool {
		return srv6ResolvabilityRequireRowExcluded(output, frrAddr, srv6ResolvabilityVPNFamily, false) == nil
	}); err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("Ze did not clear srv6-sid-unresolvable after locator resolved: %w", err), last)
	}

	if last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "Ze clears doctor-bgp-srv6-sid-unresolvable",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", showDoctor, queryEnvironment("ze", showDoctor))
	}, func(output string) bool {
		return srv6ResolvabilityRequireDoctor(output, false) == nil
	}); err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("Ze show doctor retained %s after locator resolved: %w", srv6ResolvabilityDoctorCode, err), last)
	}

	if last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     60 * time.Second,
		Interval:    time.Second,
		Description: "sink FRR receives SRv6 VPN route after locator resolves on Ze",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, peerFRRSink, []string{cmdVtysh, "-c", srv6ResolvabilityShowVPNJSON}, nil)
	}, func(output string) bool {
		return srv6ResolvabilityRequireFRRVPN(output, srv6ResolvabilityVPNRD, zeAddr, srv6IneligiblePrefix) == nil
	}); err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("sink FRR did not receive SRv6 VPN route after locator resolved: %w", err), last)
	}

	// Assertion 6: both sessions remain Established.
	assertion = 6
	if err := waitContains(ctx, check.Lab, peerFRR, frrSession, 15*time.Second, "BGP state = Established"); err != nil {
		return fmt.Errorf("source FRR session dropped: %w", err)
	}
	if err := waitContains(ctx, check.Lab, peerFRRSink, frrSession, 15*time.Second, "BGP state = Established"); err != nil {
		return fmt.Errorf("sink FRR session dropped: %w", err)
	}
	return nil
}

// srv6ResolvabilityRequireRowExcluded verifies Ze's `show bgp rib | received`
// JSON holds a row from peer in fam whose SRv6 Service SID exclusion state
// matches wantExcluded.
func srv6ResolvabilityRequireRowExcluded(output, peer, fam string, wantExcluded bool) error {
	var doc struct {
		Routes []srv6ResolvabilityRIBRow `json:"routes"`
	}
	if err := json.Unmarshal([]byte(output), &doc); err != nil {
		return fmt.Errorf("decode Ze show bgp rib JSON: %w", err)
	}
	for _, row := range doc.Routes {
		if row.Peer != peer {
			continue
		}
		if row.Family != fam {
			continue
		}
		if !srv6ResolvabilityMatchesVPNPrefix(row.Prefix) {
			continue
		}
		if wantExcluded {
			if row.SRv6SIDResolvable == nil {
				return errors.New("SRv6 row missing srv6-sid-resolvable")
			}
			if *row.SRv6SIDResolvable {
				return errors.New("SRv6 row has srv6-sid-resolvable=true, want false")
			}
			if row.IneligibleReason != srv6ResolvabilityIneligibleKey {
				return fmt.Errorf("SRv6 row has ineligible-reason %q, want %q", row.IneligibleReason, srv6ResolvabilityIneligibleKey)
			}
			return nil
		}
		if row.SRv6SIDResolvable != nil {
			return errors.New("resolved SRv6 row still carries srv6-sid-resolvable")
		}
		if row.IneligibleReason == srv6ResolvabilityIneligibleKey {
			return errors.New("resolved SRv6 row still carries srv6-sid-unresolvable")
		}
		return nil
	}
	return fmt.Errorf("Ze received RIB lacks %s row from %s", fam, peer)
}

// srv6ResolvabilityRequireBestVPN verifies Ze's `show bgp rib best` JSON
// presence of an ipv6/mpls-vpn winner from peer matches wantPresent.
// `show bgp rib best | json` renders either a top-level array of best rows or
// a {"best-path":[...]} object depending on pipe flattening; accept both.
func srv6ResolvabilityRequireBestVPN(output, peer string, wantPresent bool) error {
	trimmed := strings.TrimSpace(output)
	var rows []srv6ResolvabilityBestRow
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &rows); err != nil {
			return fmt.Errorf("decode Ze show bgp rib best JSON: %w", err)
		}
	} else {
		var doc struct {
			BestPath []srv6ResolvabilityBestRow `json:"best-path"`
		}
		if err := json.Unmarshal([]byte(trimmed), &doc); err != nil {
			return fmt.Errorf("decode Ze show bgp rib best JSON: %w", err)
		}
		rows = doc.BestPath
	}
	found := false
	for _, row := range rows {
		if row.Family != srv6ResolvabilityVPNFamily {
			continue
		}
		if !srv6ResolvabilityMatchesVPNPrefix(row.Prefix) {
			continue
		}
		if row.BestPeer == peer {
			found = true
			break
		}
	}
	if found != wantPresent {
		return fmt.Errorf("Ze best-path %s from %s present=%t, want %t", srv6ResolvabilityVPNFamily, peer, found, wantPresent)
	}
	return nil
}

// srv6ResolvabilityRequireDoctor verifies Ze's `show doctor` JSON presence of
// doctor-bgp-srv6-sid-unresolvable matches wantPresent.
func srv6ResolvabilityRequireDoctor(output string, wantPresent bool) error {
	var doc struct {
		Diagnostics []srv6ResolvabilityDoctorRow `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(output), &doc); err != nil {
		return fmt.Errorf("decode Ze show doctor JSON: %w", err)
	}
	found := false
	for _, diag := range doc.Diagnostics {
		if diag.Code != srv6ResolvabilityDoctorCode {
			continue
		}
		if diag.Severity == "warning" {
			found = true
			break
		}
	}
	if found != wantPresent {
		return fmt.Errorf("Ze show doctor %s present=%t, want %t", srv6ResolvabilityDoctorCode, found, wantPresent)
	}
	return nil
}

// srv6ResolvabilityRequireFRRVPN validates FRR 10.7.1 `show bgp ipv6 vpn rd all
// json`. When wantPrefix is empty, the table MUST be empty (`{}`). When
// wantPrefix is non-empty, RD rd MUST carry wantPrefix from neighbor with
// AS_PATH 65001.
func srv6ResolvabilityRequireFRRVPN(output, rd, neighbor, wantPrefix string) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return fmt.Errorf("decode FRR VPNv6 table JSON: %w", err)
	}
	if envelope == nil {
		return errors.New("FRR VPNv6 table JSON is null")
	}
	if wantPrefix == "" {
		if len(envelope) != 0 {
			return fmt.Errorf("FRR VPNv6 table is non-empty: %s", strings.TrimSpace(output))
		}
		return nil
	}
	var table struct {
		Routes struct {
			RDs map[string]map[string][]struct {
				Peer string `json:"peerId"`
				Path string `json:"path"`
			} `json:"routeDistinguishers"`
		} `json:"routes"`
	}
	if err := json.Unmarshal([]byte(output), &table); err != nil {
		return fmt.Errorf("decode FRR VPNv6 RD routes: %w", err)
	}
	rdRoutes := table.Routes.RDs[rd]
	paths := rdRoutes[wantPrefix]
	if len(paths) == 0 {
		return fmt.Errorf("FRR VPNv6 RD %s lacks prefix %s", rd, wantPrefix)
	}
	for _, path := range paths {
		if path.Peer != neighbor {
			continue
		}
		if strings.TrimSpace(path.Path) == srv6ResolvabilityASPath {
			return nil
		}
	}
	return fmt.Errorf("FRR VPNv6 RD %s prefix %s has no path from %s with AS_PATH %s", rd, wantPrefix, neighbor, srv6ResolvabilityASPath)
}
