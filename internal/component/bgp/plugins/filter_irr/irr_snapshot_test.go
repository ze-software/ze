package filter_irr

import (
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// irrRoute is one route as the chain hands it to the evaluator, carrying the
// route fields the filter reads.
func irrRoute(fields []byte) rpc.FilterRoute {
	return rpc.InputFilterRoute(&rpc.FilterUpdateInput{
		Filter: "65001", Direction: "import", Peer: readmitTestPeer, PeerAS: 65001, Fields: fields,
	})
}

// evaluateWithin calls the evaluator on its own goroutine and fails the test
// when it has not answered within bound: the evaluator never waits.
func evaluateWithin(t *testing.T, plug *irrPlugin, fields []byte, bound time.Duration) *sdk.FilterUpdateOutput {
	t.Helper()
	answered := make(chan *sdk.FilterUpdateOutput, 1)
	go func() {
		route := irrRoute(fields)
		out, err := plug.evaluate(&route)
		if err != nil {
			t.Errorf("evaluate(%s): %v", fields, err)
		}
		answered <- out
	}()
	select {
	case out := <-answered:
		return out
	case <-time.After(bound):
		t.Fatalf("evaluate(%s) did not answer within %v: the evaluator waited", fields, bound)
		return nil
	}
}

// VALIDATES: AC-32 at the evaluator. An UPDATE that arrives while its ASN's
// first lookup is still in flight is judged at once: the announced prefix
// takes the filter's on-error reject, the withdrawal in the same UPDATE is
// kept, and the prefix is held; once the lookup publishes the list, the
// readmitter asks the peer for its routes again and the re-sent prefix is
// accepted.
// PREVENTS: the evaluator blocking an engine goroutine on unresolved data
// (R-8), the wait firstResolveWait bounded before D-12.
func TestIRRUnresolvedTakesOnError(t *testing.T) {
	release := make(chan struct{})
	addr := fakeIRRv4Hook(t, map[string]string{"AS-TEST": "10.0.0.0/24"}, func(string) { <-release })
	engine := newFakeReadmitEngine()
	plug := readmitPlugin(t, addr, engine, time.Hour)

	resolved := make(chan struct{})
	go func() {
		plug.initialResolve()
		close(resolved)
	}()

	// The lookup is held by the server until release, longer than the bound:
	// the old evaluator waited on it, this one must not.
	out := evaluateWithin(t, plug, routeFields(map[string][]string{"ipv4/unicast": {"10.0.0.0/24"}},
		map[string][]string{"ipv4/unicast": {"198.51.100.0/24"}}), time.Second)
	if out.Action != sdk.FilterModify {
		t.Fatalf("unresolved verdict = %v, want modify keeping the withdrawal alone", out.Action)
	}
	if got := removedValues(t, out); !slices.Equal(got, []string{`"10.0.0.0/24"`}) {
		t.Fatalf("removes = %v, want the unjudged announcement 10.0.0.0/24 alone", got)
	}
	if got := plug.readmit.heldCount(); got != 1 {
		t.Fatalf("held = %d, want the rejected announcement held", got)
	}

	close(release)
	select {
	case <-resolved:
	case <-time.After(perASNRefreshTimeout + 10*time.Second):
		t.Fatal("the first lookup did not complete within the product's refresh bound")
	}
	select {
	case sent := <-engine.sent:
		if sent.peer != netip.MustParseAddr(readmitTestPeer) || sent.family != "ipv4/unicast" {
			t.Fatalf("ROUTE-REFRESH sent for %v, want %s ipv4/unicast", sent, readmitTestPeer)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ROUTE-REFRESH after the list was published")
	}
	if out := evaluateWithin(t, plug, announceFields("10.0.0.0/24"), time.Second); out.Action != sdk.FilterAccept {
		t.Fatalf("re-sent in-list prefix = %v, want accept", out.Action)
	}
}

// VALIDATES: AC-34. A config reload that changes the ASN's as-set swaps the
// snapshot while UPDATEs are judged on other goroutines, and every route is
// judged wholly by one snapshot: the old list authorizes both prefixes and
// the new config has none, so the only answers are accept (old) and reject
// (new). A modify would mean one route read two lists. Run under -race, the
// swap and the reads share no unsynchronized state.
// PREVENTS: an evaluator reading byASN beside a reload that replaces it.
func TestIRRSnapshotSwap(t *testing.T) {
	tx := newTestTx(t)
	both := &irrPrefixList{entries: mustPrefixListFromIRR(t, irr.PrefixList{IPv4: []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"), netip.MustParsePrefix("10.1.0.0/24"),
	}})}
	resolveAs := func(asSet string) {
		if err := tx.Verify(irrSection(asSet)); err != nil {
			t.Errorf("verify %s: %v", asSet, err)
			return
		}
		if err := tx.Apply(); err != nil {
			t.Errorf("apply %s: %v", asSet, err)
			return
		}
		if asSet != "AS-OLD" {
			return
		}
		// AS-OLD resolves to both prefixes, published as refreshASNCtx does.
		tx.plug.mu.Lock()
		tx.plug.byASN[65001].list = both
		tx.plug.publishLocked()
		tx.plug.mu.Unlock()
	}
	if err := tx.Configure(irrSection("AS-NEW")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	resolveAs("AS-OLD")

	var wg sync.WaitGroup
	stop := make(chan struct{})
	var accepted, rejected sync.Map
	for worker := range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				route := irrRoute(announceFields("10.0.0.0/24", "10.1.0.0/24"))
				out, err := tx.plug.evaluate(&route)
				if err != nil {
					t.Errorf("evaluate: %v", err)
					return
				}
				switch out.Action {
				case sdk.FilterAccept:
					accepted.Store(worker, true)
				case sdk.FilterReject:
					rejected.Store(worker, true)
				case sdk.FilterModify, sdk.FilterUnspecified:
					t.Errorf("verdict %v %+v: one route was judged by two snapshots", out.Action, out.Sets)
					return
				}
			}
		})
	}
	for i := range 50 {
		if i%2 == 0 {
			resolveAs("AS-NEW")
			// The reload itself swapped the snapshot: AS-NEW has no list yet.
			route := irrRoute(announceFields("10.0.0.0/24", "10.1.0.0/24"))
			if out, err := tx.plug.evaluate(&route); err != nil || out.Action != sdk.FilterReject {
				t.Fatalf("after the reload to AS-NEW: %v %v, want reject: the reload did not swap the snapshot", out, err)
			}
		} else {
			resolveAs("AS-OLD")
		}
	}
	close(stop)
	wg.Wait()

	// After the last reload (AS-OLD, resolved) the snapshot is the new one.
	route := irrRoute(announceFields("10.0.0.0/24", "10.1.0.0/24"))
	if out, err := tx.plug.evaluate(&route); err != nil || out.Action != sdk.FilterAccept {
		t.Fatalf("after the last swap: %v %v, want accept by AS-OLD's list", out, err)
	}
}

// VALIDATES: D-10. The Stage 1 declaration covers exactly the filter names
// extractASNFromFilter reads as an AS number, so a reference whose suffix is
// not one fails config load against the pattern.
func TestIRRNamePatternCoversTheASNs(t *testing.T) {
	declared, err := registry.CompileFilterDeclarations("bgp-filter-irr", filterDecls())
	if err != nil {
		t.Fatalf("declaration refused: %v", err)
	}
	pattern := regexp.MustCompile("^(?:" + asnFilterNamePattern + ")$")
	for _, name := range []string{"1", "65001", "4199999999", "4294967294", "4294967295", "999999999"} {
		if _, ok := registry.FindDeclaredFilter(declared, name); !ok {
			t.Errorf("pattern refuses %q, an AS number", name)
		}
		if extractASNFromFilter(name) == 0 {
			t.Errorf("extractASNFromFilter(%q) = 0 for a name the pattern covers", name)
		}
	}
	for _, name := range []string{"0", "abc", "065001", "4294967296", "9999999999", "65001a", "", "AS65001"} {
		if pattern.MatchString(name) {
			t.Errorf("pattern covers %q, which is no AS number", name)
		}
	}
	for i := range 64 {
		name := strings.Repeat("9", i%11+1)
		covered := pattern.MatchString(name)
		if covered != (extractASNFromFilter(name) != 0) {
			t.Errorf("%q: pattern covers=%v, extractASNFromFilter=%d", name, covered, extractASNFromFilter(name))
		}
	}
}
