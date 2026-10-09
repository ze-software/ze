// Design: docs/architecture/testing/interop.md -- native FRR recipient proof.
// Related: check_parsed_empty_mp_wire.go -- independent complete-history oracle.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// checkParsedEmptyMPFRR seeds an owned route only after both Ze peers and FRR
// are ready, then releases mixed withdrawal and genuine EOR in separate epochs.
// Each epoch ends at an independently installed FRR route and a complete wire
// fence. Initial-sync EORs remain outside the negative assertion.
func checkParsedEmptyMPFRR(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return err
	}
	directory, err := os.MkdirTemp(paths.Scratch, check.Source.Name+"-")
	if err != nil {
		return err
	}
	defer func() {
		evidence, captureErr := parsedEmptyMPDiagnostics(ctx, check)
		resultErr = errors.Join(resultErr, captureErr)
		for _, name := range []string{"ze.conf", "frr.conf", "speaker-args", "speaker2-args"} {
			content, readErr := os.ReadFile(filepath.Join(check.Source.Directory, name))
			resultErr = errors.Join(resultErr, readErr)
			if readErr == nil {
				evidence["fixture-"+name] = string(content)
			}
		}
		verdict := "passed"
		if resultErr != nil {
			verdict = resultErr.Error()
		}
		data, marshalErr := json.MarshalIndent(map[string]any{
			"scenario": check.Source.Name, "network": check.Network, "verdict": verdict,
			"evidence": evidence,
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
		return errors.New("parsed-empty-MP scenario has no selected network")
	}
	neighbor := networkHostAddress(check.Network, 11)
	readyCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := waitContains(readyCtx, check.Lab, peerFRR,
		[]string{cmdVtysh, "-c", "show bgp neighbor " + neighbor}, 60*time.Second, "BGP state = Established"); err != nil {
		return err
	}
	for _, host := range []uint8{10, 11} {
		if err := waitZePeerState(readyCtx, check.Lab, networkHostAddress(check.Network, host), 60*time.Second); err != nil {
			return err
		}
	}
	if err := waitContains(readyCtx, check.Lab, peerSpeaker,
		[]string{cmdCat, parsedEmptyMPSourceReady}, 60*time.Second, "established"); err != nil {
		return err
	}
	cancel()
	identity, err := parsedEmptyMPRecipientIdentity(ctx, check)
	if err != nil {
		return err
	}
	for phase := 1; phase <= 3; phase++ {
		if err := check.Lab.Signal(ctx, peerSpeaker, "USR1"); err != nil {
			return err
		}
		prefix := "198.51." + strconv.Itoa(100+phase) + ".0/24"
		if err := parsedEmptyMPWaitRoute(ctx, check, prefix); err != nil {
			return fmt.Errorf("phase %d recipient fence: %w", phase, err)
		}
		if phase == 1 {
			if err := parsedEmptyMPWaitRoute(ctx, check, "198.51.100.0/24"); err != nil {
				return fmt.Errorf("seed was not installed before withdrawal: %w", err)
			}
		}
		if phase >= 2 {
			_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
				Timeout: 30 * time.Second, Interval: time.Second, Description: "FRR removes previously seeded route",
			}, func(probe context.Context) (bool, error) {
				output, err := check.Lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast json"}, nil)
				if err != nil {
					return false, err
				}
				if err := requireRouteWithheld(output, "198.51.100.0/24", prefix); err != nil {
					return false, err
				}
				return true, nil
			}, func(done bool) bool { return done })
			if err != nil {
				return err
			}
		}
		for _, source := range []bool{true, false} {
			if err := parsedEmptyMPWaitHistory(ctx, check, source, phase); err != nil {
				return fmt.Errorf("phase %d source=%v: %w", phase, source, err)
			}
		}
		current, err := parsedEmptyMPRecipientIdentity(ctx, check)
		if err != nil {
			return err
		}
		if current != identity {
			return fmt.Errorf("FRR daemon or session changed: before=%+v after=%+v", identity, current)
		}
		for _, host := range []uint8{10, 11} {
			if err := waitZePeerState(ctx, check.Lab, networkHostAddress(check.Network, host), 10*time.Second); err != nil {
				return err
			}
		}
	}
	return nil
}

// Zero is not a valid identity: the native PID and session decoder admit both.
type parsedEmptyMPIdentity struct {
	pid     uint64
	session aigpRecipientFence
}

func parsedEmptyMPRecipientIdentity(ctx context.Context, check *interoplab.CheckContext) (parsedEmptyMPIdentity, error) {
	pidText, err := check.Lab.Query(ctx, peerFRR, []string{cmdCat, "/var/run/frr/bgpd.pid"}, nil)
	if err != nil {
		return parsedEmptyMPIdentity{}, err
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(pidText), 10, 64)
	if err != nil {
		return parsedEmptyMPIdentity{}, fmt.Errorf("FRR bgpd PID: %w", err)
	}
	if pid == 0 {
		return parsedEmptyMPIdentity{}, errors.New("FRR bgpd PID is zero")
	}
	neighbor := networkHostAddress(check.Network, 11)
	output, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}, nil)
	if err != nil {
		return parsedEmptyMPIdentity{}, err
	}
	fence, err := readFRRAIGPRecipientFence(output, neighbor)
	if err != nil {
		return parsedEmptyMPIdentity{}, err
	}
	return parsedEmptyMPIdentity{pid: pid, session: fence}, nil
}

func parsedEmptyMPWaitHistory(ctx context.Context, check *interoplab.CheckContext, source bool, phase int) error {
	peer, path := peerSpeaker2, extendedCaptureBase+"-ze.jsonl"
	if source {
		peer, path = peerSpeaker, parsedEmptyMPSourceCapture
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "complete parsed-empty-MP wire epoch",
	}, func(probe context.Context) (bool, error) {
		text, err := check.Lab.Query(probe, peer, []string{cmdCat, path}, nil)
		if err != nil {
			return false, err
		}
		bodies, err := parsedEmptyMPCapture(text, source)
		if err != nil {
			return false, err
		}
		// RFC 4724 Section 2 and RFC 7606 Section 5.1: preserve control meaning.
		if err := parsedEmptyMPHistory(bodies, source, phase); err != nil {
			return false, err
		}
		return true, nil
	}, func(done bool) bool { return done })
	return err
}

func parsedEmptyMPWaitRoute(ctx context.Context, check *interoplab.CheckContext, prefix string) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "FRR installs source-owned fence route",
	}, func(probe context.Context) (bool, error) {
		output, err := check.Lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + prefix + " json"}, nil)
		if err != nil {
			return false, err
		}
		var route struct {
			Prefix string `json:"prefix"`
			Paths  []struct {
				Valid bool `json:"valid"`
				Peer  struct {
					ID string `json:"peerId"`
				} `json:"peer"`
				ASPath struct {
					String string `json:"string"`
				} `json:"aspath"`
			} `json:"paths"`
		}
		if err := json.Unmarshal([]byte(output), &route); err != nil {
			return false, err
		}
		if route.Prefix != prefix {
			return false, errors.New("FRR does not hold the requested prefix")
		}
		if len(route.Paths) != 1 {
			return false, errors.New("FRR must hold exactly one source-owned path")
		}
		path := &route.Paths[0]
		if !path.Valid {
			return false, errors.New("FRR path is invalid")
		}
		if path.Peer.ID != networkHostAddress(check.Network, 11) {
			return false, errors.New("FRR route came from another session")
		}
		if path.ASPath.String != "65004" {
			return false, errors.New("FRR did not decode the downgraded two-octet AS_PATH")
		}
		return true, nil
	}, func(done bool) bool { return done })
	return err
}

// parsedEmptyMPDiagnostics retains the complete wire histories before teardown,
// including on cancellation. Missing required captures cannot produce a green proof.
func parsedEmptyMPDiagnostics(ctx context.Context, check *interoplab.CheckContext) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	evidence := make(map[string]string)
	var captureErr error
	for _, file := range []struct{ peer, path string }{
		{peerSpeaker, parsedEmptyMPSourceCapture},
		{peerSpeaker2, extendedCaptureBase + "-ze.jsonl"},
		{peerSpeaker2, extendedCaptureBase + "-frr.jsonl"},
		{peerFRR, "/var/run/frr/bgpd.pid"},
	} {
		text, err := check.Lab.Query(ctx, file.peer, []string{cmdCat, file.path}, nil)
		key := file.peer + " " + file.path
		evidence[key] = text
		if err != nil {
			evidence[key+"-query-error"] = err.Error()
			captureErr = errors.Join(captureErr, err)
		}
	}
	for _, query := range []string{"show version", "show bgp ipv4 unicast json"} {
		text, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", query}, nil)
		evidence[query] = text
		if err != nil {
			evidence[query+"-query-error"] = err.Error()
			captureErr = errors.Join(captureErr, err)
		}
	}
	text, logErr := check.Lab.Query(ctx, peerFRR, []string{cmdCat, "/tmp/frr.log"}, nil)
	evidence["frr-protocol-log"] = text
	if logErr != nil {
		evidence["frr-protocol-log-error"] = logErr.Error()
	}
	logs, err := check.Lab.Logs(ctx, peerFRR, 300)
	evidence["frr-log"] = logs.Text
	if err != nil {
		evidence["frr-log-error"] = err.Error()
	}
	return evidence, captureErr
}
