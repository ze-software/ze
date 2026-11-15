// Design: docs/architecture/testing/interop.md -- RFC 6608 wire and FRR parsed evidence.
// Related: speaker_fsm_error.go -- a relay of captured native frames, not a synthetic peer.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func checkFSMErrorFRR(ctx context.Context, check *interoplab.CheckContext, test fsmRelayCase) (resultErr error) {
	directory, err := retainedProofDirectory(check)
	if err != nil {
		return err
	}
	evidence := map[string]string{}
	defer func() {
		diagnostic, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if resultErr != nil {
			log, logErr := check.Lab.Query(diagnostic, peerFRR, []string{cmdCat, frrLogPath}, nil)
			retainFSMQuery(evidence, "frr-parsed-log", log, logErr)
		}
		version, versionErr := check.Lab.Query(diagnostic, peerFRR, []string{cmdVtysh, "-c", frrShowVersion}, nil)
		evidence["frr-version"] = version
		image, imageErr := check.Lab.PeerImage(diagnostic, peerFRR)
		evidence["frr-image"] = image
		resultErr = errors.Join(resultErr, versionErr, imageErr)
		verdict := verdictPassed
		if resultErr != nil {
			verdict = resultErr.Error()
		}
		data, marshalErr := json.MarshalIndent(map[string]any{evidenceKeyScenario: test.scenario, evidenceKeyVerdict: verdict, evidenceKeyEvidence: evidence}, "", "  ")
		resultErr = errors.Join(resultErr, marshalErr)
		if marshalErr == nil {
			resultErr = errors.Join(resultErr, os.WriteFile(filepath.Join(directory, "proof.json"), data, 0o600))
		}
		if resultErr != nil {
			resultErr = fmt.Errorf("%w (retained proof: %s)", resultErr, directory)
		}
	}()
	var report fsmRelayReport
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "RFC 6608 original-connection capture boundary"}, func(probe context.Context) (string, error) {
		text, queryErr := check.Lab.Query(probe, peerSpeaker, []string{cmdCat, fsmCaptureBase + "-result.json"}, nil)
		evidence["relay-report"] = text
		return text, queryErr
	}, func(text string) bool { return json.Unmarshal([]byte(text), &report) == nil && report.Finished })
	if err != nil {
		return err
	}
	if err := judgeFSMRelay(report, test); err != nil {
		return err
	}
	for endpoint, want := range map[string]string{report.ZeEndpoint: networkHostAddress(check.Network, 2), report.NativeEndpoint: networkHostAddress(check.Network, 3)} {
		host, port, err := net.SplitHostPort(endpoint)
		if err != nil {
			return err
		}
		if host != want || port != "179" {
			return errors.New("FSM capture names another connection endpoint")
		}
	}
	neighbor := networkHostAddress(check.Network, 10)
	command := []string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}
	if test.subcode == 0 {
		if err := waitJSONFields(ctx, check.Lab, peerFRR, command, 30*time.Second, map[string]string{frrSessionStateKey: stateEstablished}, nil); err != nil {
			return err
		}
		if err := waitFRRRoute(ctx, check.Lab, fsmWitnessPrefix, frrIPv4Unicast, 30*time.Second, true); err != nil {
			return err
		}
		neighborText, err := check.Lab.Query(ctx, peerFRR, command, nil)
		evidence["frr-neighbor"] = neighborText
		if err != nil {
			return err
		}
		routeText, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + fsmWitnessPrefix + " json"}, nil)
		evidence["frr-control-route"] = routeText
		if err != nil {
			return err
		}
		// Read the relay last: its original connections must remain healthy
		// throughout the native neighbor and decoded-route queries above.
		text, err := check.Lab.Query(ctx, peerSpeaker, []string{cmdCat, fsmCaptureBase + "-result.json"}, nil)
		evidence["relay-report-final"] = text
		if err != nil {
			return err
		}
		var final fsmRelayReport
		if err := json.Unmarshal([]byte(text), &final); err != nil {
			return err
		}
		return judgeFSMControlSnapshot(neighborText, routeText, neighbor, report, final, test)
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: time.Second, Description: "FRR parsed RFC 6608 NOTIFICATION and Data"}, func(probe context.Context) (string, error) {
		text, queryErr := check.Lab.Query(probe, peerFRR, command, nil)
		retainFSMQuery(evidence, "frr-neighbor", text, queryErr)
		log, logErr := check.Lab.Query(probe, peerFRR, []string{cmdCat, frrLogPath}, nil)
		retainFSMQuery(evidence, "frr-parsed-log", log, logErr)
		if test.subcode == 1 {
			// FRR has not received a KEEPALIVE in this case, so its
			// accepted peer has not transferred to the configured peer.
			// The parsed receive log belongs to the original connection;
			// configured-neighbor reset fields do not describe it.
			return log, logErr
		}
		return text, errors.Join(queryErr, logErr)
	}, func(text string) bool {
		if test.subcode == 1 {
			return judgeFRRFSMNotification("", text, neighbor, test) == nil
		}
		return judgeFRRFSMNotification(text, evidence["frr-parsed-log"], neighbor, test) == nil
	})
	return err
}

// retainFSMQuery keeps the last successful observation when a later probe is
// canceled. Errors are evidence too, but cannot erase a native parser's output.
func retainFSMQuery(evidence map[string]string, key, text string, err error) {
	if err != nil {
		evidence[key+"-query-error"] = err.Error()
		return
	}
	if text != "" {
		evidence[key] = text
	}
}
