// Design: docs/architecture/testing/interop.md -- received FlowSpec interoperability.
// Related: check_flowspec_gobgp_predicate.go -- foreign semantics and exact wire fences.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	flowSpecReceivedPrefix = "10.99.77.0/24"
	flowSpecCapturePath    = "/tmp/flowspec-ze.jsonl"
)

// checkFlowSpecGoBGP retains the originated rules and OR-of-AND assertion, then
// drives one received rule through live forwarding and cover-only stored replay.
// RFC 8955 Sections 4, 6 and 7.1: foreign decoding supplements, rather than
// substitutes for, the exact MP_REACH next-hop-length and action observations.
// MUTATION: bypass applyNextHopFamily's no-next-hop normalization; unchanged
// forwarding must not emit the received IPv6 hop on the IPv4 FlowSpec rule.
// MUTATION: omit AttrExtCommunity from RouteEntry.ToWireBytes; cover recovery
// must then fail both the exact action and GoBGP traffic-rate assertions.
func checkFlowSpecGoBGP(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("FlowSpec scenario has no selected IPv4 network")
	}
	if err := checkFlowSpecBaseline(ctx, check); err != nil {
		return err
	}
	source := networkHostAddress(check.Network, 9)
	if err := waitZePeerState(ctx, check.Lab, source, 30*time.Second); err != nil {
		return err
	}
	// A foreign cover-route observation precedes the only rule input, so the
	// initial UPDATE can take the live forwarding rail, not peer-up replay.
	if err := flowSpecReleaseStage(ctx, check, "198.18.0.1/32"); err != nil {
		return err
	}
	if err := waitFlowSpecCover(ctx, check); err != nil {
		return err
	}
	if err := flowSpecReleaseStage(ctx, check, "198.18.0.2/32"); err != nil {
		return err
	}
	// RFC 8955 Sections 4 and 7.1: exact zero-hop rule and traffic-rate action.
	live, err := waitFlowSpecEvidence(ctx, check, 0, true)
	if err != nil {
		return fmt.Errorf("live received FlowSpec: %w", err)
	}
	if err := flowSpecReleaseStage(ctx, check, "198.18.0.3/32"); err != nil {
		return err
	}
	// RFC 8955 Section 6: no cover means no usable rule, but the baseline
	// rules must still be decoded. A failed/empty query cannot prove absence.
	withdrawn, err := waitFlowSpecEvidence(ctx, check, live, false)
	if err != nil {
		return fmt.Errorf("cover withdrawal: %w", err)
	}
	if err := waitZePeerState(ctx, check.Lab, source, 30*time.Second); err != nil {
		return err
	}
	if err := flowSpecReleaseStage(ctx, check, "198.18.0.4/32"); err != nil {
		return err
	}
	// RFC 8955 Sections 4, 6 and 7.1: the injector restores only unicast.
	if _, err := waitFlowSpecEvidence(ctx, check, withdrawn, true); err != nil {
		return fmt.Errorf("stored FlowSpec replay: %w", err)
	}
	if err := waitFlowSpecSourceComplete(ctx, check.Lab); err != nil {
		return err
	}
	if err := waitZePeerState(ctx, check.Lab, source, 30*time.Second); err != nil {
		return err
	}
	return checkFlowSpecBaseline(ctx, check)
}

// checkFlowSpecBaseline preserves all three original scenario operations. The
// peer address alone changes: the wire relay is now GoBGP's TCP neighbor.
func checkFlowSpecBaseline(ctx context.Context, check *interoplab.CheckContext) error {
	operations := []operation{
		{kind: opGoBGPSession, argument: "172.30.0.10"},
		{kind: opWaitContains, peer: peerGoBGP,
			command:  []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4Flowspec, "-j"},
			contains: []string{peerPrefixFirst, peerPrefixSecond}, timeout: 30 * time.Second},
		{kind: opWaitContains, peer: peerGoBGP,
			command:  []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4Flowspec},
			contains: []string{flowspecOrOfAndRule}, timeout: 30 * time.Second},
	}
	for index := range operations {
		if err := runOperation(ctx, check.Network, check.Lab, &operations[index]); err != nil {
			return err
		}
	}
	return nil
}

func waitFlowSpecCover(ctx context.Context, check *interoplab.CheckContext) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "foreign same-originator covering route",
	}, func(probeCtx context.Context) (bool, error) {
		output, err := check.Lab.Query(probeCtx, peerGoBGP,
			[]string{cmdGoBGP, gobgpGlobal, gobgpRIB, flowSpecReceivedPrefix, "-j"}, nil)
		if err != nil {
			return false, err
		}
		// RFC 8955 Section 6: the unicast cover precedes the FlowSpec input.
		err = flowSpecForeignCover(output, networkHostAddress(check.Network, 10))
		return err == nil, err
	}, func(ready bool) bool { return ready })
	return err
}

// flowSpecReleaseStage adds a distinct unicast marker at GoBGP. The source's
// single-session dialog waits for that exact announced NLRI before proceeding.
func flowSpecReleaseStage(ctx context.Context, check *interoplab.CheckContext, marker string) error {
	_, err := check.Lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, marker,
		gobgpNextHop, networkHostAddress(check.Network, 5),
	}, nil)
	return err
}

// waitFlowSpecEvidence MUST follow the preceding stage's foreign verdict. Its
// returned frame fence MUST precede the next stage release. The relay owns one
// TCP session and never reconnects, so an old announcement cannot pass recovery.
func waitFlowSpecEvidence(ctx context.Context, check *interoplab.CheckContext, after int, present bool) (int, error) {
	var last string
	frame, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "GoBGP FlowSpec rule/action and fresh wire event",
	}, func(probeCtx context.Context) (int, error) {
		capture, err := check.Lab.Query(probeCtx, peerSpeaker, []string{cmdCat, flowSpecCapturePath}, nil)
		if err != nil {
			return 0, err
		}
		// RFC 8955 Section 4: observe the actual bytes delivered to GoBGP.
		wire, err := flowSpecWireEvidence(capture, networkHostAddress(check.Network, 2))
		if err != nil {
			return 0, err
		}
		if wire.frame <= after {
			return 0, errors.New("no new target FlowSpec event after the stage fence")
		}
		if wire.present != present {
			return 0, errors.New("latest target FlowSpec wire event has the wrong disposition")
		}
		last, err = check.Lab.Query(probeCtx, peerGoBGP,
			[]string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4Flowspec, "-j"}, nil)
		if err != nil {
			return 0, err
		}
		// RFC 8955 Sections 6 and 7.1: independent decoder confirms usability.
		if err := flowSpecForeignEvidence(last, networkHostAddress(check.Network, 10), present); err != nil {
			return 0, err
		}
		return wire.frame, nil
	}, func(frame int) bool { return frame > after })
	if err != nil {
		return 0, fmt.Errorf("%w; GoBGP last answered: %s", err, last)
	}
	return frame, nil
}

func waitFlowSpecSourceComplete(ctx context.Context, lab interoplab.CheckerLab) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "one rule input and cover-only replay",
	}, func(probeCtx context.Context) (bool, error) {
		logs, err := lab.Logs(probeCtx, peerInject, 2000)
		if err != nil {
			return false, err
		}
		if !logs.Available {
			return false, errors.New("injector transcript unavailable")
		}
		err = flowSpecSourceComplete(logs.Text)
		return err == nil, err
	}, func(ready bool) bool { return ready })
	return err
}
