// Design: docs/labs/l2tp-interop.md -- original accounting fixtures in the native suite.

package l2tp

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const radiusFixtureTrace = `OK: CHAP Response sent for challenge identifier 1 as subscriber@ze
OK: CHAP Success received
OK: IPCP opened, ze assigned 10.99.7.10
OK: Access-Request NAS-Port-Id resolved from the template as lns1:1.2
OK: Accounting-Start Framed-IP-Address matches the IPCP-negotiated address 10.99.7.10
OK: Accounting-Start carries the same NAS-Port-Id lns1:1.2
OK: Accounting-Start carries Event-Timestamp 1, Calling-Station-Id +441632960123, Acct-Delay-Time 0 and no Acct-Terminate-Cause
OK: Accounting-Interim-Update carries Event-Timestamp 1, Calling-Station-Id +441632960123, Acct-Delay-Time 0 and no Acct-Terminate-Cause
OK: peer stopped answering LCP echo probes
OK: Accounting-Stop carries Event-Timestamp 2, Calling-Station-Id +441632960123, Acct-Delay-Time 0 and Acct-Terminate-Cause 2
radius-mock listening on 127.0.0.1:1812
RADIUS-RX Access-Request NAS-Port-Id=lns1:1.2
RADIUS-RX Accounting-Request Acct-Status-Type=Start
RADIUS-RX Accounting-Request Acct-Status-Type=Interim-Update
RADIUS-RX Accounting-Request Acct-Status-Type=Stop
`

func TestRadiusAccountingFixtureEvidence(t *testing.T) {
	if err := radiusAccountingFixtureEvidence(radiusFixtureTrace); err != nil {
		t.Fatal(err)
	}
	// Every evidence line comes from a distinct assertion or decoded packet.
	// A successful process status cannot replace any of those observations.
	for line := range strings.SplitSeq(strings.TrimSpace(radiusFixtureTrace), "\n") {
		t.Run(line, func(t *testing.T) {
			missing := strings.ReplaceAll(radiusFixtureTrace, line+"\n", "")
			if err := radiusAccountingFixtureEvidence(missing); err == nil {
				t.Fatalf("missing evidence passed: %s", line)
			}
		})
	}
}

type radiusFixtureLab struct {
	recordingLab
	status     string
	output     string
	startError bool
}

func (l *radiusFixtureLab) ExecDetached(_ context.Context, peer string, arguments []string, _ []interoplab.EnvironmentVariable) error {
	l.calls = append(l.calls, recordedCall{operation: "exec-detached", peer: peer, arguments: append([]string(nil), arguments...)})
	if l.startError {
		return errors.New("worker unavailable")
	}
	return nil
}

func (l *radiusFixtureLab) Exec(ctx context.Context, peer string, arguments []string, environment []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: append([]string(nil), arguments...)})
	if reflect.DeepEqual(arguments, []string{"sh", "-c", radiusFixtureStatus}) {
		return interoplab.CommandResult{Stdout: l.status}, nil
	}
	if reflect.DeepEqual(arguments, []string{"cat", "/tmp/l2tp-radius-acct-wire/worker.log"}) {
		return interoplab.CommandResult{Stdout: l.output}, nil
	}
	if reflect.DeepEqual(arguments, []string{"ip", "-o", "link", "show", "type", "ppp"}) {
		return interoplab.CommandResult{}, nil
	}
	return l.recordingLab.Exec(ctx, peer, arguments, environment)
}

func TestRadiusAccountingFixtureChecker(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     string
		output     string
		startError bool
		valid      bool
	}{
		{"original fixtures completed", "0", radiusFixtureTrace, false, true},
		{"peer failure with complete output", "1", radiusFixtureTrace, false, false},
		{"missing evidence", "0", "usage: le test fixture", false, false},
		{"worker unavailable", "", "", true, false},
		{"empty status", "", radiusFixtureTrace, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lab := &radiusFixtureLab{status: tc.status, output: tc.output, startError: tc.startError}
			check := scenarioCheckerMap(time.Second)[scenarioRadiusFixture]
			if check == nil {
				t.Fatal("accounting fixture checker is not registered")
			}
			err := check(t.Context(), &interoplab.CheckContext{Lab: lab})
			if (err == nil) != tc.valid {
				t.Fatalf("checker error=%v, want valid=%t", err, tc.valid)
			}
			first := lab.calls[0]
			if first.operation != "exec-detached" || first.peer != peerZe || !reflect.DeepEqual(first.arguments, []string{"sh", "-c", radiusFixtureWorker}) {
				t.Fatalf("worker call=%+v", first)
			}
			for _, call := range lab.calls {
				if call.peer != peerZe {
					t.Fatalf("original fixtures escaped the Ze namespace: %+v", call)
				}
			}
		})
	}
}

func TestRadiusAccountingFixtureUsesOriginalRegistrations(t *testing.T) {
	for _, invocation := range []string{
		"le test fixture l2tp/radius-acct-wire/radius 1812 testing123",
		"le test fixture l2tp/radius-acct-wire 1701",
	} {
		if !strings.Contains(radiusFixtureWorker, invocation) {
			t.Fatalf("worker omits original fixture invocation %q", invocation)
		}
	}
	// A poll must see either no result or the complete exit status, even if
	// it runs between opening the worker's result file and writing its bytes.
	publication := "printf '%s\\n' \"$status\" >status.tmp\n    mv status.tmp status"
	if !strings.Contains(radiusFixtureWorker, publication) {
		t.Fatal("worker does not publish the complete exit status by atomic rename")
	}
	joined := strings.Index(radiusFixtureWorker, "wait \"$radius_pid\"")
	if joined < 0 || joined > strings.Index(radiusFixtureWorker, publication) {
		t.Fatal("worker publishes its result before joining the RADIUS mock")
	}
	root := l2tpCheckout(t)
	plans, kernelRequired, err := plansAt(root, scenarioRadiusFixture, testEnvironment())
	if err != nil {
		t.Fatal(err)
	}
	if !kernelRequired || len(plans) != 1 {
		t.Fatalf("accounting plans=%d kernelRequired=%t", len(plans), kernelRequired)
	}
	prepared, err := plans[0].Prepare(t.Context(), interoplab.PrepareContext{
		Source: plans[0].Source, Network: (&recordingLab{}).Network(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Peers) != 1 || prepared.Peers[0].Name != peerZe {
		t.Fatalf("accounting peers=%+v", prepared.Peers)
	}
}
