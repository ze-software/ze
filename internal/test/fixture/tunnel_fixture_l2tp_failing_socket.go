// Design: docs/research/l2tpv2-ze-integration.md -- receiver goroutines and a
// failing socket (spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff, AC-1).
// Related: internal/component/l2tp/listener.go -- readLoop, the loop this
// personality proves.
// Related: internal/component/l2tp/reader_metrics.go --
// ze_l2tp_listener_read_errors_total, the counter this personality reads.
// Related: register_l2tp_failing_socket.go -- registers l2tp/failing-socket.
// Related: test/draft/l2tp/subscriber-reader-failing-socket.ci -- the only
// caller, KNOWN VACUOUS, tracked but read by no gate.
//
// NO GATED CALLER, ON PURPOSE. The one .ci file that drives this
// personality lives under test/draft/, which every recursive .ci reader
// skips (test/draft/README.md), so nothing in `./le verify current mode
// full` ever runs this function. That is not an orphan a suite forgot to
// name: the .ci drives the daemon under `strace -e inject`, and ptrace's
// own signal-trap overhead suppresses the traced daemon's CPU accounting
// and iteration rate enough to make the paced and unpaced builds
// indistinguishable, so gating it would gate a test that passes against the
// BROKEN code. This fixture's own CPU and counter reads are sound; what is
// missing is a failure-injection mechanism that does not route through
// ptrace. Supply one and both files move into the l2tp suite together (the
// .ci file's header carries the full finding and the candidate mechanism).
// Until then the entry point that reaches this code is an operator typing
// `ze-test fixture l2tp/failing-socket <metrics-port>` by hand.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// tunnelL2TPFailingSocketWindow is how long the fixture watches the
// daemon's own CPU and error counter while readLoop's socket cannot read.
// It is well above the pacer's 250ms ceiling (internal/core/pacer), so the
// paced code has settled into its steady retry rate before the window
// closes, and well below a timeout an operator would notice.
const tunnelL2TPFailingSocketWindow = 3 * time.Second

// tunnelL2TPFailingSocketMaxCoreFraction bounds the fraction of one CPU
// core the daemon may spend across the window. The paced code spends a
// small fraction of a percent of it (about a dozen syscalls in the
// window); the bare-continue bug this spec removes spends the whole core.
// The bound sits far from both, so scheduling noise under software
// emulation cannot cross it by accident in either direction.
const tunnelL2TPFailingSocketMaxCoreFraction = 0.5

// tunnelL2TPFailingSocketMaxErrors bounds how many swallowed reads the
// counter may record across the window. The pacer's ceiling caps the loop
// near four attempts a second once it reaches steady state; a busy loop
// reaches this bound in microseconds.
const tunnelL2TPFailingSocketMaxErrors = 1000

// tunnelL2TPFailingSocket proves AC-1 of
// spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff: with
// readLoop's UDP socket held in a persistently failing state (the .ci file
// wraps the daemon in `strace -e inject=recvfrom:error=ENETDOWN:when=1+`,
// so every read fails from the first one), the daemon's own CPU use over
// the window stays a small fraction of one core, and the swallowed-read
// counter rises. The log line is asserted separately, by the .ci file's own
// expect=stderr line: this fixture proves what an operator's monitoring
// would show, not what the daemon printed.
func tunnelL2TPFailingSocket(ctx context.Context, args []string) error {
	metricsPort, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	var urlBuf textbuf.Buffer
	metricsURL := urlBuf.Str("http://").HostPortN("127.0.0.1", uint16(metricsPort)).Str("/metrics").String()

	// Wait for the listener's own counter to appear before either baseline
	// is read. The value is discarded: what this call buys is the ordering.
	// Read the CPU baseline first and the series could still be minutes
	// from registering, so its poll would burn seconds that count against
	// cpuEnd-cpuStart while elapsed measures only the window below, and the
	// core fraction would read high for a reason that has nothing to do
	// with readLoop. Whether the counter has MOVED is a separate question,
	// and the delta assertion at the end of the window is what answers it.
	if _, err := tunnelL2TPMetricValue(ctx, metricsURL, "ze_l2tp_listener_read_errors_total"); err != nil {
		return fmt.Errorf("read baseline ze_l2tp_listener_read_errors_total: %w", err)
	}

	cpuStart, err := tunnelL2TPMetricValue(ctx, metricsURL, "process_cpu_seconds_total")
	if err != nil {
		return fmt.Errorf("read baseline process_cpu_seconds_total: %w", err)
	}
	errorsStart, err := tunnelL2TPMetricValue(ctx, metricsURL, "ze_l2tp_listener_read_errors_total")
	if err != nil {
		return fmt.Errorf("read baseline ze_l2tp_listener_read_errors_total: %w", err)
	}
	windowStart := time.Now()

	timer := time.NewTimer(tunnelL2TPFailingSocketWindow)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	elapsed := time.Since(windowStart).Seconds()

	cpuEnd, err := tunnelL2TPMetricValue(ctx, metricsURL, "process_cpu_seconds_total")
	if err != nil {
		return fmt.Errorf("read process_cpu_seconds_total after the window: %w", err)
	}
	errorsEnd, err := tunnelL2TPMetricValue(ctx, metricsURL, "ze_l2tp_listener_read_errors_total")
	if err != nil {
		return fmt.Errorf("read ze_l2tp_listener_read_errors_total after the window: %w", err)
	}

	coreFraction := (cpuEnd - cpuStart) / elapsed
	var tb textbuf.Buffer
	if coreFraction > tunnelL2TPFailingSocketMaxCoreFraction {
		return fmt.Errorf("readLoop spent %.4f of one core over %.1fs on a failing socket, want at most %.1f",
			coreFraction, elapsed, tunnelL2TPFailingSocketMaxCoreFraction)
	}
	tb.Str("OK: CPU use stayed at ").Float(coreFraction, 4).
		Str(" of one core over the failing-socket window\n")
	if err := tb.StdOut(); err != nil {
		return err
	}

	delta := errorsEnd - errorsStart
	if delta <= 0 {
		return fmt.Errorf("ze_l2tp_listener_read_errors_total did not rise over the window (start=%.0f, end=%.0f)",
			errorsStart, errorsEnd)
	}
	if delta > tunnelL2TPFailingSocketMaxErrors {
		return fmt.Errorf("ze_l2tp_listener_read_errors_total rose by %.0f in %.1fs, want at most %.0f: a busy loop, not a paced retry",
			delta, elapsed, float64(tunnelL2TPFailingSocketMaxErrors))
	}
	tb.Reset().Str("OK: ze_l2tp_listener_read_errors_total rose by ").Float(delta, 0).
		Str(" over the failing-socket window\n")
	return tb.StdOut()
}

// tunnelL2TPMetricValue polls metricsURL (fixture.go, Poll) and returns the
// value of the first unlabeled Prometheus sample line beginning with
// "<name> ". It polls rather than reading once because a caller may ask for
// this before the daemon's HTTP listener has necessarily finished starting.
func tunnelL2TPMetricValue(ctx context.Context, metricsURL, name string) (float64, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	prefix := name + " "
	var value float64
	var lastErr error
	ok := Poll(ctx, 20, 250*time.Millisecond, func() bool {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, http.NoBody)
		if err != nil {
			lastErr = err
			return false
		}
		response, err := client.Do(request)
		if err != nil {
			lastErr = err
			return false
		}
		defer response.Body.Close() //nolint:errcheck // the body is read to completion below
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			lastErr = err
			return false
		}
		for line := range strings.SplitSeq(string(raw), "\n") {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			parsed, parseErr := strconv.ParseFloat(fields[1], 64)
			if parseErr != nil {
				lastErr = parseErr
				return false
			}
			value = parsed
			return true
		}
		lastErr = errors.New(name + " not found in metrics output")
		return false
	})
	if !ok {
		if lastErr != nil {
			return 0, lastErr
		}
		return 0, errors.New(name + " not available")
	}
	return value, nil
}
