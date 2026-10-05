// Design: docs/architecture/testing/interop.md -- RFC 8654 directional peer matrix.
// Related: check_extended_message_predicate.go -- independent frame and FRR predicates.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkExtendedMessages uses FRR as both producer and consumer, on separate
// sessions. The relay changes only capability 6 in OPENs because FRR 10.3.1 itself
// uses bilateral packet-size negotiation. Ze's original OPEN is checked separately
// from the delivered copy, so the relay cannot grant Ze local receive permission.
// RFC 8654 Section 4: "A BGP speaker MAY send BGP Extended Messages to a peer only
// if the BGP Extended Message Capability was received from that peer."
func checkExtendedMessages(ctx context.Context, check *interoplab.CheckContext, testCase extendedMessageCase) error {
	fail := func(assertion int, err error) error {
		return checkerFailure(ctx, check.Lab, check.Source.Name, assertion, err)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("extended-message scenario has no selected network"))
	}
	// Assertion 1: two real FRR sessions and a path FRR actually learned from Ze.
	for _, host := range []byte{10, 11} {
		neighbor := networkHostAddress(check.Network, host)
		if err := waitContains(ctx, check.Lab, peerFRR,
			[]string{cmdVtysh, "-c", "show bgp neighbor " + neighbor},
			60*time.Second, "BGP state = Established"); err != nil {
			return fail(1, err)
		}
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
		return fail(6, errors.New("Ze sent an extended UPDATE to a non-advertising sink"))
	}
	if sink.notification != nil {
		return fail(6, errors.New("Ze notified the sink during a permitted session"))
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
	for _, host := range []byte{10, 11} {
		generation, err := queryFRRSessionGeneration(ctx, check.Lab, networkHostAddress(check.Network, host))
		if err != nil {
			return fail(7, err)
		}
		if generation != 1 {
			return fail(7, errors.New("FRR session was replaced during the extended UPDATE"))
		}
	}
	return nil
}

// checkExtendedReceiveRejection also reads FRR's session state: a captured
// NOTIFICATION without teardown is insufficient evidence for this negative case.
// RFC 8654 Section 5: "A BGP speaker that has the ability to use BGP Extended
// Messages but has not advertised the BGP Extended Message Capability, presumably
// due to configuration, MUST NOT accept a BGP Extended Message."
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
		output, err := queryFRRIPv4Route(probeCtx, check.Lab, prefix)
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
