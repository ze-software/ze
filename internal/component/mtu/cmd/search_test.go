// Design: docs/architecture/diagnostics/path-mtu.md -- the search, table-tested over a scripted path
//
// VALIDATES: AC-7 (unmeasurable by name), AC-8 (a reported figure is
// confirmed on the wire), AC-9 (ladder then bisection under filtered ICMP),
// AC-10 (a single silence moves no bound, RFC 4821 Section 7.6.4, MAX_PROBES
// 3 of RFC 8899 Section 5.1.2), AC-11 (exhaustive discards the report), and R-2
// (the probe budget is the computed worst case).
// PREVENTS: a search that believes a router's figure unconfirmed, walks its
// estimate down under a lost probe, reads a zero Next-Hop MTU as an MTU, or
// runs past a bound nobody computed.
//
// Every test here drives searchPathMTU over a prober that is a model of a
// path, so no socket is opened and no privilege is needed. The real prober
// is proven against a Linux router in search_integration_linux_test.go.

package cmd

import (
	"context"
	"math/bits"
	"net/netip"
	"slices"
	"testing"
)

// fakePath models one path for the search. Sizes are wire octets. A probe
// above ifaceMTU is refused locally reporting ifaceMTU, as the sender's
// kernel does. A probe above pathMTU is refused by the router, reporting
// pathMTU (or reportedValue when set, the router that lies), or nothing
// when reportsMTU is false (the unmodified router of RFC 1191 Section 5),
// or silently when filtered is true (the ICMP error never arrives). A dead
// path answers nothing at any size. silences names, per payload, how many
// probes of that size are lost before one is answered. scripted answers,
// when present, are consumed first in order.
type fakePath struct {
	overhead      int
	ifaceMTU      int
	pathMTU       int
	reportedValue int
	reportsMTU    bool
	filtered      bool
	dead          bool
	silences      map[int]int
	scripted      []probeAnswer
	sent          []int
}

func (f *fakePath) probe(_ context.Context, payload int) (probeAnswer, error) {
	f.sent = append(f.sent, payload)
	if len(f.scripted) > 0 {
		ans := f.scripted[0]
		f.scripted = f.scripted[1:]
		return ans, nil
	}
	if f.dead {
		return probeAnswer{outcome: probeSilent}, nil
	}
	if f.silences[payload] > 0 {
		f.silences[payload]--
		return probeAnswer{outcome: probeSilent}, nil
	}
	wire := payload + f.overhead
	if f.ifaceMTU > 0 {
		if wire > f.ifaceMTU {
			return probeAnswer{outcome: probeRefusedReported, local: true, mtu: uint32(f.ifaceMTU)}, nil
		}
	}
	if wire <= f.pathMTU {
		return probeAnswer{outcome: probeReplied}, nil
	}
	if f.filtered {
		return probeAnswer{outcome: probeSilent}, nil
	}
	if !f.reportsMTU {
		return probeAnswer{outcome: probeRefusedUnreported}, nil
	}
	value := f.pathMTU
	if f.reportedValue != 0 {
		value = f.reportedValue
	}
	return probeAnswer{outcome: probeRefusedReported, mtu: uint32(value)}, nil
}

var (
	searchTarget4 = netip.MustParseAddr("192.0.2.1")
	searchTarget6 = netip.MustParseAddr("2001:db8::1")
)

// clampedFake is the common case: a 1500 interface and a router clamped to
// 1400 that reports it.
func clampedFake() *fakePath {
	return &fakePath{overhead: icmpOverheadIPv4, ifaceMTU: 1500, pathMTU: 1400, reportsMTU: true}
}

func mustMeasure(t *testing.T, res searchResult, err error, wantMTU uint32, wantMethod searchMethod) {
	t.Helper()
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.outcome != searchMeasured {
		t.Fatalf("outcome %v, want measured", res.outcome)
	}
	if res.pathMTU != wantMTU {
		t.Errorf("path MTU %d, want %d", res.pathMTU, wantMTU)
	}
	if res.method != wantMethod {
		t.Errorf("method %q, want %q", res.method, wantMethod)
	}
	if res.probes != len(res.records) {
		t.Errorf("probes %d but %d records", res.probes, len(res.records))
	}
}

// TestSearchDFGateFailed: a path that answers a 10000-octet DF payload is
// not honoring DF, so no figure is believed and the run says so by name.
func TestSearchDFGateFailed(t *testing.T) {
	path := &fakePath{overhead: icmpOverheadIPv4, pathMTU: 70000}
	res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.outcome != searchDFGateFailed {
		t.Fatalf("outcome %v, want df-gate-failed", res.outcome)
	}
	if res.probes != 1 {
		t.Errorf("probes %d, want the gate alone", res.probes)
	}
	if res.pathMTU != 0 {
		t.Errorf("a failed gate carried a path MTU %d", res.pathMTU)
	}
}

// TestReportedMTUConfirmedOnTheWire (AC-8): a router's figure is the answer
// only once the size passes and one octet more fails. A router that
// over-reports or under-reports is caught by that pair and the search
// takes over, naming which way the report was wrong.
func TestReportedMTUConfirmedOnTheWire(t *testing.T) {
	t.Run("confirmed", func(t *testing.T) {
		path := clampedFake()
		res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
		mustMeasure(t, res, err, 1400, searchViaICMP)
		// gate (local 1500), 1472 refused reporting 1400, then the pair.
		want := []int{sanityPayload, 1500 - icmpOverheadIPv4, 1400 - icmpOverheadIPv4, 1400 - icmpOverheadIPv4 + 1}
		if !equalInts(path.sent, want) {
			t.Errorf("probes sent %v, want %v", path.sent, want)
		}
	})
	t.Run("over-reported", func(t *testing.T) {
		path := clampedFake()
		path.pathMTU = 1300
		path.reportedValue = 1400
		res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
		mustMeasure(t, res, err, 1300, searchICMPOverReported)
	})
	t.Run("under-reported", func(t *testing.T) {
		path := clampedFake()
		path.reportedValue = 1300
		res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
		mustMeasure(t, res, err, 1400, searchICMPUnderReported)
		if !containsInt(path.sent, 1300-icmpOverheadIPv4+1) {
			t.Errorf("the reported 1300 was never tested one octet above: %v", path.sent)
		}
	})
	t.Run("ipv6 overhead", func(t *testing.T) {
		path := &fakePath{overhead: icmpOverheadIPv6, ifaceMTU: 1500, pathMTU: 1400, reportsMTU: true}
		res, err := searchPathMTU(context.Background(), path, searchTarget6, false)
		mustMeasure(t, res, err, 1400, searchViaICMP)
		if !containsInt(path.sent, 1400-icmpOverheadIPv6) {
			t.Errorf("the IPv6 confirm never sent a %d payload: %v", 1400-icmpOverheadIPv6, path.sent)
		}
	})
}

// TestSearchLadderThenBisect (AC-9): with every ICMP error filtered the
// candidates inside the bracket are tried before the number line, the
// value is still found, and the method says it came from a search.
func TestSearchLadderThenBisect(t *testing.T) {
	path := clampedFake()
	path.filtered = true
	res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
	mustMeasure(t, res, err, 1400, searchICMPFiltered)

	// After the gate and the failed confirm of the interface MTU, every
	// probe is a ladder candidate until the first one that is not, and at
	// least one candidate and one number-line probe were sent.
	afterConfirm := path.sent[2:]
	ladderProbes := 0
	for _, payload := range afterConfirm {
		if !isCandidatePayload(payload, icmpOverheadIPv4) {
			break
		}
		ladderProbes++
	}
	if ladderProbes == 0 {
		t.Fatalf("no ladder candidate was tried first: %v", afterConfirm)
	}
	if ladderProbes == len(afterConfirm) {
		t.Fatalf("the ladder never handed over to the number line: %v", afterConfirm)
	}
	for _, payload := range afterConfirm[ladderProbes:] {
		if isCandidatePayload(payload, icmpOverheadIPv4) {
			t.Errorf("candidate %d tried after the number line started: %v", payload, afterConfirm)
		}
	}
	if res.probes > runProbeBudget {
		t.Errorf("probes %d exceed the budget %d", res.probes, runProbeBudget)
	}
}

// TestSearchDoesNotMoveBoundsOnSingleSilence (AC-10, RFC 4821 Section
// 7.6.4, RFC 8899 Section 5.1.2): a size that loses one or two probes is
// retried and still believed to pass, so the answer is unchanged; the
// third silence is the one that is believed, and the answer then drops by
// one octet. The literal 3 is the Boundary row (probes per size 1..3):
// MUST NOT be read from the constant, or the test would follow a wrong one.
func TestSearchDoesNotMoveBoundsOnSingleSilence(t *testing.T) {
	if probesPerSizeMax != 3 {
		t.Fatalf("probesPerSizeMax = %d, RFC 8899 Section 5.1.2 says MAX_PROBES is 3", probesPerSizeMax)
	}
	confirmPayload := 1400 - icmpOverheadIPv4
	for silences := 1; silences < 3; silences++ {
		path := clampedFake()
		path.silences = map[int]int{confirmPayload: silences}
		res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
		mustMeasure(t, res, err, 1400, searchViaICMP)
		if res.probes != 4+silences {
			t.Errorf("%d silences: probes %d, want %d", silences, res.probes, 4+silences)
		}
	}
	t.Run("third silence is believed", func(t *testing.T) {
		path := clampedFake()
		path.silences = map[int]int{confirmPayload: 3}
		res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
		mustMeasure(t, res, err, 1399, searchICMPOverReported)
	})
	// On a filtered path the bracket IS consulted after the silence: a
	// bound moved on the first lost probe at 1400 would sit at the size
	// the retry then passes, contradict it, and mark the run lossy.
	t.Run("bounds untouched on the ladder", func(t *testing.T) {
		path := clampedFake()
		path.filtered = true
		path.silences = map[int]int{confirmPayload: 2}
		res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
		mustMeasure(t, res, err, 1400, searchICMPFiltered)
		if res.lossy {
			t.Errorf("two lost probes at 1400 moved a bound and contradicted the retry: %v", path.sent)
		}
		// gate 1, confirm 3, ladder 1430 3 + 1350 1 + 1400 3 + 1420 3 +
		// 1412 3, number line 1378 3 + 1375 3 + 1373 3.
		if res.probes != 26 {
			t.Errorf("probes %d, want 26: %v", res.probes, path.sent)
		}
	})
}

// TestSearchUnmeasurable (AC-7): a path that answers nothing at any size is
// unmeasurable by name, and the whole run costs exactly the silent worst
// case of the gate, the ladder and the floor probe, inside the budget.
func TestSearchUnmeasurable(t *testing.T) {
	path := &fakePath{overhead: icmpOverheadIPv4, dead: true}
	res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.outcome != searchUnmeasurable {
		t.Fatalf("outcome %v, want unmeasurable", res.outcome)
	}
	if res.pathMTU != 0 {
		t.Errorf("an unmeasurable path carried a path MTU %d", res.pathMTU)
	}
	wantProbes := probesPerSizeMax * (1 + ladderAttemptsMax + 1)
	if res.probes != wantProbes {
		t.Errorf("probes %d, want %d (gate, %d ladder attempts, floor, each %d probes)", res.probes, wantProbes, ladderAttemptsMax, probesPerSizeMax)
	}
	if res.probes > runProbeBudget {
		t.Errorf("probes %d exceed the budget %d", res.probes, runProbeBudget)
	}
	if !containsInt(path.sent, payloadMin) {
		t.Errorf("the floor probe at %d was never sent: %v", payloadMin, path.sent)
	}
}

// TestSearchBudgetIsTheComputedWorstCase: the budget in the code is the
// sum R-2 promises, so a change to a ladder or a range that widens a loop
// changes the constant or this test goes red.
func TestSearchBudgetIsTheComputedWorstCase(t *testing.T) {
	if got := bits.Len(uint(len(candidateLadder))); got != ladderAttemptsMax {
		t.Errorf("ladder of %d candidates bisects in %d attempts, constant says %d", len(candidateLadder), got, ladderAttemptsMax)
	}
	if got := bits.Len(uint(payloadMax + 1)); got != bisectionAttemptsMax {
		t.Errorf("number line of %d bisects in %d attempts, constant says %d", payloadMax+1, got, bisectionAttemptsMax)
	}
	want := probesPerSizeMax * (1 + 2*reportedFollowsMax + ladderAttemptsMax + 1 + 1 + bisectionAttemptsMax)
	if runProbeBudget != want {
		t.Errorf("budget %d, want %d", runProbeBudget, want)
	}
}

// TestSearchExhaustiveDiscardsTheReport (AC-11): exhaustive discards the reported
// value and the bracket, searches the full range, and says so.
func TestSearchExhaustiveDiscardsTheReport(t *testing.T) {
	path := clampedFake()
	res, err := searchPathMTU(context.Background(), path, searchTarget4, true)
	mustMeasure(t, res, err, 1400, searchForcedFullSearch)
	// The interface MTU the gate reported is never confirmed.
	if len(path.sent) > 1 {
		if path.sent[1] == 1500-icmpOverheadIPv4 {
			t.Errorf("exhaustive followed the gate's reported 1500: %v", path.sent)
		}
	}
}

// TestSearchZeroReportIsTheSearchSignal (RFC 1191 Section 5): a refusal
// that reports no value is a definitive failure that starts the search; it
// is never read as an MTU and the answer still comes out.
func TestSearchZeroReportIsTheSearchSignal(t *testing.T) {
	path := clampedFake()
	path.reportsMTU = false
	res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
	mustMeasure(t, res, err, 1400, searchICMPFiltered)
	if containsInt(path.sent, 0) {
		t.Errorf("a zero-sized probe was sent: %v", path.sent)
	}
}

// TestSearchFollowIsCapped (Boundary row: follows 0..6): a router that
// hands back a new figure on every confirm is followed six times, then
// the search takes over and the method says the report kept changing.
func TestSearchFollowIsCapped(t *testing.T) {
	path := clampedFake()
	// Each confirm's first probe is refused reporting a value 10 below the
	// one being confirmed, so no report is ever confirmed.
	var scripted []probeAnswer
	scripted = append(scripted, probeAnswer{outcome: probeRefusedReported, local: true, mtu: 1500})
	reported := 1500
	for range reportedFollowsMax {
		reported -= 10
		scripted = append(scripted, probeAnswer{outcome: probeRefusedReported, mtu: uint32(reported)})
	}
	path.scripted = scripted
	res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
	mustMeasure(t, res, err, 1400, searchICMPKeptChanging)
	if res.probes < 1+reportedFollowsMax {
		t.Errorf("probes %d, want at least the gate and %d follows", res.probes, reportedFollowsMax)
	}
}

// TestSearchContradictionIsRetestedAndNotedLossy: a path that answers a
// large probe and refuses a smaller one contradicts itself, which a lossy
// or a lying path produces. The passed size is re-tested rather than a size
// known to fail reported, the bracket restarts from the floor when the
// re-test fails too, and the result is marked lossy. The bogus 60000
// report is what the network can say; the bracket it leaves is what the
// search must survive.
func TestSearchContradictionIsRetestedAndNotedLossy(t *testing.T) {
	path := clampedFake()
	path.scripted = []probeAnswer{
		{outcome: probeRefusedReported, mtu: 60000}, // gate refused, a bogus report
		{outcome: probeReplied},                     // 59972 passes: low above high
		{outcome: probeReplied},                     // 59973 passes too: under-reported
		{outcome: probeRefusedUnreported},           // the re-test of 59973 fails
	}
	res, err := searchPathMTU(context.Background(), path, searchTarget4, false)
	mustMeasure(t, res, err, 1400, searchICMPUnderReported)
	if !res.lossy {
		t.Errorf("a contradicted bracket was not marked lossy")
	}
	if path.sent[3] != 60000-icmpOverheadIPv4+1 {
		t.Errorf("probe after the contradiction was %d, want the re-test of %d: %v", path.sent[3], 60000-icmpOverheadIPv4+1, path.sent)
	}
	for _, payload := range path.sent {
		if payload < payloadMin {
			t.Errorf("a probe below the floor was sent: %v", path.sent)
		}
	}
}

// TestSearchMethodNames: the seven meanings the ported tool prints, and a
// zero that is never one of them.
func TestSearchMethodNames(t *testing.T) {
	want := map[searchMethod]string{
		searchViaICMP:           "via ICMP",
		searchViaLocalIfaceMTU:  "via local iface MTU",
		searchICMPUnderReported: "ICMP under-reported",
		searchICMPOverReported:  "ICMP over-reported",
		searchICMPFiltered:      "ICMP filtered",
		searchICMPKeptChanging:  "ICMP kept changing",
		searchForcedFullSearch:  "forced full search",
	}
	for method, name := range want {
		if method.String() != name {
			t.Errorf("%d.String() = %q, want %q", method, method.String(), name)
		}
	}
	defer func() {
		if recover() == nil {
			t.Errorf("searchMethodUnspecified.String() did not panic")
		}
	}()
	_ = searchMethodUnspecified.String()
}

func isCandidatePayload(payload, overhead int) bool {
	for _, c := range candidateLadder {
		if c-overhead == payload {
			return true
		}
	}
	return false
}

func containsInt(list []int, v int) bool {
	return slices.Contains(list, v)
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
