// Design: docs/architecture/testing/interop.md -- received-route RFC 9252 proof.
// Related: check_srv6_identity_wire.go -- exact bytes, distinct route/control/fence.
// Receiver: https://github.com/FRRouting/frr/releases/tag/frr-10.7.1
// Parser: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_attr.c
// bgp_attr_srv6_service and bgp_attr_psid_sub consume the remaining parent
// lengths after known nested data. FRR 10.3.1 lacks those cursor advances.
// The scenario pins the released image; it never strips unknown bytes for FRR.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// checkSRv6Identity proves forwarding, not ordinary SDK origination. The SDK
// originates only a readiness token toward the raw source, after all sessions
// and initial EORs. That source owns the three IPv6 announcements. FRR accepts
// all three before either selective-byte verdict is evaluated, so a daemon
// panic, rejected route or dead session cannot masquerade as selective removal.
// MUTATION: run the same harness with an old daemon image. Explicit A-to-A
// loses Service bytes and/or policy-only A-to-B retains them. Both byte verdicts
// are collected, and neither route/session/control requirement is weakened.
func checkSRv6Identity(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
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
	evidence := make(map[string]string)
	defer func() {
		// Teardown belongs to the lab and follows this return. Collect failures
		// before it, under a separate bounded deadline even after cancellation.
		diagnosticCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		for _, peer := range []string{"ze", peerInject, peerFRR, peerFRRSink, peerSpeaker, peerSpeaker2} {
			logs, logErr := check.Lab.Logs(diagnosticCtx, peer, 300)
			evidence[peer+"-log"] = logs.Text
			if logErr != nil {
				evidence[peer+"-log-error"] = logErr.Error()
			}
		}
		for _, peer := range []string{peerSpeaker, peerSpeaker2} {
			for _, suffix := range []string{"-ze.jsonl", "-frr.jsonl", "-result.json"} {
				output, queryErr := check.Lab.Query(diagnosticCtx, peer, []string{"cat", "/tmp/srv6-identity" + suffix}, nil)
				evidence[peer+suffix] = output
				if queryErr != nil {
					// The result file appears only when the bounded transport exits.
					evidence[peer+suffix+"-query-error"] = queryErr.Error()
				}
			}
		}
		command := zeCommand(srv6IdentityState)
		output, queryErr := check.Lab.Query(diagnosticCtx, "ze", command, queryEnvironment("ze", command))
		evidence["policy-final"] = output
		if queryErr != nil {
			evidence["policy-final-error"] = queryErr.Error()
		}
		for _, peer := range []string{peerFRR, peerFRRSink} {
			for _, query := range []string{"show version", "show ipv6 route json"} {
				output, queryErr := check.Lab.Query(diagnosticCtx, peer, []string{cmdVtysh, "-c", query}, nil)
				evidence[peer+"-"+query] = output
				if queryErr != nil {
					evidence[peer+"-"+query+"-error"] = queryErr.Error()
				}
			}
		}
		for _, name := range []string{"ze.conf", "frr.conf", "frr-sink.conf", "frr-image", "daemons", "inject.msg", "inject-args", "speaker-args", "speaker2-args"} {
			content, readErr := os.ReadFile(filepath.Join(check.Source.Directory, name))
			if readErr != nil {
				resultErr = errors.Join(resultErr, readErr)
				continue
			}
			evidence["fixture-"+name] = string(content)
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
			resultErr = fmt.Errorf("SRv6 identity proof retained at %s: %w", directory, resultErr)
		}
	}()

	readiness, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for _, host := range []uint8{9, 10, 11} {
		address := networkHostAddress(check.Network, host)
		if err := waitZePeerState(readiness, check.Lab, address, 60*time.Second); err != nil {
			return err
		}
		command := zeCommand("show bgp peer " + address + " detail")
		last, _, err := interoplab.Wait(readiness, interoplab.WaitOptions{
			Timeout: 60 * time.Second, Interval: time.Second, Description: "SRv6 initial EOR for " + address,
		}, func(probe context.Context) (string, error) {
			return check.Lab.Query(probe, "ze", command, queryEnvironment("ze", command))
		}, func(output string) bool { return requireSRv6IdentityZeSession(output, address) == nil })
		evidence["initial-ze-"+address] = last
		if err != nil {
			return withLastOutput(err, last)
		}
	}
	var initial [2]aigpRecipientFence
	for index, peer := range []string{peerFRR, peerFRRSink} {
		address := networkHostAddress(check.Network, uint8(10+index))
		last, _, err := interoplab.Wait(readiness, interoplab.WaitOptions{
			Timeout: 60 * time.Second, Interval: time.Second, Description: "SRv6 independent receiver " + peer,
		}, func(probe context.Context) (string, error) {
			return check.Lab.Query(probe, peer, []string{cmdVtysh, "-c", "show bgp neighbor " + address + " json"}, nil)
		}, func(output string) bool {
			fence, err := readFRRAIGPRecipientFence(output, address)
			return err == nil && fence.established == 1 && fence.dropped == 0
		})
		evidence["initial-"+peer] = last
		if err != nil {
			return withLastOutput(err, last)
		}
		initial[index], err = readFRRAIGPRecipientFence(last, address)
		if err != nil {
			return err
		}
	}
	pids := make(map[string]int)
	for _, peer := range []string{"ze", peerInject, peerFRR, peerFRRSink} {
		pid, err := check.Lab.PeerPID(ctx, peer)
		if err != nil {
			return err
		}
		pids[peer] = pid
		evidence["initial-pid-"+peer] = strconv.Itoa(pid)
	}
	if err := releaseSRv6IdentitySource(ctx, check, evidence); err != nil {
		return err
	}

	// No selective verdict before both implementations have accepted the whole
	// non-vacuous population: subject, ordinary control and later FIFO fence.
	for index, peer := range []string{peerFRR, peerFRRSink} {
		hop := srv6IdentityA
		if index == 1 {
			hop = srv6IdentityB
		}
		for number := 1; number <= 3; number++ {
			prefix := "2001:db8:9252:" + strconv.Itoa(number) + "::/64"
			last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
				Timeout: 45 * time.Second, Interval: time.Second, Description: peer + " accepts " + prefix,
			}, func(probe context.Context) (string, error) {
				return check.Lab.Query(probe, peer, []string{cmdVtysh, "-c", "show bgp ipv6 unicast " + prefix + " json"}, nil)
			}, func(output string) bool {
				return requireSRv6IdentityFRRRoute(output, prefix, networkHostAddress(check.Network, uint8(10+index)), hop) == nil
			})
			evidence[peer+"-route-"+strconv.Itoa(number)] = last
			if err != nil {
				return fmt.Errorf("route acceptance, not selective semantics: %w", withLastOutput(err, last))
			}
		}
	}
	for index, peer := range []string{peerFRR, peerFRRSink} {
		address := networkHostAddress(check.Network, uint8(10+index))
		output, err := check.Lab.Query(ctx, peer, []string{cmdVtysh, "-c", "show bgp neighbor " + address + " json"}, nil)
		evidence["final-"+peer] = output
		if err != nil {
			return err
		}
		final, err := readFRRAIGPRecipientFence(output, address)
		if err != nil {
			return err
		}
		if final != initial[index] {
			return fmt.Errorf("%s session replaced: before=%+v after=%+v", peer, initial[index], final)
		}
	}
	for peer, before := range pids {
		after, err := check.Lab.PeerPID(ctx, peer)
		if err != nil {
			return err
		}
		evidence["final-pid-"+peer] = strconv.Itoa(after)
		if after != before {
			return fmt.Errorf("%s daemon replaced during proof", peer)
		}
	}
	for _, host := range []uint8{9, 10, 11} {
		address := networkHostAddress(check.Network, host)
		command := zeCommand("show bgp peer " + address + " detail")
		output, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
		evidence["final-ze-"+address] = output
		if err != nil {
			return err
		}
		if err := requireSRv6IdentityZeSession(output, address); err != nil {
			return err
		}
	}
	command := zeCommand(srv6IdentityState)
	// The route fence precedes the source's EOR, so accepted routes do not
	// prove callback four has run. Wait for its atomic ledger publication.
	// Invalid snapshots are terminal too: never poll past a latched failure
	// or hide malformed state behind a later successful measurement.
	output, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "SRv6 terminal SDK callback receipt",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", command, queryEnvironment("ze", command))
	}, func(output string) bool {
		var receipt *srv6IdentityPolicyReceipt
		if err := json.Unmarshal([]byte(output), &receipt); err != nil || receipt == nil {
			return true
		}
		return receipt.Attempts >= 4 || receipt.Failure != "" ||
			receipt.Rejected != (srv6IdentityRejected{}) || int(receipt.Attempts) != len(receipt.Calls)
	})
	evidence["policy-receipts"] = output
	if err != nil {
		return fmt.Errorf("terminal SDK callback receipt, not selective semantics: %w", withLastOutput(err, output))
	}
	// Readiness is not validity. Judge the exact terminal snapshot with the
	// complete byte/action/population oracle, including failures and extras.
	if err := requireSRv6IdentityPolicy(output, networkHostAddress(check.Network, 11)); err != nil {
		return err
	}
	var semanticErrors []error
	for index, peer := range []string{peerSpeaker, peerSpeaker2} {
		capture, err := check.Lab.Query(ctx, peer, []string{"cat", "/tmp/srv6-identity-ze.jsonl"}, nil)
		evidence[peer+"-wire-verdict-input"] = capture
		if err != nil {
			return err
		}
		if err := requireSRv6IdentityWire(capture, index == 1); err != nil {
			var semantic srv6IdentitySemanticError
			if !errors.As(err, &semantic) {
				// A complete selective red on one receiver cannot mask an
				// incomplete or corrupted capture on the other receiver.
				return fmt.Errorf("%s: %w", peer, err)
			}
			semanticErrors = append(semanticErrors, fmt.Errorf("%s: %w", peer, err))
		}
	}
	return errors.Join(semanticErrors...)
}

func releaseSRv6IdentitySource(ctx context.Context, check *interoplab.CheckContext, evidence map[string]string) error {
	command := zeCommand(srv6IdentityRelease + " " + networkHostAddress(check.Network, 9) + " " + networkHostAddress(check.Network, 2))
	answer, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	evidence["source-release"] = answer
	if err != nil {
		return err
	}
	var receipt struct {
		Announced *uint32 `json:"announced"`
		Withdrawn *uint32 `json:"withdrawn"`
	}
	if err := json.Unmarshal([]byte(answer), &receipt); err != nil {
		return err
	}
	if receipt.Announced == nil {
		return errors.New("source release omitted announcement receipt")
	}
	if receipt.Withdrawn == nil {
		return errors.New("source release omitted withdrawal receipt")
	}
	if *receipt.Announced != 1 {
		return errors.New("source release did not announce exactly one readiness token")
	}
	if *receipt.Withdrawn != 0 {
		return errors.New("source release unexpectedly withdrew a route")
	}
	return nil
}

func requireSRv6IdentityZeSession(output, address string) error {
	var detail struct {
		Peers map[string]struct {
			State       string  `json:"state"`
			EOR         *uint64 `json:"eor-sent"`
			Established *uint64 `json:"connections-established"`
			Dropped     *uint64 `json:"connections-dropped"`
		} `json:"peers"`
	}
	if err := json.Unmarshal([]byte(output), &detail); err != nil {
		return err
	}
	peer, found := detail.Peers[address]
	if !found {
		return errors.New("Ze peer absent")
	}
	if peer.State != "established" {
		return fmt.Errorf("Ze peer %s state %s", address, peer.State)
	}
	for _, counter := range []*uint64{peer.EOR, peer.Established, peer.Dropped} {
		if counter == nil {
			return errors.New("Ze session counters absent")
		}
	}
	if *peer.EOR == 0 {
		return errors.New("Ze initial EOR not yet sent")
	}
	if *peer.Established != 1 {
		return errors.New("Ze peer does not retain its first session")
	}
	if *peer.Dropped != 0 {
		return errors.New("Ze peer session dropped")
	}
	return nil
}

func requireSRv6IdentityFRRRoute(output, prefix, source, hop string) error {
	var route struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Valid  bool `json:"valid"`
			ASPath struct {
				String string `json:"string"`
			} `json:"aspath"`
			Peer struct {
				ID string `json:"peerId"`
			} `json:"peer"`
			NextHops []struct {
				IP string `json:"ip"`
			} `json:"nexthops"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &route); err != nil {
		return err
	}
	if route.Prefix != prefix {
		return fmt.Errorf("FRR prefix %q, want %s", route.Prefix, prefix)
	}
	if len(route.Paths) != 1 {
		return fmt.Errorf("FRR has %d paths for %s, want one", len(route.Paths), prefix)
	}
	path := &route.Paths[0]
	if !path.Valid {
		return errors.New("FRR did not accept a valid path")
	}
	if path.ASPath.String != "65001 65004" {
		return fmt.Errorf("FRR AS path %q, want 65001 65004", path.ASPath.String)
	}
	if path.Peer.ID != source {
		return fmt.Errorf("FRR source %s, want %s", path.Peer.ID, source)
	}
	if len(path.NextHops) != 1 {
		return errors.New("FRR has no single global next hop")
	}
	if path.NextHops[0].IP != hop {
		return fmt.Errorf("FRR next hop %s, want %s", path.NextHops[0].IP, hop)
	}
	return nil
}
