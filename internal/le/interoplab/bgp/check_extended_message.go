// Design: docs/architecture/testing/interop.md -- RFC 8654 directional peer matrix.
// Related: check_extended_message_predicate.go -- independent frame and FRR predicates.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkExtendedMessages uses separate FRR daemons as producer and consumer.
// The relay changes only capability 6 in OPENs because FRR 10.3.1 itself
// uses bilateral packet-size negotiation. Ze's original OPEN is checked separately
// from the delivered copy, so the relay cannot grant Ze local receive permission.
// RFC 8654 Section 4: "A BGP speaker MAY send BGP Extended Messages to a peer only
// if the BGP Extended Message Capability was received from that peer".
func checkExtendedMessages(ctx context.Context, check *interoplab.CheckContext, testCase extendedMessageCase) (resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%w%s", resultErr, extendedFailureDiagnostics(ctx, check.Lab))
		}
	}()
	fail := func(assertion int, err error) error {
		return checkerFailure(ctx, check.Lab, check.Source.Name, assertion, err)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("extended-message scenario has no selected network"))
	}
	// Assertion 1: two real FRR sessions and a path FRR actually learned from Ze.
	sessions := [...]struct {
		peer string
		host uint8
	}{{peerFRR, 10}, {peerFRRSink, 11}}
	readyCtx, cancelReady := context.WithTimeout(ctx, 60*time.Second)
	defer cancelReady()
	for _, session := range sessions {
		neighbor := networkHostAddress(check.Network, session.host)
		if err := waitContains(readyCtx, check.Lab, session.peer,
			[]string{cmdVtysh, "-c", "show bgp neighbor " + neighbor},
			60*time.Second, "BGP state = Established"); err != nil {
			return fail(1, err)
		}
		// Peer.State is published after encoding contexts and forwarding facts;
		// FRR's Established state alone does not prove that local publication.
		if err := waitZePeerState(readyCtx, check.Lab, neighbor, 60*time.Second); err != nil {
			return fail(1, err)
		}
	}
	cancelReady()
	// First origination, not a replay: the source relay starts before the sink,
	// and the native fast path cannot forward to a peer without a live session.
	if err := originateExtendedFRRRoute(ctx, check.Lab, extendedBaselinePrefix); err != nil {
		return fail(1, err)
	}
	if err := waitExtendedFRRRoute(ctx, check, extendedBaselinePrefix, false); err != nil {
		return fail(1, err)
	}
	// Assertion 2: original local advertisements match configuration; the foreign
	// daemon really advertised capability 6 and only that capability was rewritten.
	if _, err := waitExtendedTranscript(ctx, check.Lab, peerSpeaker, "ze", testCase.sourceLocal, true,
		func(c extendedCapture) bool { return c.keepalives >= 1 }); err != nil {
		return fail(2, err)
	}
	if _, err := waitExtendedTranscript(ctx, check.Lab, peerSpeaker, "frr", true, testCase.sourceRemote,
		func(c extendedCapture) bool { return c.keepalives >= 1 }); err != nil {
		return fail(2, err)
	}
	if _, err := waitExtendedTranscript(ctx, check.Lab, peerSpeaker2, "ze", testCase.sinkLocal, true,
		func(c extendedCapture) bool { return c.keepalives >= 1 }); err != nil {
		return fail(2, err)
	}
	if _, err := waitExtendedTranscript(ctx, check.Lab, peerSpeaker2, "frr", true, testCase.sinkRemote,
		func(c extendedCapture) bool { return c.keepalives >= 1 }); err != nil {
		return fail(2, err)
	}
	// Assertion 3: FRR, not the relay, encodes the indivisible 4800-octet attribute.
	if err := originateExtendedFRRRoute(ctx, check.Lab, extendedLargePrefix+" route-map EXTENDED"); err != nil {
		return fail(3, err)
	}
	source, err := waitExtendedTranscript(ctx, check.Lab, peerSpeaker, "frr", true, testCase.sourceRemote,
		func(c extendedCapture) bool { return c.largeOctets > 4096 })
	if err != nil {
		return fail(3, err)
	}
	if !testCase.sourceLocal {
		// RFC 8654 Sections 4 and 5: local advertisement alone gates reception.
		return checkExtendedReceiveRejection(ctx, check, testCase, source.largeOctets)
	}
	// Assertion 4: FRR's independent decoder reads the full forwarded attribute.
	if testCase.sinkRemote {
		if err := waitExtendedFRRRoute(ctx, check, extendedLargePrefix, true); err != nil {
			return fail(4, err)
		}
	}
	// Assertion 5: a later control bounds the negative egress assertion. It is
	// originated only after FRR's large UPDATE was delivered on the same TCP stream.
	if err := originateExtendedFRRRoute(ctx, check.Lab, extendedControlPrefix); err != nil {
		return fail(5, err)
	}
	if err := waitExtendedFRRRoute(ctx, check, extendedControlPrefix, false); err != nil {
		return fail(5, err)
	}
	sink, err := waitExtendedTranscript(ctx, check.Lab, peerSpeaker2, "ze", testCase.sinkLocal, true,
		func(c extendedCapture) bool { return c.control && c.keepalives >= 3 })
	if err != nil {
		return fail(6, err)
	}
	// RFC 8654 Section 4: a remote advertisement is the sole send permission.
	if testCase.sinkRemote {
		if sink.largeOctets <= 4096 {
			return fail(6, errors.New("FRR route exists but Ze sent no extended UPDATE for it"))
		}
	} else if sink.updatesLarge != 0 {
		return fail(6, errors.New("ze sent an extended UPDATE to a non-advertising sink"))
	}
	if sink.notification != nil {
		return fail(6, errors.New("ze notified the sink during a permitted session"))
	}
	// Assertion 7: OPEN/KEEPALIVE retain their ordinary framing; both original
	// sessions remain alive. The no-reconnect relay and FRR counter reject a reset.
	for _, direction := range []string{"ze", "frr"} {
		original, delivered := testCase.sourceLocal, true
		if direction == "frr" {
			original, delivered = true, testCase.sourceRemote
		}
		capture, err := waitExtendedTranscript(ctx, check.Lab, peerSpeaker, direction, original, delivered,
			func(c extendedCapture) bool { return c.keepalives >= 3 })
		if err != nil {
			return fail(7, err)
		}
		if capture.notification != nil {
			return fail(7, errors.New("permitted extended UPDATE reset the source session"))
		}
	}
	sinkPeer, err := waitExtendedTranscript(ctx, check.Lab, peerSpeaker2, "frr", true, testCase.sinkRemote,
		func(c extendedCapture) bool { return c.keepalives >= 3 })
	if err != nil {
		return fail(7, err)
	}
	if sinkPeer.notification != nil {
		return fail(7, errors.New("FRR rejected the sink's forwarded UPDATE"))
	}
	for _, session := range sessions {
		neighbor := networkHostAddress(check.Network, session.host)
		output, err := check.Lab.Query(ctx, session.peer,
			[]string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}, nil)
		if err != nil {
			return fail(7, err)
		}
		generation, err := parseFRRSessionGeneration(output, neighbor)
		if err != nil {
			return fail(7, err)
		}
		if generation != 1 {
			return fail(7, errors.New("FRR session was replaced during the extended UPDATE"))
		}
	}
	return nil
}

// Read both single-session relays before teardown, including empty captures and
// missing result files. These distinguish an unfinished dial from a closed socket.
// The relay bounds each capture to 256 frames; all queries share a 15-second limit.
// Diagnostic errors are retained beside the original failure, never a pass.
func extendedFailureDiagnostics(ctx context.Context, lab interoplab.CheckerLab) string {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()

	var output textbuf.Buffer
	for _, peer := range []string{peerSpeaker, peerSpeaker2} {
		for _, path := range []string{
			extendedCaptureBase + "-result.json",
			extendedCaptureBase + "-ze.jsonl",
			extendedCaptureBase + "-frr.jsonl",
			"/proc/net/tcp",
		} {
			answer, err := lab.Exec(ctx, peer, []string{cmdCat, path}, nil)
			output.Str("\n--- ").Str(peer).Str(": ").Str(path).Str(" ---\n").Str(answer.Stdout)
			output.Str("\nexit: ").Int(int64(answer.ExitCode)).Str("\nstderr: ").Str(answer.Stderr).Byte('\n')
			if err != nil {
				output.Str("\nquery error: ").Str(err.Error()).Byte('\n')
			}
		}
		logs, err := lab.Logs(ctx, peer, 80)
		output.Str("\n--- ").Str(peer).Str(" relay logs ---\n").Str(logs.Text)
		if err != nil {
			output.Str("\nlogs error: ").Str(err.Error()).Byte('\n')
		} else if !logs.Available {
			output.Str("\nlogs unavailable\n")
		}
	}
	return output.String()
}

// checkExtendedReceiveRejection also reads FRR's session state: a captured
// NOTIFICATION without teardown is insufficient evidence for this negative case.
// RFC 8654 Section 5: "A BGP speaker that has the ability to use BGP Extended
// Messages but has not advertised the BGP Extended Message Capability, presumably
// due to configuration, MUST NOT accept a BGP Extended Message".
func checkExtendedReceiveRejection(ctx context.Context, check *interoplab.CheckContext,
	testCase extendedMessageCase, offendingOctets int) error {
	capture, err := waitExtendedTranscript(ctx, check.Lab, peerSpeaker, "ze", testCase.sourceLocal, true,
		func(c extendedCapture) bool { return c.notification != nil })
	if err != nil {
		return err
	}
	// RFC 8654 Section 4: exact error and offending length, not any disconnect.
	if err := requireExtendedRejection(capture.notification, offendingOctets); err != nil {
		return err
	}
	neighbor := networkHostAddress(check.Network, 10)
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "FRR observes fatal extended length teardown",
	}, func(probeCtx context.Context) (bool, error) {
		output, err := check.Lab.Query(probeCtx, peerFRR,
			[]string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}, nil)
		if err != nil {
			return false, err
		}
		return extendedFRRSessionClosed(output, neighbor)
	}, func(closed bool) bool { return closed })
	return err
}

func waitExtendedTranscript(ctx context.Context, lab interoplab.CheckerLab, peer, direction string,
	original, delivered bool, ready func(extendedCapture) bool) (extendedCapture, error) {
	capture, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "complete extended-message relay evidence",
	}, func(probeCtx context.Context) (extendedCapture, error) {
		text, err := lab.Query(probeCtx, peer, []string{cmdCat, extendedCaptureBase + "-" + direction + ".jsonl"}, nil)
		if err != nil {
			return extendedCapture{}, err
		}
		// RFC 8654 Sections 3, 4 and 6: original and delivered OPENs stay distinct.
		return parseExtendedCapture(text, original, delivered)
	}, ready)
	return capture, err
}

func waitExtendedFRRRoute(ctx context.Context, check *interoplab.CheckContext, prefix string, large bool) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "FRR installs Ze-forwarded route",
	}, func(probeCtx context.Context) (bool, error) {
		output, err := check.Lab.Query(probeCtx, peerFRRSink,
			[]string{cmdVtysh, "-c", "show bgp ipv4 unicast " + prefix + " json"}, nil)
		if err != nil {
			return false, err
		}
		if err := requireExtendedFRRRoute(output, prefix, networkHostAddress(check.Network, 11), large); err != nil {
			return false, err
		}
		return true, nil
	}, func(installed bool) bool { return installed })
	return err
}

func originateExtendedFRRRoute(ctx context.Context, lab interoplab.CheckerLab, network string) error {
	result, err := lab.Exec(ctx, peerFRR, []string{
		cmdVtysh, "-c", frrConfigureTerminal, "-c", "router bgp 65002",
		"-c", "address-family ipv4 unicast", "-c", "network " + network,
	}, nil)
	if err != nil {
		return fmt.Errorf("FRR origin command: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("FRR origin command exited %d: %s", result.ExitCode, result.Stderr)
	}
	return nil
}
