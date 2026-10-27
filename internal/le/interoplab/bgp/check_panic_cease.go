// Design: docs/architecture/testing/interop.md -- a contained read-path panic, judged by FRR and BIRD.
// Related: register_panic_cease.go -- the registration
// Related: prepare.go -- scenarioPeers, which starts the ze_le daemon for zeLeDaemonFile
// Related: ../../../component/bgp/reactor/session_panic_inject_le.go -- the injection it triggers

package bgp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioPanicCease = "bgp-panic-cease"

	// zeLeDaemonFile, in a scenario directory, makes the ze container start
	// the daemon built with ze_le (LabBinaries) rather than the shipped one.
	zeLeDaemonFile = "ze-le-daemon"
	// zeLeDaemonPath is where test/interop/Dockerfile.ze puts that daemon,
	// outside PATH so `ze` stays the shipped build everywhere else.
	zeLeDaemonPath = "/usr/local/lib/ze-le/ze"

	// panicCeaseTrigger creates the trigger file the ze_le reactor reads
	// (readPanicTriggerDir, readPanicTriggerPrefix) for the FRR session.
	panicCeaseTrigger = "mkdir -p /run/ze-inject && touch /run/ze-inject/bgp-read-panic-" + frrLabAddress
	// panicCeaseRoute is the static route FRR redistributes once the trigger
	// is armed: its UPDATE is the message the injection panics on.
	panicCeaseRoute = "10.49.1.0/24"
	// birdBystanderAddress is BIRD's address in this scenario's bird.conf.
	birdBystanderAddress = "172.30.0.4"

	// frrFieldConnectionsEstablished and frrFieldConnectionsDropped are FRR's
	// session counters in `show bgp neighbor <address> json`.
	frrFieldConnectionsEstablished = "connectionsEstablished"
	frrFieldConnectionsDropped     = "connectionsDropped"
)

// checkPanicCease proves the read-path panic boundary end to end, judged by
// the peers rather than by ze's own log:
//
//  1. FRR and BIRD both reach Established with ze.
//  2. The trigger is armed in the ze container, and FRR sends an UPDATE.
//  3. FRR's own state says it RECEIVED a NOTIFICATION Cease whose subcode is
//     Unspecific (RFC 4271 Section 4.5), which only ceaseAfterPanic sends here.
//  4. The FRR session comes back: FRR counts two establishments, one drop.
//  5. ze counts one establishment and no drop on the BIRD session, and BIRD
//     still reports Established: the panic stayed inside its own session.
//
// NON-VACUITY: removing the NOTIFICATION from ceaseAfterPanic leaves FRR with
// a closed socket and no notification, so step 3 times out; a panic that
// escaped its session would end the daemon, so steps 4 and 5 fail.
func checkPanicCease(ctx context.Context, check *interoplab.CheckContext) error {
	assertion := 0
	run := func(operations ...operation) error {
		for i := range operations {
			assertion++
			if err := runOperation(ctx, check.Network, check.Lab, &operations[i]); err != nil {
				return checkerFailure(ctx, check.Lab, scenarioPanicCease, assertion, err)
			}
		}
		return nil
	}
	if err := run(
		operation{kind: opFRRSession, argument: zeLabAddress, timeout: 90 * time.Second},
		operation{kind: opBIRDSession, argument: birdZeProtocol, timeout: 90 * time.Second},
		operation{kind: opExec, peer: "ze", command: []string{"sh", "-c", panicCeaseTrigger}},
		operation{kind: opExec, peer: peerFRR, command: []string{cmdVtysh, "-c", frrConfigureTerminal,
			"-c", "ip route " + panicCeaseRoute + " Null0"}},
	); err != nil {
		return err
	}

	assertion++
	if err := waitFRRReceivedCeaseUnspecific(ctx, check); err != nil {
		return checkerFailure(ctx, check.Lab, scenarioPanicCease, assertion, err)
	}

	return run(
		operation{kind: opFRRSession, argument: zeLabAddress, timeout: 90 * time.Second},
		operation{kind: opWaitJSONFields, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON},
			fields:  map[string]string{frrFieldConnectionsEstablished: "2", frrFieldConnectionsDropped: "1", frrSessionStateKey: stateEstablished},
			timeout: 30 * time.Second},
		operation{kind: opRequireJSONFields, peer: "ze",
			command: zeCommand("show bgp peer " + birdBystanderAddress + " detail"),
			fields:  map[string]string{zeFieldConnectionsEstablished: "1", zeFieldConnectionsDropped: "0"}},
		operation{kind: opBIRDSession, argument: birdZeProtocol},
	)
}

// waitFRRReceivedCeaseUnspecific waits until FRR's neighbor state says the
// session to ze last ended on a NOTIFICATION FRR received, and that it was a
// Cease with no named subcode.
func waitFRRReceivedCeaseUnspecific(ctx context.Context, check *interoplab.CheckContext) error {
	// Lab.Query runs the command as written, so the address is this run's
	// network rather than the 172.30.0.0/24 the scenario files spell.
	neighbor := networkHostAddress(check.Network, zeHostNumber)
	command := []string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}
	var last string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second,
		Description: "FRR received a NOTIFICATION Cease / Unspecific from ze"},
		func(probeCtx context.Context) (map[string]any, error) {
			output, queryErr := check.Lab.Query(probeCtx, peerFRR, command, nil)
			if queryErr != nil {
				return nil, queryErr
			}
			last = output
			var document map[string]map[string]any
			if jsonErr := json.Unmarshal([]byte(output), &document); jsonErr != nil {
				return nil, jsonErr
			}
			return document[neighbor], nil
		}, frrReceivedCeaseUnspecific)
	if err != nil {
		return interoplab.WithSearchedOutput(err, last)
	}
	return nil
}

// frrReceivedCeaseUnspecific answers whether FRR's neighbor JSON records a
// received Cease carrying subcode 0. FRR prints the reason as the code name
// followed by the subcode name, so a Cease with a named subcode (Out of
// Resources, Administrative Reset) is refused; FRR names subcode 0
// "Unspecific", and a reason of the bare code is accepted as the same answer.
func frrReceivedCeaseUnspecific(peer map[string]any) bool {
	dueTo := strings.ToLower(fmt.Sprint(peer["lastResetDueTo"]))
	if !strings.Contains(dueTo, "receive") {
		return false
	}
	reason := strings.ToLower(fmt.Sprint(peer["lastNotificationReason"]))
	subcode, found := strings.CutPrefix(reason, "cease")
	if !found {
		return false
	}
	subcode = strings.TrimSpace(subcode)
	if subcode == "" {
		return true
	}
	return strings.HasPrefix(subcode, "/unspecific")
}
