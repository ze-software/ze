// Design: docs/architecture/testing/interop.md -- RFC 8212 default reject judged by FRR.
// Related: register_rfc8212_default_reject.go -- the checker registration.
package bgp

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The prefixes each daemon originates. Ze originates rfc8212ZePrefix toward
// both peers from its config `update` blocks, so the control session's
// delivery and the policy-less session's absence measure the same egress path.
const (
	rfc8212ZePrefix      = "10.81.0.0/24"
	rfc8212FRRPrefix     = "10.81.1.0/24"
	rfc8212SinkPrefix    = "10.81.15.0/24"
	rfc8212FRRAddress    = "172.30.0.3"
	rfc8212ShowRIBBest   = "show bgp rib best"
	rfc8212FRRAdvertised = "show bgp ipv4 unicast neighbors " + zeLabAddress + " advertised-routes json"
)

// checkRFC8212DefaultRejectFRR proves RFC 8212 Section 2 on the session with
// FRR, which Ze configures with no policy: Ze installs none of FRR's routes
// and sends FRR none of its own, while the control session to the FRR sink,
// which carries an explicit policy, exchanges both ways. A reload that adds a
// bgp-level `final accept` in both directions must then deliver every route
// across the FRR session without resetting it.
//
// RFC 8212 Section 2: "Routes are neither imported nor exported unless
// specifically enabled by configuration."
func checkRFC8212DefaultRejectFRR(ctx context.Context, check *interoplab.CheckContext) error {
	assertion := 0
	run := func(operations ...operation) error {
		for i := range operations {
			assertion++
			if err := runOperation(ctx, check.Network, check.Lab, &operations[i]); err != nil {
				return checkerFailure(ctx, check.Lab, scenarioRFC8212DefaultRejectFRR, assertion, err)
			}
		}
		return nil
	}
	frrTable := []string{cmdVtysh, "-c", showTableJSON}
	sessionsKept := []operation{
		{kind: opRequireJSONFields, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON},
			fields: map[string]string{frrFieldConnectionsEstablished: "1", frrFieldConnectionsDropped: "0", frrSessionStateKey: stateEstablished}},
		{kind: opRequireJSONFields, peer: peerFRRSink, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON},
			fields: map[string]string{frrFieldConnectionsEstablished: "1", frrFieldConnectionsDropped: "0", frrSessionStateKey: stateEstablished}},
	}
	// Both sessions up, then the control session's routes cross both ways:
	// the import of the sink's route and the export of Ze's own.
	if err := run(
		operation{kind: opFRRSession, argument: zeLabAddress, timeout: 90 * time.Second},
		operation{kind: opWaitJSONFields, peer: peerFRRSink, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON},
			fields: map[string]string{frrSessionStateKey: stateEstablished}, timeout: 90 * time.Second},
		operation{kind: opWaitContains, peer: "ze", command: zeCommand(rfc8212ShowRIBBest),
			contains: []string{rfc8212SinkPrefix}, timeout: 60 * time.Second},
		operation{kind: opWaitContains, peer: peerFRRSink, command: []string{cmdVtysh, "-c", showTableJSON},
			contains: []string{rfc8212ZePrefix}, timeout: 60 * time.Second},
		// FRR offered its route to Ze, and Ze read that UPDATE and FRR's
		// End-of-RIB, so an absence below is Ze's verdict, not a route never sent.
		operation{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", rfc8212FRRAdvertised},
			contains: []string{rfc8212FRRPrefix}, timeout: 60 * time.Second},
		operation{kind: opWaitJSONFields, peer: "ze", command: zeCommand("show bgp peer " + rfc8212FRRAddress + " detail"),
			minimum: map[string]int{"updates-received": 2}, timeout: 60 * time.Second},
	); err != nil {
		return err
	}
	// RFC 8212 Section 2, both directions of the policy-less session: Ze's RIB
	// holds the sink's route and not FRR's; FRR holds its own route and neither
	// of Ze's; the sink never hears FRR's route, because Ze never imported it.
	if err := run(
		operation{kind: opRequireAbsent, peer: "ze", command: zeCommand(rfc8212ShowRIBBest),
			absent: []string{rfc8212FRRPrefix}, proof: []string{rfc8212SinkPrefix}},
		operation{kind: opRequireAbsent, peer: peerFRR, command: frrTable,
			absent: []string{rfc8212ZePrefix, rfc8212SinkPrefix}, proof: []string{rfc8212FRRPrefix}},
		operation{kind: opRequireAbsent, peer: peerFRRSink, command: frrTable,
			absent: []string{rfc8212FRRPrefix}, proof: []string{rfc8212ZePrefix, rfc8212SinkPrefix}},
	); err != nil {
		return err
	}
	if err := run(sessionsKept...); err != nil {
		return err
	}
	assertion++
	if err := applyZeReload(ctx, check.Lab, zeMountedReloadConfig); err != nil {
		return checkerFailure(ctx, check.Lab, scenarioRFC8212DefaultRejectFRR, assertion, err)
	}
	// The explicit policy now enables both directions. Export first: Ze's own
	// route and the sink's reach FRR. Then import: FRR's route reaches Ze's RIB
	// and, through it, the sink. Neither session may have been reset for it.
	if err := run(
		operation{kind: opWaitContains, peer: peerFRR, command: frrTable,
			contains: []string{rfc8212ZePrefix, rfc8212SinkPrefix}, timeout: 90 * time.Second},
	); err != nil {
		return err
	}
	if err := run(sessionsKept...); err != nil {
		return err
	}
	if err := run(
		operation{kind: opWaitContains, peer: "ze", command: zeCommand(rfc8212ShowRIBBest),
			contains: []string{rfc8212FRRPrefix, rfc8212SinkPrefix}, timeout: 60 * time.Second},
		operation{kind: opWaitContains, peer: peerFRRSink, command: frrTable,
			contains: []string{rfc8212FRRPrefix}, timeout: 60 * time.Second},
	); err != nil {
		return err
	}
	return run(sessionsKept...)
}
