// Design: docs/architecture/testing/interop.md -- foreign receiver LLGR lifecycle.
// Related: speaker_llgr.go -- fixed code-64 omission and code-71 timers.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	llgrIPv4Prefix        = "198.51.94.0/24"
	llgrIPv6Prefix        = "2001:db8:94::/48"
	llgrZeroPrefix        = "198.51.95.0/24"
	llgrRouteCommunity    = "65004:94"
	llgrRestartTime       = 20 * time.Second
	llgrStaleTime         = 40 * time.Second
	llgrObservationMargin = 3 * time.Second
)

// checkLLGRIndependentFRR preserves every original GR/EOR assertion, then
// observes received-route retention, community changes and expiry in FRR.
// Source loss is TCP-only. FRR's session identity is fenced throughout, so its
// own graceful-restart retention or session-reset loss cannot satisfy the test.
// RFC 9494 Section 4.2: "The interval for which they are retained is limited by
// the sum of the Restart Time in the received Graceful Restart Capability and
// the Long-Lived Stale Time in the received Long-Lived Graceful Restart Capability".
func checkLLGRIndependentFRR(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("LLGR scenario has no selected IPv4 network")
	}
	if !check.Network.IPv6.IsValid() {
		return errors.New("LLGR scenario has no selected IPv6 network")
	}
	baseline := llgrBaselineOperations()
	for index := range baseline {
		if err := runOperation(ctx, check.Network, check.Lab, &baseline[index]); err != nil {
			return fmt.Errorf("original GR baseline assertion %d: %w", index+1, err)
		}
	}
	if err := llgrReceivedFence(ctx, check, 3); err != nil {
		return fmt.Errorf("original FRR-to-Ze received route: %w", err)
	}
	identity, err := llgrFRRSession(ctx, check)
	if err != nil {
		return err
	}
	text, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + networkHostAddress(check.Network, 2)}, nil)
	if err != nil {
		return err
	}
	if err := llgrFRRFamilies(text); err != nil {
		return err
	}
	if err := llgrInitialFence(ctx, check, identity); err != nil {
		return err
	}
	// RFC 9494 Section 4.2: zero GR period and zero LLST retain nothing.
	// Both sources MUST close only their transport after the receipt fences;
	// USR1 leaves PID 1 and the advertised next-hop interfaces alive.
	if err := check.Lab.Signal(ctx, peerSpeaker2, "USR1"); err != nil {
		return err
	}
	if _, err := llgrWaitDown(ctx, check, 11); err != nil {
		return err
	}
	if err := llgrWaitRoute(ctx, check, llgrZeroPrefix, frrIPv4Unicast, "65001 65005", "65005:94", false, false, 5*time.Second); err != nil {
		return fmt.Errorf("no-LLST control was retained: %w", err)
	}
	if err := llgrRequireSession(ctx, check, identity); err != nil {
		return err
	}
	// Recheck the main source after the negative control, immediately before
	// inducing the one loss whose original timer boundaries are measured.
	if err := llgrReceivedFence(ctx, check, 10); err != nil {
		return err
	}
	if err := llgrWaitRoute(ctx, check, llgrIPv4Prefix, frrIPv4Unicast, zeInjectorASPath, llgrRouteCommunity, true, false, 5*time.Second); err != nil {
		return err
	}
	if err := llgrWaitRoute(ctx, check, llgrIPv6Prefix, "ipv6 unicast", zeInjectorASPath, llgrRouteCommunity, true, false, 5*time.Second); err != nil {
		return err
	}
	loss := time.Now()
	if err := check.Lab.Signal(ctx, peerSpeaker, "USR1"); err != nil {
		return err
	}
	down, err := llgrWaitDown(ctx, check, 10)
	if err != nil {
		return err
	}
	if down.Sub(loss) > llgrObservationMargin {
		return fmt.Errorf("source DOWN fence took %s; cannot establish timer boundaries", down.Sub(loss))
	}
	// RFC 9494 Section 4.2: the omitted family enters LLGR immediately.
	if err := llgrWaitRoute(ctx, check, llgrIPv4Prefix, frrIPv4Unicast, zeInjectorASPath, llgrRouteCommunity, true, true, llgrObservationMargin); err != nil {
		return fmt.Errorf("omitted GR family did not enter immediate LLGR: %w", err)
	}
	return llgrObserveExpiry(ctx, check, identity, loss, down)
}

// llgrBaselineOperations retains the original scenario and extra assertions.
// endOfRibRecv distinguishes a real received GR family tuple from code 64 with
// no tuples, which still makes FRR report gracefulRestart as negotiated.
func llgrBaselineOperations() []operation {
	return []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: injectPrefixFirst},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand(zeShowBGPRIBStatus), minimum: map[string]int{fieldRoutesIn: 1}},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"gracefulRestart", frrCapabilityNegotiated}},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON}, contains: []string{"endOfRibRecv"}, timeout: 60 * time.Second},
	}
}

// llgrFRRIdentity uses FRR's exact session counters. The displayed establishment
// epoch is recomputed from separately truncated wall/monotonic seconds and can
// move without a reset; it is not part of the session identity.
type llgrFRRIdentity struct {
	Established uint64
	Dropped     uint64
}

func llgrFRRSession(ctx context.Context, check *interoplab.CheckContext) (llgrFRRIdentity, error) {
	address := networkHostAddress(check.Network, 2)
	output, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + address + " json"}, nil)
	if err != nil {
		return llgrFRRIdentity{}, err
	}
	return llgrParseFRRSession(output, address)
}

func llgrParseFRRSession(output, address string) (llgrFRRIdentity, error) {
	var document map[string]struct {
		State        string  `json:"bgpState"`
		Established  *uint64 `json:"connectionsEstablished"`
		Dropped      *uint64 `json:"connectionsDropped"`
		Capabilities struct {
			LLGR string `json:"longLivedGracefulRestart"`
		} `json:"neighborCapabilities"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return llgrFRRIdentity{}, err
	}
	peer, ok := document[address]
	if !ok {
		return llgrFRRIdentity{}, errors.New("FRR neighbor JSON omits the selected Ze address")
	}
	if peer.State != stateEstablished {
		return llgrFRRIdentity{}, fmt.Errorf("FRR session state is %q", peer.State)
	}
	if peer.Established == nil {
		return llgrFRRIdentity{}, errors.New("FRR neighbor JSON omits the established counter")
	}
	if peer.Dropped == nil {
		return llgrFRRIdentity{}, errors.New("FRR neighbor JSON omits the dropped counter")
	}
	if *peer.Established == 0 {
		return llgrFRRIdentity{}, errors.New("FRR has never established the session")
	}
	if peer.Capabilities.LLGR != "advertisedAndReceived" {
		return llgrFRRIdentity{}, errors.New("FRR did not negotiate LLGR in both directions")
	}
	return llgrFRRIdentity{*peer.Established, *peer.Dropped}, nil
}

func llgrRequireSession(ctx context.Context, check *interoplab.CheckContext, initial llgrFRRIdentity) error {
	current, err := llgrFRRSession(ctx, check)
	if err != nil {
		return err
	}
	if current != initial {
		return fmt.Errorf("FRR session identity changed: initial=%+v current=%+v", initial, current)
	}
	return nil
}

// llgrFRRFamilies reads only FRR 10.3.1's LLGR text capability section.
// Its similarly named JSON family map tests ENHE flags, not LLGR flags.
func llgrFRRFamilies(output string) error {
	inLLGR, inFamilies := false, false
	ipv4, ipv6 := false, false
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "    Long-lived Graceful Restart:") {
			if inLLGR {
				return errors.New("FRR neighbor output repeats its LLGR section")
			}
			if strings.TrimSpace(line) != "Long-lived Graceful Restart: advertised and received" {
				return errors.New("FRR text output does not confirm bilateral LLGR")
			}
			inLLGR = true
			continue
		}
		if !inLLGR {
			continue
		}
		// The next capability or peer section is at indentation four or less.
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "     ") {
			break
		}
		if line == "      Address families by peer:" {
			inFamilies = true
			continue
		}
		if !inFamilies {
			continue
		}
		if line == "           IPv4 Unicast" {
			ipv4 = true
		}
		if line == "           IPv6 Unicast" {
			ipv6 = true
		}
	}
	if !ipv4 {
		return errors.New("FRR LLGR text section has no received IPv4 Unicast tuple")
	}
	if !ipv6 {
		return errors.New("FRR LLGR text section has no received IPv6 Unicast tuple")
	}
	return nil
}

func llgrInitialFence(ctx context.Context, check *interoplab.CheckContext, identity llgrFRRIdentity) error {
	for _, host := range []uint8{10, 11} {
		if err := waitZePeerState(ctx, check.Lab, networkHostAddress(check.Network, host), 60*time.Second); err != nil {
			return err
		}
		if err := llgrReceivedFence(ctx, check, host); err != nil {
			return err
		}
	}
	for _, route := range []struct{ prefix, family, path, community string }{
		{llgrIPv4Prefix, frrIPv4Unicast, zeInjectorASPath, llgrRouteCommunity},
		{llgrIPv6Prefix, "ipv6 unicast", zeInjectorASPath, llgrRouteCommunity},
		{llgrZeroPrefix, frrIPv4Unicast, "65001 65005", "65005:94"},
	} {
		if err := llgrWaitRoute(ctx, check, route.prefix, route.family, route.path, route.community, true, false, 60*time.Second); err != nil {
			return err
		}
	}
	return llgrRequireSession(ctx, check, identity)
}

func llgrReceivedFence(ctx context.Context, check *interoplab.CheckContext, host uint8) error {
	address := networkHostAddress(check.Network, host)
	want := []llgrReceivedExpectation{{llgrZeroPrefix, zeIPv4Unicast, address}}
	if host == 10 {
		want = []llgrReceivedExpectation{
			{llgrIPv4Prefix, zeIPv4Unicast, address},
			{llgrIPv6Prefix, "ipv6/unicast", networkHostAddress6(check.Network, host)},
		}
	}
	if host == 3 {
		want = []llgrReceivedExpectation{{"10.20.0.0/24", zeIPv4Unicast, address}}
	}
	command := zeCommand("show bgp rib received")
	var lastCompleted interoplab.CommandResult
	var lastCompletedErr error
	var haveCompleted bool
	var lastMeasured string
	_, report, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: time.Second, Description: "received LLGR source routes"}, func(probe context.Context) (bool, error) {
		// Query discards stdout on command failure. Retain the actual result
		// here, including a refused CLI answer, before the final deadline
		// probe can replace the error that explains the missing observation.
		result, probeErr := check.Lab.Exec(probe, "ze", command, queryEnvironment("ze", command))
		ready := false
		if probeErr == nil {
			if strings.TrimSpace(result.Stdout) == "" {
				probeErr = errors.New("peer ze query returned no output")
			} else {
				ready, probeErr = llgrReceivedRoutesPresent(result.Stdout, address, want)
			}
		}
		if probe.Err() == nil {
			lastCompleted, lastCompletedErr, haveCompleted = result, probeErr, true
		}
		if probeErr == nil {
			lastMeasured = result.Stdout
		}
		return ready, probeErr
	}, func(ready bool) bool { return ready })
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w; attempts=%d transient-failures=%d; last completed probe (available=%t): exit=%d stdout=%q stderr=%q error=%v; last measured RIB=%q",
		err, report.Attempts, report.TransientFailures, haveCompleted, lastCompleted.ExitCode,
		lastCompleted.Stdout, lastCompleted.Stderr, lastCompletedErr, lastMeasured)
}

type llgrReceivedExpectation struct {
	prefix, family, nextHop string
}

func llgrReceivedRoutesPresent(output, peer string, want []llgrReceivedExpectation) (bool, error) {
	var routes []struct {
		Peer      string `json:"peer"`
		Direction string `json:"direction"`
		Prefix    string `json:"prefix"`
		Family    string `json:"family"`
		NextHop   string `json:"next-hop"`
	}
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return false, err
	}
	if routes == nil {
		return false, errors.New("ze received RIB query omitted routes")
	}
	for _, expected := range want {
		found := false
		for _, route := range routes {
			if route.Peer != peer {
				continue
			}
			if route.Direction != rpc.DirectionReceived.String() {
				continue
			}
			if route.Prefix != expected.prefix {
				continue
			}
			if route.Family != expected.family {
				continue
			}
			if route.NextHop != expected.nextHop {
				return false, fmt.Errorf("ze received %s with next hop %s, want %s", route.Prefix, route.NextHop, expected.nextHop)
			}
			found = true
			break
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

func llgrWaitDown(ctx context.Context, check *interoplab.CheckContext, host uint8) (time.Time, error) {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: llgrObservationMargin, Interval: 100 * time.Millisecond, Description: "LLGR source TCP DOWN"}, func(probe context.Context) (string, error) {
		return zePeerState(probe, check.Lab, networkHostAddress(check.Network, host))
	}, func(state string) bool { return state != "absent" && state != peerStateEstablished })
	return time.Now(), err
}

// llgrRouteState reads FRR's detailed per-prefix schema from route_vty_out_detail
// (FRR 10.3.1 bgp_route.c). Only an actual empty object is an absence answer;
// null, unrelated documents and missing path attributes fail closed.
func llgrRouteState(output, prefix, peer, path, marker string) (present, stale bool, err error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return false, false, err
	}
	if envelope == nil {
		return false, false, errors.New("FRR route query returned null")
	}
	if len(envelope) == 0 {
		return false, false, nil
	}
	var document struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Valid bool `json:"valid"`
			Peer  struct {
				ID string `json:"peerId"`
			} `json:"peer"`
			ASPath struct {
				Value string `json:"string"`
			} `json:"aspath"`
			Community struct {
				List []string `json:"list"`
			} `json:"community"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return false, false, err
	}
	if document.Prefix != prefix {
		return false, false, fmt.Errorf("FRR answered prefix %q, want %s", document.Prefix, prefix)
	}
	if len(document.Paths) != 1 {
		return false, false, fmt.Errorf("FRR has %d paths for %s, want exactly one", len(document.Paths), prefix)
	}
	route := document.Paths[0]
	if !route.Valid {
		return false, false, errors.New("FRR received an invalid LLGR route")
	}
	if route.Peer.ID != peer {
		return false, false, fmt.Errorf("FRR route peer %q differs from %s", route.Peer.ID, peer)
	}
	if route.ASPath.Value != path {
		return false, false, fmt.Errorf("FRR AS path %q differs from %s", route.ASPath.Value, path)
	}
	if !slices.Contains(route.Community.List, marker) {
		return false, false, errors.New("FRR route has no source marker community")
	}
	return true, slices.Contains(route.Community.List, "llgrStale"), nil
}

func llgrQueryRoute(ctx context.Context, check *interoplab.CheckContext, prefix, family, path, marker string) (bool, bool, error) {
	output, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp " + family + " " + prefix + " json"}, nil)
	if err != nil {
		return false, false, err
	}
	present, stale, err := llgrRouteState(output, prefix, networkHostAddress(check.Network, 2), path, marker)
	if err != nil {
		return present, stale, fmt.Errorf("FRR route %s: %w; response=%q", prefix, err, output)
	}
	return present, stale, nil
}

func llgrWaitRoute(ctx context.Context, check *interoplab.CheckContext, prefix, family, path, marker string, present, stale bool, timeout time.Duration) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: 200 * time.Millisecond, Description: "FRR LLGR route " + prefix}, func(probe context.Context) (bool, error) {
		gotPresent, gotStale, err := llgrQueryRoute(probe, check, prefix, family, path, marker)
		return gotPresent == present && gotStale == stale, err
	}, func(matches bool) bool { return matches })
	return err
}

// llgrObserveExpiry samples continuously, with explicit uncertainty only around
// each original DOWN boundary. Presence is mandatory until loss+timer-margin;
// absence is mandatory after observed-DOWN+timer+margin. The gap is bounded by
// the measured DOWN fence, never restarted at a later family transition.
func llgrObserveExpiry(ctx context.Context, check *interoplab.CheckContext, identity llgrFRRIdentity, loss, down time.Time) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	deadline := down.Add(llgrRestartTime + llgrStaleTime + llgrObservationMargin)
	lastSample := down
	sawBothStale, sawSurvivingControl := false, false
	for {
		if err := llgrSampleCurrent(lastSample); err != nil {
			return err
		}
		lastSample = time.Now()
		bothStale, onlyControl := true, true
		if err := llgrRequireSession(ctx, check, identity); err != nil {
			return err
		}
		if err := llgrSampleCurrent(lastSample); err != nil {
			return err
		}
		for _, route := range []struct {
			prefix, family, path, marker string
			enter, expire                time.Duration
		}{
			{llgrIPv4Prefix, frrIPv4Unicast, zeInjectorASPath, llgrRouteCommunity, 0, llgrStaleTime},
			{llgrIPv6Prefix, "ipv6 unicast", zeInjectorASPath, llgrRouteCommunity, llgrRestartTime, llgrRestartTime + llgrStaleTime},
			{llgrZeroPrefix, frrIPv4Unicast, "65001 65005", "65005:94", 0, 0},
		} {
			started := time.Now()
			present, stale, err := llgrQueryRoute(ctx, check, route.prefix, route.family, route.path, route.marker)
			if err != nil {
				return err
			}
			finished := time.Now()
			if finished.Sub(started) > llgrObservationMargin {
				return errors.New("FRR route query exceeded timer observation margin")
			}
			if route.prefix != llgrZeroPrefix {
				bothStale = bothStale && present && stale
				if route.prefix == llgrIPv4Prefix {
					onlyControl = onlyControl && !present
				} else {
					onlyControl = onlyControl && present && stale
				}
			}
			if err := llgrCheckWindow(started, finished, loss, down, route.enter, route.expire, present, stale); err != nil {
				return fmt.Errorf("FRR %s at loss+%s: %w", route.prefix, finished.Sub(loss), err)
			}
		}
		now := time.Now()
		if now.Before(loss.Add(llgrStaleTime - llgrObservationMargin)) {
			sawBothStale = sawBothStale || bothStale
		}
		if now.After(down.Add(llgrStaleTime + llgrObservationMargin)) {
			if now.Before(loss.Add(llgrRestartTime + llgrStaleTime - llgrObservationMargin)) {
				sawSurvivingControl = sawSurvivingControl || onlyControl
			}
		}
		// A positive baseline route and the same live FRR session prove each
		// empty per-prefix answer came from a functioning receiver, not teardown.
		if err := waitFRRRoute(ctx, check.Lab, injectPrefixFirst, frrIPv4Unicast, time.Second, true); err != nil {
			return err
		}
		if err := llgrSampleCurrent(lastSample); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			if !sawBothStale {
				return errors.New("never observed both families LLGR-stale before the original IPv4 expiry")
			}
			if !sawSurvivingControl {
				return errors.New("never observed IPv6 survive the original IPv4 expiry")
			}
			// All observations MUST occur after the final boundary, not merely
			// finish on its far side while their queries started before it.
			if err := llgrWaitRoute(ctx, check, llgrIPv6Prefix, "ipv6 unicast", zeInjectorASPath, llgrRouteCommunity, false, false, time.Second); err != nil {
				return err
			}
			if err := llgrRequireSession(ctx, check, identity); err != nil {
				return err
			}
			return llgrSampleCurrent(lastSample)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func llgrSampleCurrent(started time.Time) error {
	if time.Since(started) > 5*time.Second {
		return errors.New("LLGR polling missed a timer observation interval")
	}
	return nil
}

func llgrCheckWindow(started, finished, loss, down time.Time, enter, expire time.Duration, present, stale bool) error {
	if expire == 0 {
		if present {
			return errors.New("zero-LLST source route reappeared")
		}
		return nil
	}
	if finished.Before(loss.Add(expire - llgrObservationMargin)) {
		if !present {
			return errors.New("retained route disappeared before its original expiry")
		}
	}
	if started.After(down.Add(expire + llgrObservationMargin)) {
		if present {
			return errors.New("route survived its original expiry")
		}
		return nil
	}
	if !present {
		return nil // Only the explicitly bounded expiry window admits absence.
	}
	if finished.Before(loss.Add(enter - llgrObservationMargin)) {
		if stale {
			return errors.New("conventional-GR control entered LLGR early")
		}
	}
	if started.After(down.Add(enter + llgrObservationMargin)) {
		if !stale {
			return errors.New("retained route lacks LLGR_STALE after LLGR entry")
		}
	}
	return nil
}
