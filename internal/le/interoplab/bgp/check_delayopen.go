// Design: docs/architecture/testing/interop.md -- native DelayOpen wire proof.
// Related: check_delayopen_wire.go -- complete-history timing oracle.
package bgp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	delayOpenScenario    = "bgp-delayopen-frr"
	delayOpenCapture     = "/tmp/bgp-delayopen.pcap"
	delayOpenCaptureLog  = "/tmp/bgp-delayopen-capture.log"
	delayOpenCaptureExit = "/tmp/bgp-delayopen-capture.exit"
)

var delayOpenCases = [...]struct {
	peer              string
	host              uint8
	zeDelay, frrDelay time.Duration
	early             bool
}{
	{peerFRR, 3, 5 * time.Second, 30 * time.Second, false},
	{peerFRRSink, 15, 20 * time.Second, time.Second, true},
}

// checkDelayOpenFRR never relays a socket or edits a BGP payload. Both speakers
// start administratively held; native controls release them only after tcpdump
// reports its activated packet socket. The capture remains live until BOTH
// speakers report Established for BOTH peers. Every failure retains observations.
func checkDelayOpenFRR(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	directory, err := retainedProofDirectory(check)
	if err != nil {
		return err
	}
	evidence := make(map[string]string)
	var history []*delayOpenConnection
	captureStarted := false
	defer func() {
		diagnostic, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if captureStarted {
			resultErr = errors.Join(resultErr, stopDelayOpenCapture(diagnostic, check.Lab))
			text, readErr := check.Lab.Query(diagnostic, "ze", []string{cmdCat, delayOpenCaptureLog}, nil)
			evidence["capture-log"] = text
			captured, captureErr := delayOpenCaptureComplete(text)
			resultErr = errors.Join(resultErr, readErr, captureErr)
			encoded, readErr := check.Lab.Query(diagnostic, "ze", []string{"base64", delayOpenCapture}, nil)
			resultErr = errors.Join(resultErr, readErr)
			if readErr == nil {
				raw, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
				resultErr = errors.Join(resultErr, decodeErr)
				if decodeErr == nil {
					resultErr = errors.Join(resultErr, os.WriteFile(filepath.Join(directory, "wire.pcap"), raw, 0o600))
					var parseErr error
					history, parseErr = delayOpenHistory(raw, netip.MustParseAddr(networkHostAddress(check.Network, 2)))
					resultErr = errors.Join(resultErr, parseErr)
					packets := 0
					for _, connection := range history {
						packets += connection.Packets
					}
					if parseErr == nil && packets != captured {
						captureErr = fmt.Errorf("incomplete DelayOpen history: pcap holds %d packets, tcpdump captured %d", packets, captured)
						resultErr = errors.Join(resultErr, captureErr)
					}
					// Timing judgements require a completed, lossless capture, not
					// a successful Established poll with absent wire evidence.
					if parseErr == nil && captureErr == nil {
						for _, test := range delayOpenCases {
							observation, timingErr := judgeDelayOpen(history, netip.MustParseAddr(networkHostAddress(check.Network, test.host)), test.zeDelay, test.frrDelay, test.early)
							evidence[test.peer+" timing"] = observation
							resultErr = errors.Join(resultErr, timingErr)
						}
					}
				}
			}
		}
		for _, query := range []struct {
			peer, key string
			command   []string
		}{
			{"ze", "rendered-ze.conf", []string{cmdCat, zeMountedConfig}},
			{"ze", "ze-binary-sha256", []string{"sha256sum", "/usr/local/bin/ze"}},
			{"ze", "tcp-sockets", []string{"ss", "-tnp"}},
			{peerFRR, "rendered-frr.conf", []string{cmdCat, "/etc/frr/frr.conf"}},
			{peerFRRSink, "rendered-frr-sink.conf", []string{cmdCat, "/etc/frr/frr.conf"}},
			{peerFRR, "frr-version", []string{cmdVtysh, "-c", frrShowVersion}},
			{peerFRRSink, "frr-sink-version", []string{cmdVtysh, "-c", frrShowVersion}},
			{peerFRR, "frr-bgpd-pid", []string{cmdCat, frrBGPDPIDFile}},
			{peerFRRSink, "frr-sink-bgpd-pid", []string{cmdCat, frrBGPDPIDFile}},
		} {
			text, queryErr := check.Lab.Query(diagnostic, query.peer, query.command, nil)
			evidence[query.key] = text
			if queryErr != nil {
				evidence[query.key+" error"] = queryErr.Error()
			}
			resultErr = errors.Join(resultErr, queryErr)
		}
		for _, peer := range []string{"ze", peerFRR, peerFRRSink} {
			logs, logErr := check.Lab.Logs(diagnostic, peer, 200)
			evidence[peer+" logs"] = logs.Text
			resultErr = errors.Join(resultErr, logErr)
		}
		for _, name := range []string{zeConfigFile, frrConfigFile, frrSinkConfigFile} {
			data, readErr := os.ReadFile(filepath.Join(check.Source.Directory, name)) //nolint:gosec // fixed fixture names inside the selected scenario
			evidence["fixture-"+name] = string(data)
			resultErr = errors.Join(resultErr, readErr)
		}
		verdict := verdictPassed
		if resultErr != nil {
			verdict = resultErr.Error()
		}
		data, marshalErr := json.MarshalIndent(map[string]any{
			"scenario": check.Source.Name, evidenceKeyNetwork: check.Network, "verdict": verdict,
			"frr-image":   interoplab.FRRImage,
			"connections": history, evidenceKeyEvidence: evidence,
		}, "", "  ")
		resultErr = errors.Join(resultErr, marshalErr)
		if marshalErr == nil {
			resultErr = errors.Join(resultErr, os.WriteFile(filepath.Join(directory, "proof.json"), data, 0o600))
		}
		if resultErr != nil {
			resultErr = fmt.Errorf("%w (retained proof: %s)", resultErr, directory)
		}
	}()
	if !check.Network.IPv4.IsValid() {
		return errors.New("DelayOpen scenario has no selected IPv4 network")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	zeAddress := networkHostAddress(check.Network, 2)
	for _, test := range delayOpenCases {
		address := networkHostAddress(check.Network, test.host)
		_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: time.Second, Description: "Ze peer administratively held before capture"}, func(probe context.Context) (string, error) {
			return zePeerState(probe, check.Lab, address)
		}, func(state string) bool { return state == "idle-hold" })
		if err != nil {
			return err
		}
		required := []string{"neighbor " + zeAddress + " shutdown", fmt.Sprintf("neighbor %s timers delayopen %d", zeAddress, int(test.frrDelay/time.Second))}
		config, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: time.Second, Description: test.peer + " native DelayOpen configuration loaded"}, func(probe context.Context) (string, error) {
			return check.Lab.Query(probe, test.peer, []string{cmdVtysh, "-c", "show running-config"}, nil)
		}, func(config string) bool {
			return strings.Contains(config, required[0]) && strings.Contains(config, required[1])
		})
		evidence[test.peer+" initial running-config"] = config
		if err != nil {
			return err
		}
	}
	// Retain failed activation as well: a detached process may have started
	// even when its readiness marker was never observed.
	captureStarted = true
	if err := startDelayOpenCapture(ctx, check.Lab); err != nil {
		return err
	}
	for _, test := range delayOpenCases {
		address := networkHostAddress(check.Network, test.host)
		command := zeCommand("request peer " + address + " enable")
		answer, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
		evidence[test.peer+" Ze enable"] = answer
		if err != nil {
			return err
		}
		release, err := check.Lab.Exec(ctx, test.peer, []string{cmdVtysh, "-c", "configure terminal", "-c", frrRouterBGP65002, "-c", "no neighbor " + zeAddress + " shutdown"}, nil)
		evidence[test.peer+" FRR enable"] = fmt.Sprintf("exit=%d stdout=%q stderr=%q", release.ExitCode, release.Stdout, release.Stderr)
		if err != nil {
			return err
		}
		if release.ExitCode != 0 {
			return fmt.Errorf("%s administrative release failed: %s", test.peer, evidence[test.peer+" FRR enable"])
		}
	}
	for _, test := range delayOpenCases {
		address := networkHostAddress(check.Network, test.host)
		var answer string
		_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: test.peer + " native Established generation"}, func(probe context.Context) (uint64, error) {
			var queryErr error
			answer, queryErr = check.Lab.Query(probe, test.peer, []string{cmdVtysh, "-c", "show bgp neighbor " + zeAddress + " json"}, nil)
			if queryErr != nil {
				return 0, queryErr
			}
			return parseFRRSessionGeneration(answer, zeAddress)
		}, func(generation uint64) bool { return generation == 1 })
		evidence[test.peer+" native neighbor"] = answer
		if err != nil {
			return interoplab.WithLastAnswer(err, answer)
		}
		if err := waitZePeerState(ctx, check.Lab, address, 30*time.Second); err != nil {
			return err
		}
	}
	command := zeCommand("show bgp peer list")
	answer, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	evidence["Ze native peers"] = answer
	return err
}

// startDelayOpenCapture MUST be paired with stopDelayOpenCapture even after a
// scenario assertion fails. timeout bounds the detached process if its caller
// disappears; packet and snapshot limits bound the file independently.
func startDelayOpenCapture(ctx context.Context, lab interoplab.CheckerLab) error {
	script := "timeout -s INT 120 tcpdump -n -U -s 65535 -B 4096 -c 4096 -i eth0 -w " + delayOpenCapture + " 'ip and tcp port 179' 2>" + delayOpenCaptureLog + "; echo $? >" + delayOpenCaptureExit
	if err := lab.ExecDetached(ctx, "ze", []string{"sh", "-c", script}, nil); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 10 * time.Second, Interval: 100 * time.Millisecond, Description: "native DelayOpen capture socket activated"}, func(probe context.Context) (string, error) {
		return lab.Query(probe, "ze", []string{cmdCat, delayOpenCaptureLog}, nil)
	}, func(log string) bool { return strings.Contains(log, "listening on eth0") })
	return err
}

// stopDelayOpenCapture MUST complete before reading wire.pcap; a log without
// the final captured/drop counters cannot support any negative timing claim.
func stopDelayOpenCapture(ctx context.Context, lab interoplab.CheckerLab) error {
	signal, err := lab.Exec(ctx, "ze", []string{"pkill", "-INT", "-x", "tcpdump"}, nil)
	if err != nil {
		return err
	}
	if signal.ExitCode != 0 {
		return fmt.Errorf("DelayOpen capture signal failed: exit=%d stdout=%q stderr=%q", signal.ExitCode, signal.Stdout, signal.Stderr)
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 5 * time.Second, Interval: 100 * time.Millisecond, Description: "DelayOpen capture flushed"}, func(probe context.Context) (string, error) {
		return lab.Query(probe, "ze", []string{cmdCat, delayOpenCaptureExit}, nil)
	}, func(code string) bool { return strings.TrimSpace(code) == "0" })
	return err
}
