// Design: docs/architecture/testing/interop.md -- the Link-Local-only multihop withdrawal scenario
// Related: check_rfc_predicate.go -- frrDecodeFields, the FRR decode-line reader
// Related: prepare.go -- scenarioFRRImage, the FRR release the scenario pins

package bgp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// frrMultihopLoopback is FRR's session address in the Link-Local-only multihop
// scenario: on FRR's loopback (frr.conf), outside every subnet Ze is attached
// to, so Ze's link scope classifies FRR as off-link.
const frrMultihopLoopback = "10.255.0.3"

// checkLinkLocalOnlyMultihopWithdraw proves, at FRR, that Ze withdraws a route
// whose only next hop is Link-Local instead of relaying it to a peer whose
// session address is outside Ze's connected subnets.
//
// draft-ietf-idr-linklocal-capability Section 4: "If, after completing these
// procedures, there are no IPv6 next hop addresses included in the next hop,
// the BGP route MUST not be advertised to its peer. Instead, treat-as-withdraw
// (Section 2 of [RFC7606]) is used."
//
// The injector relays one prefix twice: generation 1 with a Global next hop,
// generation 2 with the Link-Local fe80::9 alone, then a control prefix. FRR
// negotiates capability 77, so the multihop gate is the only one that can
// withhold generation 2, and FRR's own per-UPDATE decode says what reached it.
func checkLinkLocalOnlyMultihopWithdraw(ctx context.Context, check *interoplab.CheckContext) error {
	const (
		name = "bgp-linklocal-only-multihop-withdraw-frr"
		// inject.msg (1) and (2): one prefix, generation 1 then generation 2.
		subjectPrefix = "2001:db8:5701::/48"
		// inject.msg (3): sent after generation 2 on the same session.
		controlPrefix = "2001:db8:5702::/48"
		// The Global next hop of generation 1 and of the control.
		globalNextHop = "2001:db8:9::9"
		// The only next hop of generation 2.
		linkLocalNextHop = "fe80::9"
	)
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, withFRRLogTail(ctx, check.Lab, cause))
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("link-local multihop scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)

	// Assertion 1. Ze reaches FRR's loopback through FRR's adjacent interface.
	// No transit router is present. The host route adds no connected subnet,
	// so the link scope classifies the session address as off-link.
	route := operation{kind: opExec, peer: "ze", command: []string{"ip", ipObjectRoute, "add", frrMultihopLoopback + "/32", "via", networkHostAddress(check.Network, 3)}}
	if err := runOperation(ctx, check.Network, check.Lab, &route); err != nil {
		return fail(1, err)
	}
	session := operation{kind: opFRRSession, argument: zeAddress}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(1, err)
	}

	// Assertion 2. Generation 1 reached FRR with its Global next hop in place:
	// the prior generation the withdrawal below must take back.
	first := operation{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv6 unicast " + subjectPrefix}, contains: []string{subjectPrefix, globalNextHop}, timeout: 120 * time.Second}
	if err := runOperation(ctx, check.Network, check.Lab, &first); err != nil {
		return fail(2, err)
	}

	// Assertion 3. FRR decoded the control route, which the injector sent after
	// generation 2, so generation 2 has been through the same rail.
	frrLog, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     150 * time.Second,
		Interval:    time.Second,
		Description: "FRR decode of the control route",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdCat, frrLogPath}, nil)
	}, func(text string) bool {
		return frrDecodedPrefix(text, zeAddress, controlPrefix)
	})
	if err != nil {
		return fail(3, err)
	}

	// Assertion 4. In that same log, FRR decoded a WITHDRAWAL of the subject.
	if !frrDecodedWithdrawal(frrLog, zeAddress, subjectPrefix) {
		return fail(4, fmt.Errorf("FRR decoded no withdrawal of %s after its next hop became Link-Local only", subjectPrefix))
	}

	// Assertion 5. FRR decoded exactly one announcement of the subject, generation
	// 1. A second is generation 2 relayed with its Link-Local next hop to the
	// off-link session address.
	if announcements := frrDecodedAnnouncements(frrLog, zeAddress, subjectPrefix); announcements != 1 {
		return fail(5, fmt.Errorf("FRR decoded %d announcements of %s, expected 1 (generation 1 only)", announcements, subjectPrefix))
	}

	// Assertion 5b. No line of that log names the Link-Local next hop. FRR
	// treats a Link-Local-only next hop from a peer with no interface as a
	// withdrawal of its own, so a relayed generation 2 can also end in a
	// withdrawn decode. Its attribute decode, which prints the next hop, is what
	// tells the two apart.
	if strings.Contains(frrLog, linkLocalNextHop) {
		return fail(5, fmt.Errorf("FRR decoded the Link-Local next hop %s, which Ze relayed to an off-link session address", linkLocalNextHop))
	}

	// Assertion 6. FRR's table holds no path for the subject.
	absent := operation{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv6 unicast " + subjectPrefix}, contains: []string{"Network not in table"}}
	if err := runOperation(ctx, check.Network, check.Lab, &absent); err != nil {
		return fail(6, err)
	}

	// Assertion 7. Withholding one route left the session up.
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(7, err)
	}
	return nil
}

// frrDecodedAnnouncements counts FRR's decodes of an UPDATE that ANNOUNCED
// prefix from peer: the decode lines naming it that carry no withdrawn marker.
func frrDecodedAnnouncements(log, peer, prefix string) int {
	count := 0
	for line := range strings.SplitSeq(log, "\n") {
		fields := frrDecodeFields(line, peer, prefix)
		if fields == nil {
			continue
		}
		if slices.Contains(fields, frrWithdrawnMarker) {
			continue
		}
		count++
	}
	return count
}

// frrLogTailOctets bounds the FRR log a failure carries: enough for the last
// session events and decodes, short enough to read.
const frrLogTailOctets = 6000

// withFRRLogTail joins the tail of FRR's own log to cause. The container logs
// checkerFailure attaches stop at FRR's startup, and the reason FRR ends a
// session or refuses an UPDATE is only in this file.
func withFRRLogTail(ctx context.Context, lab interoplab.CheckerLab, cause error) error {
	text, err := lab.Query(ctx, peerFRR, []string{cmdCat, frrLogPath}, nil)
	if err != nil {
		return fmt.Errorf("%w (FRR log unreadable: %w)", cause, err)
	}
	if len(text) > frrLogTailOctets {
		text = text[len(text)-frrLogTailOctets:]
	}
	return fmt.Errorf("%w\n--- FRR log tail ---\n%s", cause, text)
}
