// Design: docs/architecture/testing/interop.md -- RFC 7947 Section 2.1 filtered Adj-RIB-In retention and replay exclusion.
// Related: check_special.go -- registry entry for bgp-route-server-filtered-adj-rib-in-frr.
// Related: check_rfc.go -- checkRouteServerASPath, the two-client FRR/BIRD route-server topology this extends.
// RFC: rfc/short/rfc7947.md -- Section 2.1, pre-policy Adj-RIB-In acceptance with policy omission from Loc-RIB and replay.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	routeServerFilteredScenario = "bgp-route-server-filtered-adj-rib-in-frr"
	rsFilteredAcceptedPrefix    = "10.99.0.0/24"
	rsFilteredDeniedPrefix      = "10.99.1.0/24"
	rsFilteredClientASPath      = "65002"
	rsFilteredPolicyStep        = "peer-policy-chain"
	rsFilteredAcceptedNLRIHex   = "180a6300"
	rsFilteredDeniedNLRIHex     = "180a6301"
)

type adjRIBInShowEnvelope struct {
	AdjRIBIn map[string][]adjRIBInShowRoute `json:"adj-rib-in"`
}

type adjRIBInShowRoute struct {
	Family     string `json:"family"`
	NLRIHex    string `json:"nlri-hex"`
	Filtered   *bool  `json:"filtered"`
	FilterStep string `json:"filter-step"`
}

type adjRIBInStatusEnvelope struct {
	TotalRoutes    int `json:"total-routes"`
	FilteredRoutes int `json:"filtered-routes"`
}

func requireRSFilteredAdjRIBIn(raw, frrAddress string) error {
	var env adjRIBInShowEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return fmt.Errorf("decode show bgp adj-rib-in: %w", err)
	}
	rows := env.AdjRIBIn[frrAddress]
	if len(rows) != 2 {
		return fmt.Errorf("peer %s has %d Adj-RIB-In entries, want 2", frrAddress, len(rows))
	}
	var sawAccepted, sawDenied bool
	for _, row := range rows {
		nlri := strings.ToLower(row.NLRIHex)
		if row.Filtered == nil {
			return fmt.Errorf("Adj-RIB-In row %s missing filtered boolean", nlri)
		}
		switch nlri {
		case rsFilteredAcceptedNLRIHex:
			if *row.Filtered {
				return fmt.Errorf("accepted prefix %s unexpectedly marked filtered", rsFilteredAcceptedPrefix)
			}
			sawAccepted = true
		case rsFilteredDeniedNLRIHex:
			if !*row.Filtered {
				return fmt.Errorf("denied prefix %s not marked filtered", rsFilteredDeniedPrefix)
			}
			if row.FilterStep != rsFilteredPolicyStep {
				return fmt.Errorf("denied prefix %s filter-step=%q, want %q", rsFilteredDeniedPrefix, row.FilterStep, rsFilteredPolicyStep)
			}
			sawDenied = true
		default:
			return fmt.Errorf("unexpected Adj-RIB-In nlri-hex %q for peer %s", nlri, frrAddress)
		}
	}
	if !sawAccepted {
		return fmt.Errorf("peer %s Adj-RIB-In missing accepted prefix %s", frrAddress, rsFilteredAcceptedPrefix)
	}
	if !sawDenied {
		return fmt.Errorf("peer %s Adj-RIB-In missing denied prefix %s", frrAddress, rsFilteredDeniedPrefix)
	}
	return nil
}

func requireRSFilteredStatus(raw string) error {
	var status adjRIBInStatusEnvelope
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		return fmt.Errorf("decode show bgp adj-rib-in status: %w", err)
	}
	if status.TotalRoutes != 2 {
		return fmt.Errorf("adj-rib-in status total-routes=%d, want 2", status.TotalRoutes)
	}
	if status.FilteredRoutes != 1 {
		return fmt.Errorf("adj-rib-in status filtered-routes=%d, want 1", status.FilteredRoutes)
	}
	return nil
}

func requireBIRDDeniedAbsent(table string) error {
	if !strings.Contains(table, rsFilteredAcceptedPrefix) {
		return fmt.Errorf("BIRD table missing accepted control %s", rsFilteredAcceptedPrefix)
	}
	if strings.Contains(table, rsFilteredDeniedPrefix) {
		return fmt.Errorf("BIRD table leaked denied prefix %s", rsFilteredDeniedPrefix)
	}
	return nil
}

// checkRouteServerFilteredAdjRIBIn proves RFC 7947 Section 2.1 end to end
// against FRR 10.7.1 and BIRD 3.4.0:
//   - FRR originates both an accepted prefix (10.99.0.0/24) and an
//     import-policy-denied prefix (10.99.1.0/24).
//   - Ze retains both in bgp-adj-rib-in with 10.99.1.0/24 marked filtered
//     (filter-step peer-policy-chain) and excludes 10.99.1.0/24 from bgp-rib.
//   - BIRD receives 10.99.0.0/24 with AS_PATH 65002 and never receives
//     10.99.1.0/24 on the live rail.
//   - After BIRD bounces its BGP session (`birdc disable ze_peer` then
//     `birdc enable ze_peer`), bgp-rs replays 10.99.0.0/24 from bgp-adj-rib-in
//     and still never advertises 10.99.1.0/24.
func checkRouteServerFilteredAdjRIBIn(ctx context.Context, check *interoplab.CheckContext) error {
	const name = routeServerFilteredScenario
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("filtered route server scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)
	frrAddress := networkHostAddress(check.Network, 3)

	// Assertions 1-3: both client sessions established and accepted control reaches BIRD live.
	relay := []operation{
		{kind: opFRRSession, argument: zeAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opBIRDRoute, argument: rsFilteredAcceptedPrefix, timeout: 60 * time.Second},
	}
	for index := range relay {
		if err := runOperation(ctx, check.Network, check.Lab, &relay[index]); err != nil {
			return fail(index+1, err)
		}
	}

	// Assertion 4: BIRD holds accepted control with transparent client AS_PATH 65002.
	route, err := check.Lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show route for " + rsFilteredAcceptedPrefix + " all"}, nil)
	if err != nil {
		return fail(4, err)
	}
	if err := requireBIRDASPath(route, rsFilteredAcceptedPrefix, rsFilteredClientASPath); err != nil {
		return fail(4, err)
	}

	// Assertion 5: Ze's Adj-RIB-In retains BOTH FRR prefixes, marking 10.99.1.0/24 filtered.
	showAdj := zeCommand("show bgp adj-rib-in")
	lastAdj, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     45 * time.Second,
		Interval:    time.Second,
		Description: "Ze Adj-RIB-In retains accepted and import-filtered FRR routes",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", showAdj, queryEnvironment("ze", showAdj))
	}, func(output string) bool {
		return requireRSFilteredAdjRIBIn(output, frrAddress) == nil
	})
	if err != nil {
		return fail(5, interoplab.WithSearchedOutput(err, lastAdj))
	}

	// Assertion 6: show bgp adj-rib-in status reports total-routes=2, filtered-routes=1.
	statusCmd := zeCommand("show bgp adj-rib-in status")
	statusOut, err := check.Lab.Query(ctx, "ze", statusCmd, queryEnvironment("ze", statusCmd))
	if err != nil {
		return fail(6, err)
	}
	if err := requireRSFilteredStatus(statusOut); err != nil {
		return fail(6, interoplab.WithSearchedOutput(err, statusOut))
	}

	// Assertion 7: Ze's Loc-RIB holds 10.99.0.0/24 and omits 10.99.1.0/24.
	ribCmd := zeCommand("show bgp rib")
	lastRib, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "Ze bgp-rib holds accepted control and omits denied prefix",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", ribCmd, queryEnvironment("ze", ribCmd))
	}, func(output string) bool {
		return strings.Contains(output, rsFilteredAcceptedPrefix) && !strings.Contains(output, rsFilteredDeniedPrefix)
	})
	if err != nil {
		return fail(7, interoplab.WithSearchedOutput(err, lastRib))
	}

	// Assertion 8: BIRD's full table omits the denied prefix on the live rail.
	liveTable, err := check.Lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show route all"}, nil)
	if err != nil {
		return fail(8, err)
	}
	if err := requireBIRDDeniedAbsent(liveTable); err != nil {
		return fail(8, interoplab.WithSearchedOutput(err, liveTable))
	}

	// Assertion 9: Bounce BIRD's session so bgp-rs replays from bgp-adj-rib-in.
	if _, err := check.Lab.Exec(ctx, peerBIRD, []string{cmdBirdc, "disable " + birdZeProtocol}, nil); err != nil {
		return fail(9, fmt.Errorf("disable BIRD protocol %s: %w", birdZeProtocol, err))
	}
	if err := waitContains(ctx, check.Lab, peerBIRD, []string{cmdBirdc, birdcShowProtocols}, 30*time.Second, birdZeProtocol, "down"); err != nil {
		return fail(9, err)
	}
	if _, err := check.Lab.Exec(ctx, peerBIRD, []string{cmdBirdc, "enable " + birdZeProtocol}, nil); err != nil {
		return fail(9, fmt.Errorf("enable BIRD protocol %s: %w", birdZeProtocol, err))
	}

	// Assertions 10-11: BIRD re-establishes and receives 10.99.0.0/24 via peer-up replay.
	reestablish := []operation{
		{kind: opBIRDSession, argument: birdZeProtocol, timeout: 60 * time.Second},
		{kind: opBIRDRoute, argument: rsFilteredAcceptedPrefix, timeout: 60 * time.Second},
	}
	for index := range reestablish {
		if err := runOperation(ctx, check.Network, check.Lab, &reestablish[index]); err != nil {
			return fail(10+index, err)
		}
	}

	// Assertion 12: After replay, BIRD still holds 10.99.0.0/24 (AS_PATH 65002) and omits 10.99.1.0/24.
	replayedRoute, err := check.Lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show route for " + rsFilteredAcceptedPrefix + " all"}, nil)
	if err != nil {
		return fail(12, err)
	}
	if err := requireBIRDASPath(replayedRoute, rsFilteredAcceptedPrefix, rsFilteredClientASPath); err != nil {
		return fail(12, err)
	}
	replayedTable, err := check.Lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show route all"}, nil)
	if err != nil {
		return fail(12, err)
	}
	if err := requireBIRDDeniedAbsent(replayedTable); err != nil {
		return fail(12, interoplab.WithSearchedOutput(err, replayedTable))
	}

	// Assertion 13: FRR session remains Established and Ze still retains the filtered entry.
	if err := runOperation(ctx, check.Network, check.Lab, &relay[0]); err != nil {
		return fail(13, err)
	}
	finalAdj, err := check.Lab.Query(ctx, "ze", showAdj, queryEnvironment("ze", showAdj))
	if err != nil {
		return fail(13, err)
	}
	if err := requireRSFilteredAdjRIBIn(finalAdj, frrAddress); err != nil {
		return fail(13, interoplab.WithSearchedOutput(err, finalAdj))
	}
	return nil
}
