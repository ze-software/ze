// Design: docs/architecture/testing/interop.md -- concurrent state validation.
// Related: checkers.go -- registry entry for the load scenario.
package bgp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	ribLoadRoutes  = 256
	ribLoadWalkers = 8
	ribLoadWindow  = 45 * time.Second
)

var specialCheckers = map[string]interoplab.Checker{
	"bfd-frr":                               checkBFDFailover,
	"show-rib-under-frr-load":               checkShowRIBUnderFRRLoad,
	"bgp-addpath-rail-agreement-speaker":    checkAddPathRailAgreement,
	"bgp-addpath-readvertise-collision-frr": checkAddPathReadvertiseCollision,
	"bgp-local-pref-strip-gobgp":            checkLocalPrefStrip,
	"bgp-med-across-as-gobgp":               checkMEDAcrossAS,
	"bgp-med-remove-configured-gobgp":       checkMEDRemovalConfiguration,
	scenarioWireEditAPIOriginBIRD:           checkWireEditAPIOriginBIRD,
	"bgp-relay-withdraw-reflector-frr":      checkReflectorWithdrawal,
	"bgp-relay-withdraw-shape-frr":          checkRelayWithdrawalShape,
	"bgp-rfc2545-linklocal-nexthop-frr":     checkRFC2545NextHops,
	"bgp-rfc7606-relay-shape-frr":           checkRFC7606MixedUpdate,
	"bgp-rfc7606-typed-nlri-discard":        checkRFC7606TypedNLRIDiscard,
	"bgp-rfc7999-blackhole-frr":             checkRFC7999Blackhole,
	"bgp-role-otc-withdraw-frr":             checkOTCWithdrawal,
	"bgp-route-server-frr":                  checkRouteServerASPath,
	"bgp-self-nexthop-withheld-frr":         checkSelfNextHopWithheld,
	"bgp-wellknown-noexport-frr":            checkNoExportBoundary,
	"bgp-holdtime-deadpeer-frr":             checkHoldtimeDeadPeer,
	"isis-p2p-frr":                          checkISISDynamicHostname,
	"isis-purge-reorig-frr":                 checkISISOwnLSPPurge,
	"no-family-peer-eor-frr":                checkNoFamilyEndOfRIB,
	"ospf-lfa-frr":                          checkOSPFLFA,
	"ospf-stub-nssa-frr":                    checkNSSADefault,
	"ospf-ti-lfa-frr":                       checkOSPFTILFA,
	"bgp-max-prefix-per-family-frr":         checkMaxPrefixPerFamily,
}

func checkBFDFailover(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	if !check.Network.IPv4.IsValid() {
		return errors.New("BFD scenario has no selected IPv4 network")
	}
	neighbor := networkHostAddress(check.Network, 2)
	var command textbuf.Buffer
	session := []string{
		cmdVtysh,
		"-c",
		command.Str("show bgp neighbor ").Str(neighbor).String(),
	}
	if err := waitContains(ctx, check.Lab, peerFRR, session, 90*time.Second, "BGP state = Established"); err != nil {
		return err
	}
	if err := waitContains(ctx, check.Lab, peerFRR, []string{cmdVtysh, "-c", frrShowBFDPeers}, 90*time.Second, neighbor, bfdStatusUp); err != nil {
		return err
	}

	started := time.Now()
	drop := []string{cmdIptables, "-I", iptablesChainOutput, "1", "-p", iptablesProtocolUDP, iptablesDestinationPortFlag, "3784", "-j", iptablesTargetDrop}
	if _, err := check.Lab.Exec(ctx, peerFRR, drop, nil); err != nil {
		return err
	}
	restored := false
	defer func() {
		if restored {
			return
		}
		restore := []string{cmdIptables, "-D", iptablesChainOutput, "-p", iptablesProtocolUDP, iptablesDestinationPortFlag, "3784", "-j", iptablesTargetDrop}
		_, restoreErr := check.Lab.Exec(context.WithoutCancel(ctx), peerFRR, restore, nil)
		if restoreErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("restore BFD traffic: %w", restoreErr)
		}
	}()

	_, report, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     5 * time.Second,
		Interval:    100 * time.Millisecond,
		Description: "BGP teardown after BFD failure",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, session, nil)
	}, bfdSessionDown)
	if err != nil {
		return errors.New("BGP session still Established 5.0s after BFD link break")
	}
	elapsed := time.Since(started)
	elapsed = max(elapsed, report.Elapsed)
	if err := requireBFDTeardownBudget(elapsed); err != nil {
		return err
	}
	restore := []string{cmdIptables, "-D", iptablesChainOutput, "-p", iptablesProtocolUDP, iptablesDestinationPortFlag, "3784", "-j", iptablesTargetDrop}
	if _, err := check.Lab.Exec(ctx, peerFRR, restore, nil); err != nil {
		return fmt.Errorf("restore BFD traffic: %w", err)
	}
	restored = true
	return nil
}

func bfdSessionDown(output string) bool {
	return !strings.Contains(output, "BGP state = Established")
}

func requireBFDTeardownBudget(elapsed time.Duration) error {
	if elapsed >= 2*time.Second {
		return fmt.Errorf("BGP teardown took %.3fs, expected < 2.0s", elapsed.Seconds())
	}
	return nil
}

// RFC requirement: RFC4456-8-1 positive -- Ze sets ORIGINATOR_ID on a route it reflects between two clients, observed on the wire by the receiving peer, so the identifier is set when a route IS reflected.
// RFC requirement: RFC4456-8-1 negative -- Ze creates no ORIGINATOR_ID on the withdrawal of that same route, asserted byte-exact on the wire, because the clause's condition ("reflects a route") is not met.
// RFC requirement: RFC4456-8-2 positive -- the same reflected route carries a CLUSTER_LIST, prepended with Ze's configured cluster-id.
// RFC requirement: RFC4456-8-2 negative -- and the withdrawal carries none, for the same reason, in the same byte-exact assertion.
func checkReflectorWithdrawal(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("route-reflector scenario has no selected IPv4 network")
	}
	zeAddress := networkHostAddress(check.Network, 2)
	var command textbuf.Buffer
	session := []string{
		cmdVtysh,
		"-c",
		command.Str("show bgp neighbor ").Str(zeAddress).String(),
	}
	if err := waitContains(
		ctx,
		check.Lab,
		peerFRR,
		session,
		90*time.Second,
		"BGP state = Established",
	); err != nil {
		return err
	}
	originator, cluster := reflectorAttributeTokens(check.Network)
	logs, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "reflected ORIGINATOR_ID and CLUSTER_LIST",
	}, func(probeCtx context.Context) (string, error) {
		answer, logErr := check.Lab.Logs(probeCtx, peerInject, 4000)
		if logErr != nil {
			return "", logErr
		}
		if !answer.Available {
			return "", errors.New("injector logs were not read")
		}
		return strings.ToUpper(answer.Text), nil
	}, func(output string) bool {
		return strings.Contains(output, originator) && strings.Contains(output, cluster)
	})
	if err != nil {
		return err
	}
	const withdrawal = ":02:0004180A14000000"
	if err := requireNoEarlyReflectorWithdrawal(logs); err != nil {
		return err
	}
	if _, err := check.Lab.Exec(ctx, peerFRR, []string{
		cmdVtysh, "-c", frrConfigureTerminal, "-c", "router bgp 65000",
		"-c", "address-family ipv4 unicast", "-c", "no network 10.20.0.0/24",
	}, nil); err != nil {
		return err
	}
	if err := waitLogContains(ctx, check.Lab, peerInject, 90*time.Second, withdrawal); err != nil {
		return err
	}
	output, err := check.Lab.Query(ctx, peerFRR, session, nil)
	if err != nil {
		return err
	}
	if !strings.Contains(output, "BGP state = Established") {
		return errors.New("FRR session dropped during the reflected withdrawal")
	}
	return nil
}

func reflectorAttributeTokens(network interoplab.Network) (originator, cluster string) {
	octets := network.IPv4.Addr().As4()
	var token textbuf.Buffer
	originator = token.Str("800904").HexUpper(octets[:3]).Str("03").String()
	cluster = token.Reset().Str("800A04").HexUpper(octets[:3]).Str("02").String()
	return originator, cluster
}

func requireNoEarlyReflectorWithdrawal(logs string) error {
	if strings.Contains(strings.ToUpper(logs), ":02:0004180A14000000") {
		return errors.New("withdraw-shaped UPDATE reached the peer before FRR was asked to withdraw")
	}
	return nil
}
func checkWireEditAPIOriginBIRD(ctx context.Context, check *interoplab.CheckContext) error {
	timeout, err := wireEditSessionTimeout(check)
	if err != nil {
		return err
	}
	if err := observerFailure(ctx, check.Lab); err != nil {
		return err
	}
	if err := waitContains(ctx, check.Lab, peerBIRD, []string{cmdBirdc, "show protocols"}, timeout, birdZeProtocol, "Established"); err != nil {
		return err
	}
	var command textbuf.Buffer
	for _, prefix := range []string{"10.55.0.0/24", "10.55.1.0/24"} {
		argv := []string{
			cmdBirdc,
			command.Str("show route for ").Str(prefix).String(),
		}
		if err := waitContains(ctx, check.Lab, peerBIRD, argv, timeout, prefix); err != nil {
			return err
		}
		if err := checkBIRDAPICommunities(ctx, check.Lab, prefix); err != nil {
			return err
		}
		command.Reset()
	}
	if err := observerFailure(ctx, check.Lab); err != nil {
		return err
	}
	logs, err := check.Lab.Logs(ctx, peerBIRD, 2000)
	if err != nil {
		return err
	}
	if !logs.Available {
		return errors.New("BIRD logs were not read, so session stability was not measured")
	}
	const established = "ze_peer: State changed to up"
	if count := strings.Count(logs.Text, established); count != 1 {
		return fmt.Errorf("BIRD session reached up %d times, expected exactly one", count)
	}
	return nil
}

func wireEditSessionTimeout(check *interoplab.CheckContext) (time.Duration, error) {
	data, err := os.ReadFile(filepath.Join(check.Source.Directory, "bird.conf"))
	if err != nil {
		return 0, fmt.Errorf("read BIRD establishment barrier: %w", err)
	}
	match := regexp.MustCompile(`(?m)^\s*connect delay time\s+(\d+)\s*;`).FindSubmatch(data)
	if len(match) != 2 {
		return 0, errors.New("bird.conf states no `connect delay time`; the queue-rail barrier is unmeasured")
	}
	delaySeconds, err := strconv.Atoi(string(match[1]))
	if err != nil {
		return 0, fmt.Errorf("parse BIRD establishment barrier: %w", err)
	}
	timeout := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{}).SessionTimeout
	floor := time.Duration(delaySeconds+10) * time.Second
	if timeout < floor {
		return 0, fmt.Errorf("SESSION_TIMEOUT=%s cannot cover BIRD's %ds connect delay; minimum is %s", timeout, delaySeconds, floor)
	}
	return timeout, nil
}

func observerFailure(ctx context.Context, lab interoplab.CheckerLab) error {
	logs, err := lab.Logs(ctx, "ze", 2000)
	if err != nil {
		return err
	}
	if !logs.Available {
		return errors.New("ze logs were not read, so observer success was not measured")
	}
	for line := range strings.SplitSeq(logs.Text, "\n") {
		if strings.Contains(line, "ZE-OBSERVER-FAIL") {
			return fmt.Errorf("process plugin failed: %s", strings.TrimSpace(line))
		}
	}
	return nil
}

func checkBIRDAPICommunities(ctx context.Context, lab interoplab.CheckerLab, prefix string) error {
	var command textbuf.Buffer
	output, err := lab.Query(
		ctx,
		peerBIRD,
		[]string{
			cmdBirdc,
			command.Str("show route for ").Str(prefix).Str(" all").String(),
		},
		nil,
	)
	if err != nil {
		return err
	}
	if !strings.Contains(output, prefix) {
		return fmt.Errorf("BIRD has no route dump for %s", prefix)
	}
	standard, standardOK := birdAttributeLine(output, "BGP.community:")
	if !standardOK {
		return fmt.Errorf("BIRD route %s has no BGP.community line", prefix)
	}
	if err := requireContains(standard, "(65001,100)", "(65001,200)"); err != nil {
		return fmt.Errorf("BIRD route %s standard communities: %w", prefix, err)
	}
	large, largeOK := birdAttributeLine(output, "BGP.large_community:")
	if !largeOK {
		return fmt.Errorf("BIRD route %s has no BGP.large_community line", prefix)
	}
	if err := requireContains(large, "(65001, 0, 1)"); err != nil {
		return fmt.Errorf("BIRD route %s large community: %w", prefix, err)
	}
	return nil
}

func birdAttributeLine(output, attribute string) (string, bool) {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), attribute) {
			return line, true
		}
	}
	return "", false
}
func checkMaxPrefixPerFamily(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("max-prefix scenario has no selected IPv4 network")
	}
	neighbor := networkHostAddress(check.Network, 2)
	var command textbuf.Buffer
	session := []string{
		cmdVtysh,
		"-c",
		command.Str("show bgp neighbor ").Str(neighbor).String(),
	}
	if err := waitContains(ctx, check.Lab, peerFRR, session, 90*time.Second, "BGP state = Established"); err != nil {
		return err
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    time.Second,
		Description: "ipv4/unicast prefix-limit decision",
	}, func(probeCtx context.Context) (string, error) {
		logs, logErr := check.Lab.Logs(probeCtx, "ze", 200)
		if logErr != nil {
			return "", logErr
		}
		if !logs.Available {
			return "", errors.New("ze logs were not read")
		}
		for line := range strings.SplitSeq(logs.Text, "\n") {
			if strings.Contains(line, "prefix count exceeded maximum") {
				return line, nil
			}
		}
		return "", nil
	}, maxPrefixWarnOnlyDecision); err != nil {
		return err
	}

	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		output, err := check.Lab.Query(ctx, peerFRR, session, nil)
		if err != nil {
			return err
		}
		if !strings.Contains(output, "BGP state = Established") {
			return errors.New("warn-only ipv4/unicast prefix limit tore the session down")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return nil
		case <-ticker.C:
		}
	}
}

func maxPrefixWarnOnlyDecision(line string) bool {
	if strings.ContainsAny(line, "\r\n") {
		return false
	}
	return strings.Contains(line, "prefix count exceeded maximum") &&
		strings.Contains(line, "family=ipv4/unicast") &&
		strings.Contains(line, "teardown=false")
}

func checkHoldtimeDeadPeer(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	if !check.Network.IPv4.IsValid() {
		return errors.New("holdtime scenario has no selected IPv4 network")
	}
	neighbor := networkHostAddress(check.Network, 2)
	var command textbuf.Buffer
	session := []string{
		cmdVtysh,
		"-c",
		command.Str("show bgp neighbor ").Str(neighbor).String(),
	}
	if err := waitContains(
		ctx,
		check.Lab,
		peerFRR,
		session,
		90*time.Second,
		"BGP state = Established",
	); err != nil {
		return err
	}
	if err := check.Lab.Pause(ctx, peerFRR); err != nil {
		return fmt.Errorf("freeze FRR: %w", err)
	}
	thawed := false
	defer func() {
		if thawed {
			return
		}
		thawErr := check.Lab.Unpause(context.WithoutCancel(ctx), peerFRR)
		if thawErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("thaw FRR during cleanup: %w", thawErr)
		}
	}()
	_, report, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: time.Second, Description: "Hold Timer Expired NOTIFICATION"}, func(probeCtx context.Context) (string, error) {
		logs, logErr := check.Lab.Logs(probeCtx, "ze", 400)
		if logErr != nil {
			return "", logErr
		}
		if !logs.Available {
			return "", errors.New("ze logs were not read")
		}
		return logs.Text, nil
	}, holdNotificationSeen)
	if thawErr := check.Lab.Unpause(context.WithoutCancel(ctx), peerFRR); thawErr != nil {
		return fmt.Errorf("thaw FRR: %w", thawErr)
	}
	thawed = true
	if err != nil {
		return err
	}
	if report.Elapsed < 6*time.Second || report.Elapsed >= 13500*time.Millisecond {
		return fmt.Errorf("hold timer expired after %s, expected 6s <= elapsed < 13.5s", report.Elapsed)
	}
	jsonSession := []string{
		cmdVtysh,
		"-c",
		command.Reset().Str("show bgp neighbor ").Str(neighbor).Str(" json").String(),
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "FRR received Hold Timer Expired"}, func(probeCtx context.Context) (map[string]any, error) {
		output, queryErr := check.Lab.Query(probeCtx, peerFRR, jsonSession, nil)
		if queryErr != nil {
			return nil, queryErr
		}
		var document map[string]map[string]any
		if jsonErr := json.Unmarshal([]byte(output), &document); jsonErr != nil {
			return nil, jsonErr
		}
		return document[neighbor], nil
	}, frrReceivedHoldExpiry); err != nil {
		return err
	}
	return waitContains(
		ctx,
		check.Lab,
		peerFRR,
		session,
		90*time.Second,
		"BGP state = Established",
	)
}
func holdNotificationSeen(logs string) bool {
	for line := range strings.SplitSeq(logs, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "notification sent") && strings.Contains(lower, "hold timer expired") {
			return true
		}
	}
	return false
}

func frrReceivedHoldExpiry(peer map[string]any) bool {
	dueTo := strings.ToLower(fmt.Sprint(peer["lastResetDueTo"]))
	reason := strings.ToLower(fmt.Sprint(peer["lastNotificationReason"]))
	return strings.Contains(dueTo, "receive") && strings.Contains(reason, "hold timer expired")
}

type isisLSPRow struct {
	sequence uint64
	holdtime int
	pduLen   int
}

func checkISISOwnLSPPurge(ctx context.Context, check *interoplab.CheckContext) error {
	return checkISISOwnLSPPurgeWith(ctx, check, injectISISPurgeHost)
}

func checkISISOwnLSPPurgeWith(ctx context.Context, check *interoplab.CheckContext, send isisPurgeSender) error {
	if err := waitContains(ctx, check.Lab, peerFRR, []string{cmdVtysh, "-c", frrShowISISNeighbor}, 90*time.Second, "Up"); err != nil {
		return err
	}
	before, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "live Ze LSP before purge"}, func(probeCtx context.Context) (isisLSPRow, error) {
		return queryISISLSP(probeCtx, check.Lab)
	}, func(row isisLSPRow) bool { return row.holdtime > 0 && row.pduLen > isisPurgePDULength })
	if err != nil {
		return err
	}
	if before.sequence >= isisClaimedSequence {
		return fmt.Errorf("ze LSP sequence %d already reached purge claim %d", before.sequence, isisClaimedSequence)
	}
	if err := injectISISOwnLSPPurge(ctx, check.Lab, send); err != nil {
		return err
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "Ze LSP re-originated above purge"}, func(probeCtx context.Context) (isisLSPRow, error) {
		return queryISISLSP(probeCtx, check.Lab)
	}, func(row isisLSPRow) bool {
		return row.sequence > isisClaimedSequence && row.holdtime > 0 && row.pduLen > isisPurgePDULength
	}); err != nil {
		return err
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "Ze in FRR Level-1 topology"}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdVtysh, "-c", "show isis topology level-1"}, nil)
	}, func(output string) bool {
		return strings.Contains(output, "ze-purge") || strings.Contains(output, "0000.0000.0002")
	}); err != nil {
		return err
	}
	stability := time.NewTimer(5 * time.Second)
	defer stability.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-stability.C:
	}
	output, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", frrShowISISNeighbor}, nil)
	if err != nil {
		return err
	}
	if !strings.Contains(output, "Up") {
		return errors.New("the IS-IS adjacency dropped while answering the purge")
	}
	return nil
}

func queryISISLSP(ctx context.Context, lab interoplab.CheckerLab) (isisLSPRow, error) {
	output, err := lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", frrShowISISDatabase}, nil)
	if err != nil {
		return isisLSPRow{}, err
	}
	for line := range strings.SplitSeq(output, "\n") {
		tokens := strings.Fields(line)
		if len(tokens) < 5 || (tokens[0] != "0000.0000.0002.00-00" && tokens[0] != "ze-purge.00-00") {
			continue
		}
		for index, token := range tokens {
			if len(token) != 10 || !strings.HasPrefix(token, "0x") {
				continue
			}
			sequence, parseErr := strconv.ParseUint(token[2:], 16, 32)
			if parseErr != nil || index < 1 || index+2 >= len(tokens) {
				continue
			}
			pduLen, pduErr := strconv.Atoi(tokens[index-1])
			if pduErr != nil {
				continue
			}
			holdToken := tokens[index+2]
			holdtime := 0
			if !strings.HasPrefix(holdToken, "(") {
				holdtime, parseErr = strconv.Atoi(holdToken)
				if parseErr != nil {
					continue
				}
			}
			return isisLSPRow{sequence: sequence, holdtime: holdtime, pduLen: pduLen}, nil
		}
	}
	return isisLSPRow{}, errors.New("FRR database has no parseable Ze LSP row")
}

func checkOSPFLFA(ctx context.Context, check *interoplab.CheckContext) error {
	return checkOSPFFastReroute(ctx, check, false)
}

func checkOSPFTILFA(ctx context.Context, check *interoplab.CheckContext) error {
	return checkOSPFFastReroute(ctx, check, true)
}

func checkOSPFFastReroute(ctx context.Context, check *interoplab.CheckContext, tiLFA bool) (resultErr error) {
	if !check.Network.IPv4.IsValid() {
		return errors.New("LFA scenario has no selected IPv4 network")
	}
	if err := waitContains(ctx, check.Lab, peerFRR, []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, 90*time.Second, ospfStateFull); err != nil {
		return err
	}
	if tiLFA {
		if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: 2 * time.Second, Description: "TI-LFA SR programming"}, func(probeCtx context.Context) (map[string]any, error) {
			command := zeCommand("show ospf segment-routing")
			output, queryErr := check.Lab.Query(probeCtx, "ze", command, queryEnvironment("ze", command))
			if queryErr != nil {
				return nil, queryErr
			}
			var state map[string]any
			if jsonErr := json.Unmarshal([]byte(output), &state); jsonErr != nil {
				return nil, jsonErr
			}
			return state, nil
		}, func(state map[string]any) bool {
			var prefix textbuf.Buffer
			nodePrefix := prefix.Str(networkHostAddress(check.Network, 2)).Str("/32").String()
			return srProgrammedFor(state, nodePrefix)
		}); err != nil {
			return err
		}
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: 2 * time.Second, Description: "pre-computed fast-reroute backup"}, func(probeCtx context.Context) ([]map[string]any, error) {
		command := zeCommand("show ospf route fast-reroute")
		output, queryErr := check.Lab.Query(probeCtx, "ze", command, queryEnvironment("ze", command))
		if queryErr != nil {
			return nil, queryErr
		}
		var rows []map[string]any
		if jsonErr := json.Unmarshal([]byte(output), &rows); jsonErr != nil {
			return nil, jsonErr
		}
		return rows, nil
	}, func(rows []map[string]any) bool {
		return fastRerouteProtected(rows, tiLFA)
	}); err != nil {
		return err
	}
	if _, err := check.Lab.Exec(ctx, peerFRR, []string{"ip", ipObjectLink, ipActionSet, containerInterface, linkDown}, nil); err != nil {
		return err
	}
	restored := false
	defer func() {
		if restored {
			return
		}
		_, restoreErr := check.Lab.Exec(context.WithoutCancel(ctx), peerFRR, []string{"ip", ipObjectLink, ipActionSet, containerInterface, "up"}, nil)
		if restoreErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("restore LFA primary link: %w", restoreErr)
		}
	}()
	failurePropagation := time.NewTimer(2 * time.Second)
	defer failurePropagation.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-failurePropagation.C:
	}
	if err := waitPingSuccess(ctx, check.Lab, "172.30.255.3", 12*time.Second); err != nil {
		return err
	}
	if _, err := check.Lab.Exec(ctx, peerFRR, []string{"ip", ipObjectLink, ipActionSet, containerInterface, "up"}, nil); err != nil {
		return err
	}
	restored = true
	return nil
}
func waitPingSuccess(ctx context.Context, lab interoplab.CheckerLab, address string, timeout time.Duration) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "LFA forwarding"}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, "ze", []string{"ping", "-c", "1", "-W", "1", address}, nil)
	}, func(output string) bool {
		return strings.Contains(output, "1 received") || strings.Contains(output, "1 packets received")
	})
	return err
}

func srProgrammed(state map[string]any) bool {
	return srProgrammedFor(state, "172.30.0.2/32")
}

func srProgrammedFor(state map[string]any, nodePrefix string) bool {
	enabled, _ := state["enabled"].(bool)
	if !enabled {
		return false
	}
	srgbOK := false
	for _, row := range anyRows(state["srgb"]) {
		if row["lower-bound"] == float64(16000) && row["upper-bound"] == float64(23999) {
			srgbOK = true
		}
	}
	sidOK := false
	for _, row := range anyRows(state["prefix-sids"]) {
		if row["prefix"] == nodePrefix && row["index"] == float64(200) {
			sidOK = true
		}
	}
	return srgbOK && sidOK
}

func fastRerouteProtected(rows []map[string]any, validateRepairLabels bool) bool {
	for _, row := range rows {
		if row["prefix"] != "172.30.255.3/32" {
			continue
		}
		for _, hop := range anyRows(row["next-hops"]) {
			protected, _ := hop["protected"].(bool)
			backup, _ := hop["backup"].(string)
			if !protected || backup == "" {
				continue
			}
			if validateRepairLabels {
				for _, label := range anyValues(hop["repair-labels"]) {
					number, ok := label.(float64)
					if !ok || number < 0 || number > 0xfffff {
						return false
					}
				}
			}
			return true
		}
	}
	return false
}

func anyRows(value any) []map[string]any {
	values, _ := value.([]any)
	rows := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if row, ok := value.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func anyValues(value any) []any {
	values, _ := value.([]any)
	return values
}

func checkAddPathRailAgreement(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("ADD-PATH rail scenario has no selected IPv4 network")
	}
	speaker1 := networkHostAddress(check.Network, 10)
	speaker2 := networkHostAddress(check.Network, 11)
	if err := waitZePeerState(ctx, check.Lab, speaker1, peerStateEstablished, 60*time.Second); err != nil {
		return err
	}
	zeAddress := networkHostAddress(check.Network, 2)
	if err := waitContainsFold(ctx, check.Lab, peerGoBGP, []string{cmdGoBGP, gobgpNeighbor, zeAddress}, 90*time.Second, peerStateEstablished); err != nil {
		return err
	}
	if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, peerPrefixFirst, "-a", gobgpFamilyIPv4,
		gobgpNextHop, networkHostAddress(check.Network, 5),
	}, nil); err != nil {
		return err
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: time.Second, Description: "stored ADD-PATH route"}, func(probeCtx context.Context) (int, error) {
		return zeRIBCount(probeCtx, check.Lab)
	}, func(count int) bool { return count >= 1 }); err != nil {
		return err
	}
	state, err := zePeerState(ctx, check.Lab, speaker2)
	if err != nil {
		return err
	}
	if state == peerStateEstablished {
		return errors.New("speaker2 joined before the route was stored, so the replay rail was not exercised")
	}
	live, err := waitSpeakerUpdate(ctx, check.Lab, peerSpeaker, 150*time.Second)
	if err != nil {
		return err
	}
	replayed, err := waitSpeakerUpdate(ctx, check.Lab, peerSpeaker2, 150*time.Second)
	if err != nil {
		return err
	}
	if !bytes.Equal(live, replayed) {
		return fmt.Errorf("live and replay UPDATE bodies differ: live=%s replay=%s", hex.EncodeToString(live), hex.EncodeToString(replayed))
	}
	return nil
}

func waitZePeerState(ctx context.Context, lab interoplab.CheckerLab, address, want string, timeout time.Duration) error {
	var description textbuf.Buffer
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: description.Str("ze peer ").Str(address).String(),
	}, func(probeCtx context.Context) (string, error) {
		return zePeerState(probeCtx, lab, address)
	}, func(state string) bool { return state == want })
	return err
}

func zePeerState(ctx context.Context, lab interoplab.CheckerLab, address string) (string, error) {
	command := zeCommand("show bgp peer list")
	output, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return "", err
	}
	var document struct {
		Peers map[string]struct {
			State string `json:"state"`
		} `json:"peers"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return "", fmt.Errorf("show bgp peer list did not answer JSON: %w", err)
	}
	peer, ok := document.Peers[address]
	if !ok {
		return "absent", nil
	}
	if peer.State == "" {
		return "", fmt.Errorf("show bgp peer list has no state for %s", address)
	}
	return peer.State, nil
}

func waitSpeakerUpdate(ctx context.Context, lab interoplab.CheckerLab, peer string, timeout time.Duration) ([]byte, error) {
	var description textbuf.Buffer
	output, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    3 * time.Second,
		Description: description.Str(peer).Str(" UPDATE verdict").String(),
	}, func(probeCtx context.Context) (string, error) {
		logs, logErr := lab.Logs(probeCtx, peer, 200)
		if logErr != nil {
			return "", logErr
		}
		if !logs.Available {
			return "", errors.New("speaker logs were not read")
		}
		return logs.Text, nil
	}, func(logs string) bool {
		return strings.Contains(logs, "result:")
	})
	if err != nil {
		return nil, err
	}
	return speakerRouteUpdate(output, peer)
}

func speakerRouteUpdate(logs, peer string) ([]byte, error) {
	if established, ok := logField(logs, fieldEstablished); !ok || established != logValueYes {
		return nil, fmt.Errorf("%s never reached Established with Ze", peer)
	}
	updates := speakerUpdateHexes(logs)
	if len(updates) != 1 {
		return nil, fmt.Errorf("%s logged %d route-bearing UPDATEs, expected exactly one", peer, len(updates))
	}
	body, err := hex.DecodeString(updates[0])
	if err != nil {
		return nil, fmt.Errorf("%s UPDATE is not hexadecimal: %w", peer, err)
	}
	nlri, err := updateNLRI(body)
	if err != nil {
		return nil, fmt.Errorf("%s UPDATE framing: %w", peer, err)
	}
	prefix := []byte{24, 10, 99, 0}
	if len(nlri) != 8 || !bytes.Equal(nlri[4:], prefix) {
		return nil, fmt.Errorf("%s NLRI %s is not four ADD-PATH identifier octets followed by %s", peer, hex.EncodeToString(nlri), hex.EncodeToString(prefix))
	}
	_ = binary.BigEndian.Uint32(nlri[:4])
	return body, nil
}

func updateNLRI(body []byte) ([]byte, error) {
	if len(body) < 4 {
		return nil, errors.New("body is shorter than the UPDATE length fields")
	}
	withdrawn := int(binary.BigEndian.Uint16(body[:2]))
	attributesOffset := 2 + withdrawn
	if attributesOffset+2 > len(body) {
		return nil, errors.New("withdrawn-routes length exceeds the UPDATE body")
	}
	attributes := int(binary.BigEndian.Uint16(body[attributesOffset : attributesOffset+2]))
	nlriOffset := attributesOffset + 2 + attributes
	if nlriOffset > len(body) {
		return nil, errors.New("path-attributes length exceeds the UPDATE body")
	}
	return body[nlriOffset:], nil
}

func speakerUpdateHexes(logs string) []string {
	const token = "note: update-hex:" // #nosec G101 -- protocol log marker, not an authentication credential.
	var updates []string
	for line := range strings.SplitSeq(logs, "\n") {
		_, update, found := strings.Cut(line, token)
		if found {
			updates = append(updates, strings.TrimSpace(update))
		}
	}
	return updates
}

func checkShowRIBUnderFRRLoad(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("load scenario has no selected IPv4 network")
	}
	neighbor := networkHostAddress(check.Network, 2)
	var command textbuf.Buffer
	session := []string{
		cmdVtysh,
		"-c",
		command.Str("show bgp neighbor ").Str(neighbor).String(),
	}
	if err := waitContains(
		ctx,
		check.Lab,
		peerFRR,
		session,
		90*time.Second,
		"BGP state = Established",
	); err != nil {
		return err
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: time.Second, Description: "ze RIB reaching 256 routes"}, func(probeCtx context.Context) (int, error) {
		return zeRIBCount(probeCtx, check.Lab)
	}, func(count int) bool { return count >= ribLoadRoutes }); err != nil {
		return err
	}
	count, err := zeRIBDocumentCount(ctx, check.Lab)
	if err != nil {
		return err
	}
	if count < ribLoadRoutes {
		return fmt.Errorf("show bgp rib dumped %d routes, expected at least %d", count, ribLoadRoutes)
	}
	rows, err := zeRIBBestRows(ctx, check.Lab)
	if err != nil {
		return err
	}
	if rows != 50 {
		return fmt.Errorf("show bgp rib best first 50 carried %d rows, expected 50", rows)
	}

	counts, walks, err := runRIBLoad(ctx, check.Lab)
	if err != nil {
		return err
	}
	if walks == 0 {
		return errors.New("no RIB walk answered while FRR changed the table")
	}
	if len(counts) < 2 {
		return fmt.Errorf("every RIB count walk saw the same route total %v", counts)
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: time.Second, Description: "ze RIB returning to 256 routes"}, func(probeCtx context.Context) (int, error) {
		return zeRIBCount(probeCtx, check.Lab)
	}, func(count int) bool { return count >= ribLoadRoutes }); err != nil {
		return err
	}
	return waitContains(
		ctx,
		check.Lab,
		peerFRR,
		session,
		90*time.Second,
		"BGP state = Established",
	)
}

func runRIBLoad(ctx context.Context, lab interoplab.CheckerLab) (map[int]struct{}, uint64, error) {
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	timer := time.NewTimer(ribLoadWindow)
	defer timer.Stop()

	errorsFound := make(chan error, ribLoadWalkers+1)
	var group sync.WaitGroup
	var countsMu sync.Mutex
	counts := make(map[int]struct{})
	var walks atomic.Uint64
	var stop atomic.Bool

	// Every worker is bounded by the timer and workerCtx. This function MUST
	// stop and wait for all workers before it returns, so no checker query
	// outlives its scenario.
	group.Go(func() {
		enabled := false
		for !stop.Load() && workerCtx.Err() == nil {
			if err := setFRRRedistribution(workerCtx, lab, enabled); err != nil {
				recordLoadError(errorsFound, err)
				cancel()
				return
			}
			enabled = !enabled
		}
	})
	for walker := range ribLoadWalkers {
		document := walker == 0
		group.Go(func() {
			for !stop.Load() && workerCtx.Err() == nil {
				if document {
					if _, err := zeRIBDocumentCount(workerCtx, lab); err != nil {
						recordLoadError(errorsFound, err)
						cancel()
						return
					}
					walks.Add(1)
					continue
				}
				count, err := zeRIBCount(workerCtx, lab)
				if err != nil {
					recordLoadError(errorsFound, err)
					cancel()
					return
				}
				countsMu.Lock()
				counts[count] = struct{}{}
				countsMu.Unlock()
				walks.Add(1)
			}
		})
	}

	var workerErr error
	select {
	case <-timer.C:
		stop.Store(true)
		cancel()
	case workerErr = <-errorsFound:
		cancel()
	case <-ctx.Done():
		workerErr = ctx.Err()
		cancel()
	}
	group.Wait()
	if workerErr != nil {
		return nil, walks.Load(), workerErr
	}
	if err := setFRRRedistribution(ctx, lab, true); err != nil {
		return nil, walks.Load(), err
	}
	return counts, walks.Load(), nil
}

func recordLoadError(destination chan<- error, err error) {
	select {
	case destination <- err:
	default:
	}
}

func setFRRRedistribution(ctx context.Context, lab interoplab.CheckerLab, enabled bool) error {
	verb := "no redistribute static"
	if enabled {
		verb = "redistribute static"
	}
	_, err := lab.Exec(ctx, peerFRR, []string{
		cmdVtysh, "-c", frrConfigureTerminal, "-c", "router bgp 65002",
		"-c", "address-family ipv4 unicast", "-c", verb,
	}, nil)
	return err
}

func zeRIBCount(ctx context.Context, lab interoplab.CheckerLab) (int, error) {
	output, err := lab.Query(ctx, "ze", zeCommand("show bgp rib count"), queryEnvironment("ze", zeCommand("show bgp rib count")))
	if err != nil {
		return 0, err
	}
	var answer struct {
		Count json.Number `json:"count"`
	}
	if err := json.Unmarshal([]byte(output), &answer); err != nil {
		return 0, fmt.Errorf("show bgp rib count did not answer JSON: %w", err)
	}
	if answer.Count == "" {
		return 0, errors.New("show bgp rib count answered without a count field")
	}
	count, err := strconv.Atoi(answer.Count.String())
	if err != nil {
		return 0, fmt.Errorf("show bgp rib count is not an integer: %w", err)
	}
	return count, nil
}

func zeRIBDocumentCount(ctx context.Context, lab interoplab.CheckerLab) (int, error) {
	command := zeCommand("show bgp rib")
	output, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return 0, err
	}
	var document struct {
		RIB map[string][]json.RawMessage `json:"adj-rib-in"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return 0, fmt.Errorf("show bgp rib did not answer a document: %w", err)
	}
	count := 0
	for _, routes := range document.RIB {
		count += len(routes)
	}
	return count, nil
}

func zeRIBBestRows(ctx context.Context, lab interoplab.CheckerLab) (int, error) {
	command := zeCommand("show bgp rib best first 50")
	output, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return 0, err
	}
	var rows []struct {
		Prefix string `json:"prefix"`
	}
	if err := json.Unmarshal([]byte(output), &rows); err != nil {
		return 0, fmt.Errorf("show bgp rib best did not answer rows: %w", err)
	}
	for _, row := range rows {
		if row.Prefix == "" {
			return 0, errors.New("show bgp rib best answered a row without a prefix")
		}
	}
	return len(rows), nil
}
