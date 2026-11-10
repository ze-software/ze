// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- the blind-detector diagnostic through show doctor
// Overview: register_ddos_detect_blind.go -- the observers registered from these functions
// Related: plugin_fixture_06.go -- fixture06DispatchObject, the show doctor read
// Related: dns_source_operator.go -- operatorPollAttempts, operatorPollDelay

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// ddosBlindExternalText is the part of the blind-detector warning that names
// the external-process cause. A finding without it (the no-interface text an
// external detector's empty samples would otherwise produce) fails the run.
const ddosBlindExternalText = "runs as an external plugin process"

// ddosBlindZeroText is the part of the blind-detector warning that names the
// zero-traffic case: interfaces are reported, and every one received nothing.
// The no-sample and no-interface texts lack it, so either fails the run.
const ddosBlindZeroText = "has reported zero received packets and bytes"

// ddosBlindQuiet is how long the zero-traffic observer stays silent before each
// show doctor read. The read itself is the obstacle: this fixture reaches the
// daemon over a TLS connection on lo, so every dispatch is received traffic and
// restarts the detector's zero run. The finding needs one baseline window of
// zero samples (baseline-window 10 times check-interval 1 in
// ddos-detect-internal-blind.ci, so 10 samples). The
// interface rate tracker and the trafficstat tick each run once a second, so
// the last packet of a read is out of the samples two seconds later at most;
// 13 seconds leaves one second of margin.
const ddosBlindQuiet = 13 * time.Second

// ddosBlindQuietReads bounds the zero-traffic observer's reads. A read can
// miss when a tick lands between its request and its answer, or when a TCP
// keepalive probe (Go's default, every 15 seconds of idle) falls inside the
// silence; a second read then sees the next run. Three reads of 13 seconds fit
// the test's 60-second timeout.
const ddosBlindQuietReads = 3

// doctorDDoSDetectBlind polls show doctor until it carries
// doctor-ddos-detect-blind with the external-process cause.
func doctorDDoSDetectBlind(ctx context.Context, p *sdk.Plugin) error {
	var last map[string]any
	found := Poll(ctx, operatorPollAttempts, operatorPollDelay, func() bool {
		data, err := fixture06DispatchObject(ctx, p, "show doctor")
		if err != nil {
			return false
		}
		last = data
		return ddosBlindReported(data, ddosBlindExternalText)
	})
	if !found {
		return fmt.Errorf("show doctor lacks %s naming %q; last answer %v",
			diagnostic.CodeDoctorDDoSDetectBlind, ddosBlindExternalText, last)
	}
	_, err := fmt.Fprintln(os.Stderr, "OK: show doctor reports doctor-ddos-detect-blind for an external ddos-detect")
	return err
}

// doctorDDoSDetectBlindZero reads show doctor after each silence until it
// carries doctor-ddos-detect-blind with the zero-traffic text. It cannot poll:
// a poll is the received traffic that keeps the finding from forming (see
// ddosBlindQuiet).
func doctorDDoSDetectBlindZero(ctx context.Context, p *sdk.Plugin) error {
	var last map[string]any
	for range ddosBlindQuietReads {
		if err := ddosBlindWaitQuiet(ctx); err != nil {
			return err
		}
		data, err := fixture06DispatchObject(ctx, p, "show doctor")
		if err != nil {
			return fmt.Errorf("show doctor: %w", err)
		}
		last = data
		if ddosBlindReported(data, ddosBlindZeroText) {
			_, err := fmt.Fprintln(os.Stderr, "OK: show doctor reports doctor-ddos-detect-blind for a detector that sees no traffic")
			return err
		}
	}
	return fmt.Errorf("show doctor lacks %s naming %q after %d reads, each after %s of silence; last answer %v",
		diagnostic.CodeDoctorDDoSDetectBlind, ddosBlindZeroText, ddosBlindQuietReads, ddosBlindQuiet, last)
}

// ddosBlindWaitQuiet holds the observer silent for ddosBlindQuiet, or answers
// the context's error when the run ends first. The wait is the precondition of
// the state under test, not a guess at how long something takes: no read can
// shorten it, because every read restarts it.
func ddosBlindWaitQuiet(ctx context.Context) error {
	timer := time.NewTimer(ddosBlindQuiet)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("waiting %s of silence before show doctor: %w", ddosBlindQuiet, ctx.Err())
	case <-timer.C:
		return nil
	}
}

// ddosBlindReported answers whether data holds the blind-detector diagnostic
// whose message contains text.
func ddosBlindReported(data map[string]any, text string) bool {
	diagnostics, ok := data["diagnostics"].([]any)
	if !ok {
		return false
	}
	for _, raw := range diagnostics {
		entry, isMap := raw.(map[string]any)
		if !isMap || entry["code"] != diagnostic.CodeDoctorDDoSDetectBlind {
			continue
		}
		if message, isString := entry["message"].(string); isString && strings.Contains(message, text) {
			return true
		}
	}
	return false
}
