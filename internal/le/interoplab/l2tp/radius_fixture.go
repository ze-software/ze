// Design: docs/labs/l2tp-interop.md -- original accounting fixture regression on Ze's kernel.
// Related: internal/test/fixture/tunnel_fixture_l2tp_ppp.go -- unchanged accounting peer.

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The native suite owns this container and removes it after the checker. The
// worker MUST stop its mock before publishing its exit status; the checker MUST
// read that status and both original fixtures' evidence before reporting success.
// Both fixtures share the private directory because the peer reads radius-rx.log.
const radiusFixtureWorker = `set -eu
mkdir /tmp/l2tp-radius-acct-wire
cd /tmp/l2tp-radius-acct-wire
exec >worker.log 2>&1
export TEST_SECRET=s3cr3t TEST_NAS_ID=lns1
le test fixture l2tp/radius-acct-wire/radius 1812 testing123 >radius-mock.log 2>&1 &
radius_pid=$!
finish() {
    status=$?
    trap - EXIT HUP INT TERM
    kill "$radius_pid" 2>/dev/null || true
    wait "$radius_pid" 2>/dev/null || true
    cat radius-mock.log
    printf '%s\n' "$status" >status.tmp
    mv status.tmp status
}
trap finish EXIT
trap 'exit 1' HUP INT TERM
attempt=0
until grep -q 'radius-mock listening on 127.0.0.1:1812' radius-mock.log; do
    kill -0 "$radius_pid"
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 100 ]; then
        echo 'RADIUS fixture did not bind its socket'
        exit 1
    fi
    sleep 0.1
done
le test fixture l2tp/radius-acct-wire 1701
`

const radiusFixtureStatus = `if [ -f /tmp/l2tp-radius-acct-wire/status ]; then cat /tmp/l2tp-radius-acct-wire/status; else printf 'running\n'; fi`

func checkRadiusAccountingFixture(ctx context.Context, lab labOperations, timeout time.Duration) error {
	// Preserve radius-acct-wire.ci's bound: its legal minimum acct-interval
	// is 60s, and the unchanged peer's Interim-Update deadline alone is 100s.
	probeCtx, cancel := context.WithTimeout(ctx, max(timeout, 240*time.Second))
	defer cancel()
	if err := lab.ExecDetached(probeCtx, peerZe, []string{"sh", "-c", radiusFixtureWorker}, nil); err != nil {
		return fmt.Errorf("start original accounting fixtures: %w", err)
	}
	for {
		result, err := lab.Exec(probeCtx, peerZe, []string{"sh", "-c", radiusFixtureStatus}, nil)
		if err != nil {
			return fmt.Errorf("read original accounting fixture status: %w", err)
		}
		status := strings.TrimSpace(result.Stdout)
		if result.ExitCode != 0 || (status != "running" && status != "0") {
			output, _ := lab.Exec(probeCtx, peerZe, []string{commandCat, "/tmp/l2tp-radius-acct-wire/worker.log"}, nil)
			return interoplab.WithSearchedOutput(fmt.Errorf("original accounting fixtures failed with status %q (reader exit %d)", status, result.ExitCode), output.Stdout+output.Stderr)
		}
		if status == "0" {
			output, err := lab.Exec(probeCtx, peerZe, []string{commandCat, "/tmp/l2tp-radius-acct-wire/worker.log"}, nil)
			if err != nil {
				return fmt.Errorf("read original accounting fixture evidence: %w", err)
			}
			if output.ExitCode != 0 {
				return errors.New("original accounting fixture evidence reader failed")
			}
			if err := radiusAccountingFixtureEvidence(output.Stdout); err != nil {
				return interoplab.WithSearchedOutput(err, output.Stdout+output.Stderr)
			}
			if _, err := fmt.Fprintln(os.Stdout, "Original accounting fixture evidence:\n"+output.Stdout); err != nil {
				return fmt.Errorf("retain original accounting fixture evidence: %w", err)
			}
			_, _, err = interoplab.Wait(probeCtx, interoplab.WaitOptions{
				Timeout: timeout, Interval: 250 * time.Millisecond,
				Description: "original accounting session's kernel PPP interface removal",
			}, func(readCtx context.Context) ([]string, error) {
				return pppLinks(readCtx, lab, peerZe)
			}, func(links []string) bool { return len(links) == 0 })
			return err
		}
		if err := sleepContext(probeCtx, 250*time.Millisecond); err != nil {
			return fmt.Errorf("original accounting fixtures did not finish: %w", err)
		}
	}
}

func radiusAccountingFixtureEvidence(output string) error {
	for _, expected := range []string{
		"radius-mock listening on 127.0.0.1:1812",
		"OK: CHAP Response sent for challenge identifier ",
		" as subscriber@ze",
		"OK: CHAP Success received",
		"OK: IPCP opened, ze assigned 10.99.7.10",
		"OK: Access-Request NAS-Port-Id resolved from the template as lns1:",
		"OK: Accounting-Start Framed-IP-Address matches the IPCP-negotiated address 10.99.7.10",
		"OK: Accounting-Start carries the same NAS-Port-Id lns1:",
		"OK: Accounting-Start carries Event-Timestamp ",
		"OK: Accounting-Interim-Update carries Event-Timestamp ",
		"OK: peer stopped answering LCP echo probes",
		"OK: Accounting-Stop carries Event-Timestamp ",
		"and Acct-Terminate-Cause 2",
		"RADIUS-RX Access-Request ",
		"Acct-Status-Type=Start",
		"Acct-Status-Type=Interim-Update",
		"Acct-Status-Type=Stop",
	} {
		if !strings.Contains(output, expected) {
			return fmt.Errorf("original accounting fixtures lack %q", expected)
		}
	}
	return nil
}
