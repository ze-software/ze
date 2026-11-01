package fakefilter

import (
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// conformanceRoutes are the routes the harness judges: two prefixes of one
// family, each a filter-update the forked transport would carry.
func conformanceRoutes() []sdk.ConformanceRoute {
	return []sdk.ConformanceRoute{
		{Input: rpc.FilterUpdateInput{Filter: FilterName, Direction: "import", Peer: "127.0.0.1", PeerAS: 65001,
			Fields: json.RawMessage(`{"prefix":{"ipv4/unicast":["10.0.0.0/24"]}}`)}},
		{Input: rpc.FilterUpdateInput{Filter: FilterName, Direction: "import", Peer: "127.0.0.2", PeerAS: 65002,
			Fields: json.RawMessage(`{"prefix":{"ipv4/unicast":["10.1.0.0/24"]}}`)}},
	}
}

// TestFakeFilterConformance proves AC-35's fast-path half: the evaluator the
// plugin hands sdk.OnFilter, written against the SDK alone, passes the
// fast-path conformance harness (deadline, repeat, borrowed view, transport
// equality, concurrency). Method: the harness itself, under -race, with the
// stop trigger off, as a plugin runs when no test asks it to stop.
func TestFakeFilterConformance(t *testing.T) {
	judge := newJudge(0, func() {}, func(string, string) {})
	if !sdk.CheckFilterConformance(t, Name, judge.evaluate, conformanceRoutes()) {
		t.Fatal("the fakefilter evaluator breaks the fast-path contract")
	}
}

// TestFakeFilterAcceptsAndNamesThePrefix proves the verdict and the record a
// .ci reads: every route is accepted, and the judged prefix is reported.
func TestFakeFilterAcceptsAndNamesThePrefix(t *testing.T) {
	var judged []string
	judge := newJudge(0, func() {}, func(prefix, verdict string) { judged = append(judged, prefix+" "+verdict) })
	routes := conformanceRoutes()
	route := rpc.InputFilterRoute(&routes[0].Input)
	out, err := judge.evaluate(&route)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if out == nil || out.Action != rpc.FilterAccept {
		t.Fatalf("verdict = %+v, want accept", out)
	}
	if len(judged) != 1 || judged[0] != "10.0.0.0/24 accept" {
		t.Fatalf("judged = %q, want [\"10.0.0.0/24 accept\"]", judged)
	}
}

// TestFakeFilterRefusesUnreadableFields proves the evaluator does not answer a
// verdict for a route whose fields it cannot read: the member's on-error
// decides that route, not an accept invented by the fixture.
func TestFakeFilterRefusesUnreadableFields(t *testing.T) {
	judge := newJudge(0, func() {}, func(string, string) {})
	input := rpc.FilterUpdateInput{Filter: FilterName, Direction: "import", Fields: json.RawMessage(`{"prefix":`)}
	route := rpc.InputFilterRoute(&input)
	if out, err := judge.evaluate(&route); err == nil {
		t.Fatalf("evaluate answered %+v for unreadable fields, want an error", out)
	}
}

// TestFakeFilterStopTriggerFiresOnce proves the AC-33 stop trigger: with
// stop-after N, the Nth judged route is still answered (by the live plugin),
// the stop runs exactly once on it, and never before it.
func TestFakeFilterStopTriggerFiresOnce(t *testing.T) {
	var stops atomic.Int32
	judge := newJudge(2, func() { stops.Add(1) }, func(string, string) {})
	routes := conformanceRoutes()
	for i := range 4 {
		route := rpc.InputFilterRoute(&routes[i%2].Input)
		out, err := judge.evaluate(&route)
		if err != nil || out == nil || out.Action != rpc.FilterAccept {
			t.Fatalf("route %d: verdict %+v, error %v, want accept", i, out, err)
		}
		want := int32(0)
		if i >= 1 {
			want = 1
		}
		if got := stops.Load(); got != want {
			t.Fatalf("after route %d: stop ran %d times, want %d", i, got, want)
		}
	}
}

// TestFakeFilterDeclaration proves the declaration the daemon receives at
// Stage 1 names the filter a chain references (fakefilter:judge), reads the
// prefix, and fails closed: the on-error AC-33 asserts.
func TestFakeFilterDeclaration(t *testing.T) {
	decls := filterDecls()
	if len(decls) != 1 {
		t.Fatalf("declared %d filters, want 1", len(decls))
	}
	decl := decls[0]
	if decl.Name != FilterName || decl.Direction != sdk.FilterImport || decl.OnError != sdk.OnErrorReject {
		t.Fatalf("declaration = %+v, want %s import on-error reject", decl, FilterName)
	}
	if len(decl.Reads) != 1 || decl.Reads[0] != fieldPrefix {
		t.Fatalf("reads = %q, want [%q]", decl.Reads, fieldPrefix)
	}
}
