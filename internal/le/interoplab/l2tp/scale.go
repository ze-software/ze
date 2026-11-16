// Design: docs/labs/l2tp-interop.md -- real 3 x 16 subscriber scale proof.
// Related: scale_observe.go -- kernel, FRR, accounting and traffic collection.
// Related: scale_window.go -- independently decoded control trace and live window.
package l2tp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const scaleCaptureLines = 16384

// checkScaleSessions deliberately retains the dependency red for observers
// keyed only by SID. A working kernel PPP route cannot satisfy the independent
// FRR assertion, nor can one session's traffic satisfy another session's probe.
func checkScaleSessions(ctx context.Context, lab labOperations, timeout time.Duration) error {
	fail := func(assertion int, cause error) error {
		return scaleFailureDiagnostics(ctx, lab, fmt.Errorf("scenario %s assertion %d: %w", scenarioScale, assertion, cause))
	}
	if timeout <= 0 {
		return fail(1, errors.New("scale proof timeout must be positive"))
	}
	if err := waitFRRSession(ctx, lab, zeAddress, timeout); err != nil {
		return fail(1, err)
	}
	for lac := 1; lac <= scaleLACCount; lac++ {
		// The plan MUST keep the peer idle until this detached launch. The
		// checker MUST release the measured real ACK hold before starting
		// another configured tunnel; Suite cleanup owns every child process.
		if err := startScalePeer(ctx, lab, lac); err != nil {
			return fail(1, fmt.Errorf("launch LAC%d: %w", lac, err))
		}
		if err := exerciseScaleWindow(ctx, lab, lac, timeout); err != nil {
			return fail(2, fmt.Errorf("LAC%d: %w", lac, err))
		}
		if _, err := waitScalePopulation(ctx, lab, scaleUsers(lac), timeout); err != nil {
			return fail(3, err)
		}
	}
	baseline, err := waitScalePopulation(ctx, lab, scaleUsers(scaleLACCount), timeout)
	if err != nil {
		return fail(3, err)
	}
	if err := requireScaleTopology(baseline.population); err != nil {
		return fail(3, err)
	}
	if err := waitScalePeers(ctx, lab, baseline.population, timeout); err != nil {
		return fail(3, err)
	}
	if err := prepareScalePeers(ctx, lab, baseline.population); err != nil {
		return fail(3, err)
	}
	if err := requireScaleTraffic(ctx, lab, baseline, true); err != nil {
		return fail(3, err)
	}
	for lac := 1; lac <= scaleLACCount; lac++ {
		wire, err := scaleWireLog(ctx, lab, lac)
		if err != nil {
			return fail(4, err)
		}
		if err := writeScaleWireLog(lac, wire); err != nil {
			return fail(4, err)
		}
		if err := requireScaleWireSessions(wire, baseline.population); err != nil {
			return fail(4, interoplab.WithSearchedOutput(err, wire))
		}
	}
	starts, err := waitScaleAccounting(ctx, lab, baseline.population, "Start", timeout)
	if err != nil {
		return fail(4, err)
	}
	if err := waitScaleFRR(ctx, lab, baseline.population, baseline.population, timeout); err != nil {
		return fail(4, err)
	}
	first, second, err := scaleCollisionTargets(baseline.population)
	if err != nil {
		return fail(5, err)
	}
	if err := checkScaleReadableSelection(ctx, lab, baseline, starts, first, second); err != nil {
		return fail(5, err)
	}
	if err := scaleDisconnect(ctx, lab, starts.byUser[first]); err != nil {
		return fail(5, err)
	}
	afterFirst, err := observeScaleRemoval(ctx, lab, baseline, baseline.population, starts, []string{first}, first, timeout)
	if err != nil {
		return fail(5, err)
	}
	if err := scaleStopPeerPPP(ctx, lab, second); err != nil {
		return fail(6, err)
	}
	if _, err := observeScaleRemoval(ctx, lab, afterFirst, baseline.population, starts, []string{first, second}, second, timeout); err != nil {
		return fail(6, err)
	}
	if err := waitScaleCDN(ctx, lab, baseline.population.byUser[second], timeout); err != nil {
		return fail(6, err)
	}
	for lac := 1; lac <= scaleLACCount; lac++ {
		wire, err := scaleWireLog(ctx, lab, lac)
		if err != nil {
			return fail(6, err)
		}
		if err := writeScaleWireLog(lac, wire); err != nil {
			return fail(6, err)
		}
	}
	return nil
}

func scaleWireLog(ctx context.Context, lab labOperations, lac int) (string, error) {
	peer := "scale-relay-" + strconv.Itoa(lac)
	logs, err := lab.Logs(ctx, peer, scaleCaptureLines)
	if err != nil {
		return "", err
	}
	if !logs.Available {
		return "", fmt.Errorf("%s capture unavailable", peer)
	}
	return logs.Text, nil
}

// writeScaleWireLog retains the full evidence at a completed check boundary,
// rather than repeating a growing capture from every polling read.
func writeScaleWireLog(lac int, wire string) error {
	_, err := fmt.Fprintf(os.Stdout, "L2TP-SCALE-WIRE scale-relay-%d %s\n", lac, wire)
	return err
}

func exerciseScaleWindow(ctx context.Context, lab labOperations, lac int, timeout time.Duration) (resultErr error) {
	var latestWire string
	defer func() { resultErr = errors.Join(resultErr, writeScaleWireLog(lac, latestWire)) }()
	held, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: 10 * time.Millisecond, Description: "five processed ICRQs behind real CWND4 ACK hold"}, func(probe context.Context) (scaleWindowEvidence, error) {
		wire, err := scaleWireLog(probe, lab, lac)
		if err != nil {
			return scaleWindowEvidence{}, err
		}
		latestWire = wire
		window, err := readScaleWindow(wire)
		if err != nil {
			return scaleWindowEvidence{}, interoplab.WithSearchedOutput(err, wire)
		}
		if window.released {
			return scaleWindowEvidence{}, errors.New("window released before the checker measured saturation")
		}
		raw, err := lnsPolicyCommand(probe, lab, "show l2tp reliable "+strconv.Itoa(int(window.tunnel)))
		if err != nil {
			return scaleWindowEvidence{}, err
		}
		if err := requireScaleReliable(raw, window); err != nil {
			return scaleWindowEvidence{}, interoplab.WithSearchedOutput(err, string(raw))
		}
		return window, nil
	}, func(scaleWindowEvidence) bool { return true })
	if err != nil {
		return err
	}
	if _, err := lab.Exec(ctx, "scale-relay-"+strconv.Itoa(lac), []string{commandTouch, "/run/scale-release"}, nil); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: 10 * time.Millisecond, Description: "marker releases genuine cumulative ACK and queued fifth response"}, func(probe context.Context) (scaleWindowEvidence, error) {
		wire, err := scaleWireLog(probe, lab, lac)
		if err != nil {
			return scaleWindowEvidence{}, err
		}
		latestWire = wire
		window, err := readScaleWindow(wire)
		if err != nil {
			return scaleWindowEvidence{}, interoplab.WithSearchedOutput(err, wire)
		}
		if window.tunnel != held.tunnel {
			return scaleWindowEvidence{}, errors.New("release changed tunnel identity")
		}
		return window, nil
	}, func(window scaleWindowEvidence) bool { return window.released })
	return err
}

func scaleCollisionTargets(population scalePopulation) (string, string, error) {
	var first string
	for _, user := range scaleUsers(scaleLACCount) {
		session, exists := population.byUser[user]
		if !exists {
			return "", "", fmt.Errorf("collision baseline lacks %s", user)
		}
		if session.local != 1 {
			continue
		}
		if first == "" {
			first = user
			continue
		}
		if session.tunnel != population.byUser[first].tunnel {
			return first, user, nil
		}
	}
	return "", "", errors.New("no equal Ze-local SID on distinct tunnel IDs")
}

func observeScaleRemoval(ctx context.Context, lab labOperations, before scaleObservation, original scalePopulation, starts scaleAccounting, removed []string, target string, timeout time.Duration) (scaleObservation, error) {
	users := make([]string, 0, len(original.byUser)-len(removed))
	stopped := scalePopulation{byUser: make(map[string]scaleSession, len(removed))}
	for _, user := range scaleUsers(scaleLACCount) {
		if slices.Contains(removed, user) {
			stopped.byUser[user] = original.byUser[user]
		} else {
			users = append(users, user)
		}
	}
	after, err := waitScalePopulation(ctx, lab, users, timeout)
	if err != nil {
		return scaleObservation{}, err
	}
	if err := requireScaleRemoval(before.population, after.population, target); err != nil {
		return scaleObservation{}, err
	}
	for user, link := range after.links {
		if before.links[user] != link {
			return scaleObservation{}, fmt.Errorf("survivor %s changed Ze interface %s to %s", user, before.links[user], link)
		}
	}
	stops, err := waitScaleAccounting(ctx, lab, stopped, "Stop", timeout)
	if err != nil {
		return scaleObservation{}, err
	}
	for user, id := range stops.byUser {
		if starts.byUser[user] != id {
			return scaleObservation{}, fmt.Errorf("Stop for %s changed full accounting ID from %s to %s", user, starts.byUser[user], id)
		}
	}
	if err := waitScaleFRR(ctx, lab, original, after.population, timeout); err != nil {
		return scaleObservation{}, err
	}
	if err := waitScalePeers(ctx, lab, after.population, timeout); err != nil {
		return scaleObservation{}, err
	}
	if err := requireScaleTraffic(ctx, lab, after, false); err != nil {
		return scaleObservation{}, err
	}
	return after, nil
}

func waitScaleCDN(ctx context.Context, lab labOperations, target scaleSession, timeout time.Duration) (resultErr error) {
	lac, _, err := scaleUserLocation(target.username)
	if err != nil {
		return err
	}
	var latestWire string
	defer func() { resultErr = errors.Join(resultErr, writeScaleWireLog(lac, latestWire)) }()
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: 100 * time.Millisecond, Description: "peer CDN for the selected session tuple"}, func(probe context.Context) (bool, error) {
		wire, err := scaleWireLog(probe, lab, lac)
		if err != nil {
			return false, err
		}
		latestWire = wire
		if err := requireScalePeerCDN(wire, target); err != nil {
			return false, interoplab.WithSearchedOutput(err, wire)
		}
		return true, nil
	}, func(found bool) bool { return found })
	return err
}
