package fakeflow

import (
	"net/netip"
	"strconv"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/trafficfeature"
	"codeberg.org/thomas-mangin/ze/internal/core/observation"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// captureFlows swaps publishFlow for a slice-capturing stub and returns the
// captured slice plus a restore function.
func captureFlows(t *testing.T) (*[]observation.Observation, func()) {
	t.Helper()
	got := &[]observation.Observation{}
	orig := publishFlow
	publishFlow = func(obs observation.Observation) { *got = append(*got, obs) }
	return got, func() { publishFlow = orig }
}

// VALIDATES: parseInject accepts a well-formed inject request and maps every
// token to the right field (src, dst, dst-port, bytes, default count=1).
// PREVENTS: a silently misparsed injection that would publish the wrong flow
// and make the harness assert on garbage.
func TestParseInjectValid(t *testing.T) {
	p, err := parseInject([]string{"10.0.0.9", "203.0.113.5", "4444", "1000000"})
	if err != nil {
		t.Fatalf("parseInject: unexpected error: %v", err)
	}
	if p.src != netip.MustParseAddr("10.0.0.9") || p.dst != netip.MustParseAddr("203.0.113.5") {
		t.Fatalf("parseInject: src/dst = %v/%v", p.src, p.dst)
	}
	if p.dstPort != 4444 || p.bytes != 1000000 || p.count != 1 {
		t.Fatalf("parseInject: port=%d bytes=%v count=%d", p.dstPort, p.bytes, p.count)
	}
}

// VALIDATES: the optional 5th token sets the publish count.
// PREVENTS: a harness that can only inject one flow per RPC when it needs N.
func TestParseInjectCount(t *testing.T) {
	p, err := parseInject([]string{"10.0.0.9", "203.0.113.5", "80", "500", "3"})
	if err != nil {
		t.Fatalf("parseInject: unexpected error: %v", err)
	}
	if p.count != 3 {
		t.Fatalf("parseInject: count = %d, want 3", p.count)
	}
}

// VALIDATES: every malformed inject request is rejected with an error and no
// partial state.
// PREVENTS: an unbounded count wedging the daemon, or a zero/negative byte
// count that trafficfeature.ingest would silently drop (feature.go:107).
func TestParseInjectRejects(t *testing.T) {
	cases := map[string][]string{
		"too few args":    {"10.0.0.9", "203.0.113.5", "80"},
		"too many args":   {"10.0.0.9", "203.0.113.5", "80", "500", "3", "extra"},
		"bad src":         {"not-an-ip", "203.0.113.5", "80", "500"},
		"bad dst":         {"10.0.0.9", "not-an-ip", "80", "500"},
		"port over range": {"10.0.0.9", "203.0.113.5", "70000", "500"},
		"bytes zero":      {"10.0.0.9", "203.0.113.5", "80", "0"},
		"bytes negative":  {"10.0.0.9", "203.0.113.5", "80", "-5"},
		"count zero":      {"10.0.0.9", "203.0.113.5", "80", "500", "0"},
		"count over max":  {"10.0.0.9", "203.0.113.5", "80", "500", "100001"},
	}
	for name, args := range cases {
		if _, err := parseInject(args); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

// VALIDATES: runInject publishes exactly one flow observation carrying the
// KindFlow/FeatureFlowBytes shape trafficfeature.ingest requires (feature.go:103).
// PREVENTS: an injector whose records fail the ingest gate, so nothing is ever
// scored and the harness silently proves nothing.
func TestFakeflowInjectPublishes(t *testing.T) {
	got, restore := captureFlows(t)
	defer restore()

	n, err := runInject([]string{"10.0.0.9", "203.0.113.5", "4444", "1000000"})
	if err != nil {
		t.Fatalf("runInject: unexpected error: %v", err)
	}
	if n != 1 || len(*got) != 1 {
		t.Fatalf("runInject: returned %d, published %d, want 1/1", n, len(*got))
	}
	obs := (*got)[0]
	if obs.Kind != observation.KindFlow || obs.Feature != observation.FeatureFlowBytes {
		t.Fatalf("obs kind/feature = %v/%v", obs.Kind, obs.Feature)
	}
	if obs.Flow.Src != netip.MustParseAddr("10.0.0.9") || obs.Flow.Dst != netip.MustParseAddr("203.0.113.5") {
		t.Fatalf("obs flow src/dst = %v/%v", obs.Flow.Src, obs.Flow.Dst)
	}
	if obs.Flow.DstPort != 4444 || obs.Value != 1000000 {
		t.Fatalf("obs port/value = %d/%v", obs.Flow.DstPort, obs.Value)
	}
}

// VALIDATES: count N publishes N identical observations.
// PREVENTS: a harness that under-injects and never crosses the detector's
// confirm threshold.
func TestFakeflowInjectCount(t *testing.T) {
	got, restore := captureFlows(t)
	defer restore()

	n, err := runInject([]string{"10.0.0.9", "203.0.113.5", "80", "500", "3"})
	if err != nil {
		t.Fatalf("runInject: unexpected error: %v", err)
	}
	if n != 3 || len(*got) != 3 {
		t.Fatalf("runInject count: returned %d, published %d, want 3/3", n, len(*got))
	}
}

// VALIDATES: dispatchCommand routes the inject verb to runInject on success and
// surfaces a StatusError on a bad request or an unknown command.
// PREVENTS: a command that parses at the CLI layer but never reaches the
// injector, or an error that reports success.
func TestDispatchInject(t *testing.T) {
	_, restore := captureFlows(t)
	defer restore()

	status, data, err := dispatchCommand("", "request fakeflow inject", []string{"10.0.0.9", "203.0.113.5", "80", "500"}, "")
	if status != rpc.StatusDone || err != nil {
		t.Fatalf("dispatch inject: status=%s err=%v, want done/nil", status, err)
	}
	if m, ok := data.(map[string]any); !ok || m["published"] != 1 {
		t.Fatalf("dispatch inject: data=%v, want map with published=1", data)
	}

	status, _, err = dispatchCommand("", "request fakeflow inject", []string{"bad"}, "")
	if status != rpc.StatusError || err == nil {
		t.Fatalf("dispatch bad inject: status=%s err=%v, want error/non-nil", status, err)
	}

	status, _, _ = dispatchCommand("", "request fakeflow bogus", nil, "")
	if status != rpc.StatusError {
		t.Fatalf("dispatch unknown: status=%s, want error", status)
	}
}

// VALIDATES: numeric boundaries — dst-port 65535 (max) and bytes 1 (min > 0)
// and count injectMaxCount are accepted; one past each is rejected.
// PREVENTS: an off-by-one that rejects a valid max port, accepts a zero-byte
// flow trafficfeature would silently drop, or lets an unbounded count through.
func TestParseInjectBoundaries(t *testing.T) {
	if _, err := parseInject([]string{"10.0.0.9", "203.0.113.5", "65535", "1"}); err != nil {
		t.Errorf("port 65535 / bytes 1 should be valid: %v", err)
	}
	if _, err := parseInject([]string{"10.0.0.9", "203.0.113.5", "80", "500", strconv.Itoa(injectMaxCount)}); err != nil {
		t.Errorf("count injectMaxCount should be valid: %v", err)
	}
	if _, err := parseInject([]string{"10.0.0.9", "203.0.113.5", "65536", "1"}); err == nil {
		t.Errorf("port 65536 should be rejected")
	}
	if _, err := parseInject([]string{"10.0.0.9", "203.0.113.5", "80", "500", strconv.Itoa(injectMaxCount + 1)}); err == nil {
		t.Errorf("count injectMaxCount+1 should be rejected")
	}
}

// VALIDATES: an injected flow observation actually reaches a live
// trafficfeature.Service through the process-global observation.Feed -- the
// harness's core premise (fakeflow -> observation.Global() -> trafficfeature).
// PREVENTS: a harness that injects into a feed nothing consumes, so the .ci
// silently proves nothing (degraded snapshot, no incident).
func TestInjectReachesTrafficfeatureService(t *testing.T) {
	svc := trafficfeature.NewService(observation.Global())
	id := svc.Attach() // Attach() synchronously starts the service and subscribes to the feed.
	defer svc.Detach(id)

	if _, err := runInject([]string{"10.0.0.9", "203.0.113.5", "4444", "1000000"}); err != nil {
		t.Fatalf("runInject: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snap := svc.Snapshot()
		if !snap.Degraded && len(snap.Sources) > 0 {
			return // the injected flow reached trafficfeature
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("trafficfeature never ingested the injected flow (degraded snapshot)")
}
