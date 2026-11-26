// Design: docs/architecture/api/process-protocol.md -- end-to-end lossless burst delivery fixture for growable eventQueue.
// Related: register_bus_send_fixtures.go -- driver registration for plugin/bus-burst-observer and plugin/bus-burst-verify.
// Related: bus_send_fixtures.go -- busPollFile, busWriteAtomicFile, busPollAttempts, busPollDelay.
// Related: bus_scenario_fixtures.go -- busBGPReceivedUpdateDecl, busRunScenarioPlugin.

package fixture

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// burstUpdateEvent decodes the announced IPv4 unicast NLRI list of one bgp:update JSON event.
type burstUpdateEvent struct {
	Type string `json:"type"`
	BGP  struct {
		Message struct {
			Type string `json:"type"`
		} `json:"message"`
		Update struct {
			NLRI struct {
				IPv4Unicast []struct {
					Action string   `json:"action"`
					NLRI   []string `json:"nlri"`
				} `json:"ipv4/unicast"`
			} `json:"nlri"`
		} `json:"update"`
	} `json:"bgp"`
}

// buildExpectedIPv4Prefixes returns count sequential IPv4 prefixes starting at
// base, matching le test peer --mode inject (internal/test/peer/inject.go).
func buildExpectedIPv4Prefixes(base netip.Prefix, count int) ([]string, error) {
	if !base.Addr().Is4() {
		return nil, fmt.Errorf("base prefix %s must be IPv4", base)
	}
	if count <= 0 {
		return nil, fmt.Errorf("want count %d must be positive", count)
	}
	plen := base.Bits()
	baseBytes := base.Addr().As4()
	baseInt := binary.BigEndian.Uint32(baseBytes[:])
	step := uint32(1)
	if plen < 32 {
		step = uint32(1) << (32 - plen)
	}
	prefixes := make([]string, count)
	var raw [4]byte
	for i := range count {
		//nolint:gosec // i is bounded by positive test count and wraps identically to peer/inject.go
		addrInt := baseInt + uint32(i)*step
		binary.BigEndian.PutUint32(raw[:], addrInt)
		prefixes[i] = netip.PrefixFrom(netip.AddrFrom4(raw), plen).String()
	}
	return prefixes, nil
}

// extractAnnouncedIPv4Prefixes returns all announced IPv4 unicast prefixes from
// a JSON bgp:update event, or nil for non-UPDATE or empty (End-of-RIB) events.
func extractAnnouncedIPv4Prefixes(event string) ([]string, error) {
	var decoded burstUpdateEvent
	if err := json.Unmarshal([]byte(event), &decoded); err != nil {
		return nil, fmt.Errorf("decode event JSON: %w", err)
	}
	if decoded.Type != "bgp" {
		return nil, nil
	}
	if decoded.BGP.Message.Type != busSenderModeUpdate {
		return nil, nil
	}
	var prefixes []string
	for _, entry := range decoded.BGP.Update.NLRI.IPv4Unicast {
		if entry.Action != "add" {
			continue
		}
		prefixes = append(prefixes, entry.NLRI...)
	}
	return prefixes, nil
}

// burstObserverExpectedOK returns the exact success marker line for wantCount prefixes.
func burstObserverExpectedOK(wantCount int) string {
	var tb textbuf.Buffer
	return tb.Str("OK: received ").Int(int64(wantCount)).Byte('/').Int(int64(wantCount)).
		Str(" prefixes in exact FIFO order").String()
}

// busBurstObserverDriver runs an external bgp:update observer that pauses
// inside OnEvent on the first announced prefix (writing pausedFile and waiting
// for releaseFile), then verifies all wantCount prefixes arrive in exact FIFO
// order with zero drops and zero duplicates before writing doneFile.
// Usage: plugin/bus-burst-observer <paused-file> <release-file> <base-prefix> <want-count> <done-file>.
func busBurstObserverDriver(ctx context.Context, args []string) error {
	if len(args) != 5 {
		return errors.New("usage: plugin/bus-burst-observer <paused-file> <release-file> <base-prefix> <want-count> <done-file>")
	}
	pausedFile, releaseFile, baseArg, countArg, doneFile := args[0], args[1], args[2], args[3], args[4]

	basePrefix, err := netip.ParsePrefix(baseArg)
	if err != nil {
		return fmt.Errorf("parse base prefix %q: %w", baseArg, err)
	}
	wantCount, err := strconv.Atoi(countArg)
	if err != nil {
		return fmt.Errorf("parse want count %q: %w", countArg, err)
	}
	expected, err := buildExpectedIPv4Prefixes(basePrefix, wantCount)
	if err != nil {
		return err
	}

	reg := &sdk.Registration{
		Receives: []sdk.MessageReceiveDecl{busBGPReceivedUpdateDecl()},
	}

	var mu sync.Mutex
	paused := false
	doneWritten := false
	eventCount := 0
	received := make([]string, 0, wantCount)

	recordFail := func(cause error) error {
		ReportFailure(cause)
		var tb textbuf.Buffer
		_ = busWriteAtomicFile(doneFile, tb.Str("FAIL: ").Err(cause).Byte('\n').String()) //nolint:errcheck // best-effort diagnostic marker for verifier
		return cause
	}

	return busRunScenarioPlugin(ctx, "burst-observer", reg, func(p *sdk.Plugin) error {
		p.OnEvent(func(event string) error {
			prefixes, parseErr := extractAnnouncedIPv4Prefixes(event)
			if parseErr != nil {
				return recordFail(parseErr)
			}
			if len(prefixes) == 0 {
				return nil
			}

			mu.Lock()
			shouldPause := !paused
			if shouldPause {
				paused = true
			}
			mu.Unlock()

			if shouldPause {
				if err := busWriteAtomicFile(pausedFile, "paused\n"); err != nil {
					return recordFail(err)
				}
				if err := busPollFile(ctx, releaseFile); err != nil {
					return recordFail(err)
				}
			}

			mu.Lock()
			defer mu.Unlock()
			eventCount++
			for _, pfx := range prefixes {
				idx := len(received)
				if idx >= wantCount {
					return recordFail(fmt.Errorf("received extra prefix %s at index %d (want %d)", pfx, idx, wantCount))
				}
				if pfx != expected[idx] {
					return recordFail(fmt.Errorf("prefix mismatch at index %d: got %s, want %s", idx, pfx, expected[idx]))
				}
				received = append(received, pfx)
			}
			if len(received) == wantCount && !doneWritten {
				if eventCount != wantCount {
					return recordFail(fmt.Errorf("received %d prefixes in only %d update events (want %d distinct events; coalescing must not collapse the burst)", wantCount, eventCount, wantCount))
				}
				doneWritten = true
				var tb textbuf.Buffer
				if err := busWriteAtomicFile(doneFile, tb.Str(burstObserverExpectedOK(wantCount)).Byte('\n').String()); err != nil {
					return recordFail(err)
				}
			}
			return nil
		})
		return nil
	}, nil)
}

// busReadRIBReceivedCountCLI queries the daemon's received BGP RIB count via `ze cli`.
func busReadRIBReceivedCountCLI(ctx context.Context) (int, error) {
	code, stdout, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "cli", "-c", "show bgp rib | received | count | json")
	if err != nil {
		return -1, fmt.Errorf("run ze cli: %w", err)
	}
	if code != 0 {
		return -1, fmt.Errorf("ze cli exited %d: %s", code, strings.TrimSpace(stderr))
	}
	var payload struct {
		Count *int `json:"count"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return -1, fmt.Errorf("decode rib count JSON: %w (stdout=%q)", err, stdout)
	}
	if payload.Count == nil {
		return -1, fmt.Errorf("rib count JSON missing count field: %q", stdout)
	}
	return *payload.Count, nil
}

// busCheckNoDeliveryBackpressureCLI queries `show doctor | json` on the running
// daemon and fails if any plugin delivery finding reports failed event batches
// or producer backpressure waits at the eventQueue ceiling. Intentional slow-batch
// timings (from pausing the observer inside OnEvent) are allowed so long as
// zero batches failed and zero producer backpressure waits occurred.
func busCheckNoDeliveryBackpressureCLI(ctx context.Context) error {
	code, stdout, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "cli", "-c", "show doctor | json")
	if err != nil {
		return fmt.Errorf("run ze cli show doctor: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("ze cli show doctor exited %d: %s", code, strings.TrimSpace(stderr))
	}
	var payload struct {
		Diagnostics []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return fmt.Errorf("decode show doctor JSON: %w (stdout=%q)", err, stdout)
	}
	for _, d := range payload.Diagnostics {
		if d.Code != "doctor-plugin-delivery-failed" {
			continue
		}
		if strings.Contains(d.Message, "producer backpressure waits at queue ceiling") {
			return fmt.Errorf("unexpected queue-ceiling backpressure wait during burst: %s", d.Message)
		}
		if !strings.Contains(d.Message, " 0 event batches failed") {
			return fmt.Errorf("unexpected failed event batch during burst: %s", d.Message)
		}
	}
	return nil
}

// busBurstVerifyDriver coordinates the end-to-end lossless burst proof:
//  1. Waits for pausedFile so the external observer is confirmed paused in OnEvent.
//  2. Polls bgp-rib until all wantCount routes are received WHILE the observer stays paused.
//  3. Writes releaseFile to unpause the observer.
//  4. Waits for doneFile and verifies exact FIFO lossless receipt of all wantCount prefixes.
//  5. Verifies `show doctor | json` reports zero failed batches and zero queue-ceiling
//     producer backpressure waits for the burst.
//
// Usage: plugin/bus-burst-verify <paused-file> <release-file> <want-count> <done-file>.
func busBurstVerifyDriver(ctx context.Context, args []string) error {
	if len(args) != 4 {
		return errors.New("usage: plugin/bus-burst-verify <paused-file> <release-file> <want-count> <done-file>")
	}
	pausedFile, releaseFile, countArg, doneFile := args[0], args[1], args[2], args[3]
	wantCount, err := strconv.Atoi(countArg)
	if err != nil {
		return fmt.Errorf("parse want count %q: %w", countArg, err)
	}
	if wantCount <= 0 {
		return fmt.Errorf("want count %d must be positive", wantCount)
	}

	if err := busPollFile(ctx, pausedFile); err != nil {
		return fmt.Errorf("wait for observer pause: %w", err)
	}

	var gotRIB int
	var lastErr error
	reached := Poll(ctx, busPollAttempts, busPollDelay, func() bool {
		n, pollErr := busReadRIBReceivedCountCLI(ctx)
		if pollErr != nil {
			lastErr = pollErr
			return false
		}
		lastErr = nil
		gotRIB = n
		return n >= wantCount
	})
	if !reached {
		if lastErr != nil {
			return fmt.Errorf("bgp-rib did not reach %d routes while observer was paused (last err: %w)", wantCount, lastErr)
		}
		return fmt.Errorf("bgp-rib stalled at %d/%d routes while external observer was paused", gotRIB, wantCount)
	}

	if err := busWriteAtomicFile(releaseFile, "release\n"); err != nil {
		return fmt.Errorf("release observer: %w", err)
	}

	if err := busPollFile(ctx, doneFile); err != nil {
		return fmt.Errorf("wait for observer completion: %w", err)
	}
	raw, err := os.ReadFile(doneFile) //nolint:gosec // test scratch file in runner workdir
	if err != nil {
		return fmt.Errorf("read %s: %w", doneFile, err)
	}
	gotLine := strings.TrimSpace(string(raw))
	wantLine := burstObserverExpectedOK(wantCount)
	if gotLine != wantLine {
		return fmt.Errorf("observer verdict %q, want %q", gotLine, wantLine)
	}
	if err := busCheckNoDeliveryBackpressureCLI(ctx); err != nil {
		return err
	}

	var tb textbuf.Buffer
	return tb.Str("OK: bgp-rib reached ").Int(int64(gotRIB)).
		Str(" routes while observer paused; observer received ").
		Int(int64(wantCount)).Byte('/').Int(int64(wantCount)).
		Str(" prefixes in exact FIFO order\n").StdOut()
}
