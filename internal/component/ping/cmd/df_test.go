// Design: docs/architecture/diagnostics/active-probes.md -- the DF keyword reaches the probe socket

package cmd

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/probe"
)

// recordingOpener replaces openProbeConn for one test: it records the DF mode
// the handler asked for and hands back the fake conn, so the test reaches the
// socket constructor through the real command path with no CAP_NET_RAW.
func recordingOpener(t *testing.T, conn pingConn) *probe.DFMode {
	t.Helper()
	got := new(probe.DFMode)
	realOpener := openProbeConn
	openProbeConn = func(_ context.Context, _ probe.Family, _ netip.Addr, df probe.DFMode) (pingConn, error) {
		*got = df
		return conn, nil
	}
	t.Cleanup(func() { openProbeConn = realOpener })
	return got
}

// TestPingDoNotFragmentReachesTheSocketOption is the wiring test for
// `show ping <dest> do-not-fragment honor-cache`: the value word resolves to
// probe.DFHonorCache and that mode is what the socket constructor receives.
// The fake conn answers the one probe so the batch completes on the real
// clock. Before the wiring, the keyword was swallowed by the destination
// branch and the constructor saw DFOff.
func TestPingDoNotFragmentReachesTheSocketOption(t *testing.T) {
	fc := newFakePingConn(clock.RealClock{})
	got := recordingOpener(t, fc)
	go func() {
		w := <-fc.wrote
		fc.injectReply(testPID(), w.seq)
	}()

	resp, err := handleShowPing(nil, []string{"192.0.2.1", "count", "1", "timeout", "1s", probe.DFKeyword, "honor-cache"})
	if err != nil {
		t.Fatalf("handleShowPing: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("show ping status = %q (%s), want done", resp.Status, resp.Error)
	}
	if *got != probe.DFHonorCache {
		t.Fatalf("socket constructor received DF mode %v, want %v", *got, probe.DFHonorCache)
	}
}

// fixedPathMTU replaces readPathMTU for one test with a fixed answer, or
// with the named absence when mtu is zero, so the summary key is proven
// without a UDP connect whose answer is the host's route.
func fixedPathMTU(t *testing.T, mtu uint32) {
	t.Helper()
	realRead := readPathMTU
	readPathMTU = func(_ context.Context, _ netip.Addr) (uint32, error) {
		if mtu == 0 {
			return 0, probe.ErrPathMTUUnknown
		}
		return mtu, nil
	}
	t.Cleanup(func() { readPathMTU = realRead })
}

// TestPingDoNotFragmentSummaryCarriesTheKernelEstimate VALIDATES AC-6 at
// the entry point: a DF run's summary carries path-mtu, the estimate the
// kernel holds for the destination once the probes have run, and a run
// whose kernel holds none carries no key rather than a zero. A run without
// the keyword carries no key either: the estimate is read for a DF run only.
func TestPingDoNotFragmentSummaryCarriesTheKernelEstimate(t *testing.T) {
	answerOne := func(fc *fakePingConn) {
		w := <-fc.wrote
		fc.injectReply(testPID(), w.seq)
	}
	cases := []struct {
		name string
		args []string
		mtu  uint32
		want any
	}{
		{"df-run-reports-the-estimate", []string{"192.0.2.1", "count", "1", "timeout", "1s", probe.DFKeyword, "bypass-cache"}, 1400, 1400},
		{"df-run-with-no-estimate-carries-no-key", []string{"192.0.2.1", "count", "1", "timeout", "1s", probe.DFKeyword, "honor-cache"}, 0, nil},
		{"run-without-the-keyword-carries-no-key", []string{"192.0.2.1", "count", "1", "timeout", "1s"}, 1400, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc := newFakePingConn(clock.RealClock{})
			recordingOpener(t, fc)
			fixedPathMTU(t, tc.mtu)
			go answerOne(fc)

			resp, err := handleShowPing(nil, tc.args)
			if err != nil {
				t.Fatalf("handleShowPing: %v", err)
			}
			if resp.Status != plugin.StatusDone {
				t.Fatalf("show ping status = %q (%s), want done", resp.Status, resp.Error)
			}
			data, ok := resp.Data.(plugin.Map)
			if !ok {
				t.Fatalf("show ping data is %T, want plugin.Map", resp.Data)
			}
			got, present := data[fieldPathMTU]
			if tc.want == nil {
				if present {
					t.Fatalf("%s = %v, want the key absent", fieldPathMTU, got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("%s = %v, want %v", fieldPathMTU, got, tc.want)
			}
		})
	}
}

// TestPingDoNotFragmentWithoutValueIsRefused proves the handler agrees with
// the RPC layer, which reads every leaf as keyword-then-value: the keyword
// standing bare, trailing or before another keyword, and a word that is not
// one of the two values are each refused by name before any socket opens.
func TestPingDoNotFragmentWithoutValueIsRefused(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"trailing", []string{"192.0.2.1", "count", "1", probe.DFKeyword}, "requires a value"},
		{"before-keyword", []string{"192.0.2.1", probe.DFKeyword, "count", "1"}, "honor-cache or bypass-cache"},
		{"unknown-value", []string{"192.0.2.1", probe.DFKeyword, "always"}, "honor-cache or bypass-cache"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc := newFakePingConn(clock.RealClock{})
			got := recordingOpener(t, fc)
			resp, err := handleShowPing(nil, tc.args)
			if err != nil {
				t.Fatalf("handleShowPing: %v", err)
			}
			if resp.Status != plugin.StatusError || !strings.Contains(resp.Error, tc.want) {
				t.Fatalf("show ping = %q (%s), want an error naming %q", resp.Status, resp.Error, tc.want)
			}
			if *got != probe.DFUnspecified {
				t.Fatalf("socket constructor was reached with DF mode %v, want no socket", *got)
			}
		})
	}
}

// TestPingDoNotFragmentBypassCacheReachesTheSocketOption proves the explicit
// value word after the keyword selects the bypass mode on the same path.
func TestPingDoNotFragmentBypassCacheReachesTheSocketOption(t *testing.T) {
	fc := newFakePingConn(clock.RealClock{})
	got := recordingOpener(t, fc)
	go func() {
		w := <-fc.wrote
		fc.injectReply(testPID(), w.seq)
	}()

	resp, err := handleShowPing(nil, []string{"192.0.2.1", "count", "1", "timeout", "1s", probe.DFKeyword, "bypass-cache"})
	if err != nil {
		t.Fatalf("handleShowPing: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("show ping status = %q (%s), want done", resp.Status, resp.Error)
	}
	if *got != probe.DFBypassCache {
		t.Fatalf("socket constructor received DF mode %v, want %v", *got, probe.DFBypassCache)
	}
}

// TestPingWithoutDoNotFragmentOpensWithDFOff proves AC-1 at the constructor:
// a command that never names the keyword asks for DFOff explicitly, never the
// zero mode, so the probe of before is the probe still sent.
func TestPingWithoutDoNotFragmentOpensWithDFOff(t *testing.T) {
	fc := newFakePingConn(clock.RealClock{})
	got := recordingOpener(t, fc)
	go func() {
		w := <-fc.wrote
		fc.injectReply(testPID(), w.seq)
	}()

	resp, err := handleShowPing(nil, []string{"192.0.2.1", "count", "1", "timeout", "1s"})
	if err != nil {
		t.Fatalf("handleShowPing: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("show ping status = %q (%s), want done", resp.Status, resp.Error)
	}
	if *got != probe.DFOff {
		t.Fatalf("socket constructor received DF mode %v, want %v", *got, probe.DFOff)
	}
}

// Identifier is the fake's echo identifier: the tests craft replies and
// refusals with testPID, so the session must match on the same value.
func (fc *fakePingConn) Identifier() uint16 { return testPID() }
