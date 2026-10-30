package policy

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The test vocabulary: one RIB import attachment on root "test-rib-src", a
// filter type with a route evaluator (rejects 192.0.2.0/24, sets tag 7 on
// everything else), one declared type with no evaluator, and one that reads a
// field the attachment does not carry.
const (
	testRoot     = "test-rib-src"
	testAttach   = "test-rib-src import"
	testProtocol = "test-rib-src-proto"
)

var testBogon = netip.MustParsePrefix("192.0.2.0/24")

func init() {
	MustRegisterAttachment(testRoot, testAttach,
		[]Field{FieldPrefix, FieldFamily, FieldTag}, []Field{FieldDistance, FieldTag})
	MustRegisterRIBImport(RIBImport{Attachment: testAttach, Root: testRoot, Scope: RIBScopeSource, Protocol: testProtocol})
	MustRegisterFilterType("test-bogon", []Field{FieldPrefix}, []Field{FieldTag})
	MustRegisterRouteEvaluator("test-bogon", func(_ string, _ map[string]any, _ InstanceResolver) (RouteEvaluator, error) {
		return func(r ribpolicy.Route) RouteResult {
			if r.Prefix == testBogon {
				return RouteResult{Reject: true}
			}
			return RouteResult{Sets: ribpolicy.Sets{Tag: 7, HasTag: true}}
		}, nil
	})
	MustRegisterFilterType("test-no-evaluator", []Field{FieldPrefix}, nil)
	MustRegisterFilterType("test-reads-aspath", []Field{FieldASPath}, nil)
}

func ribSections(chainJSON string) []sdk.ConfigSection {
	return []sdk.ConfigSection{
		{Root: RootName, Data: `{"policy":` + testPolicyJSON + `}`},
		{Root: testRoot, Data: `{"` + testRoot + `":{"filter":{"import":` + chainJSON + `}}}`},
	}
}

const testPolicyJSON = `{"test-bogon":{"B":{}},"test-no-evaluator":{"N":{}},"test-reads-aspath":{"A":{}}}`

// TestRIBImportCompilesAndJudges: a verified chain compiles into a snapshot
// whose source chain rejects the bogon, passes the rest with the filter's tag
// set, and answers final reject when the chain says so.
func TestRIBImportCompilesAndJudges(t *testing.T) {
	cfg, present, err := parseRIBImports(ribSections(`{"filters":["B"],"final":"accept"}`))
	if err != nil || !present {
		t.Fatalf("parse: present=%v err=%v", present, err)
	}
	snap, err := compileRIBSnapshot(cfg)
	if err != nil || snap == nil {
		t.Fatalf("compile: snap=%v err=%v", snap, err)
	}
	id, ok := redistevents.ProtocolIDOf(testProtocol)
	if !ok {
		t.Fatal("the RIB import did not register its protocol")
	}
	if v := snap.Evaluate(ribpolicy.Route{Source: id, Prefix: testBogon}); v.Action != ribpolicy.ActionReject {
		t.Fatalf("bogon: action %v, want reject", v.Action)
	}
	v := snap.Evaluate(ribpolicy.Route{Source: id, Prefix: netip.MustParsePrefix("198.51.100.0/24")})
	if v.Action != ribpolicy.ActionAccept || !v.Sets.HasTag || v.Sets.Tag != 7 {
		t.Fatalf("other prefix: verdict %+v, want accept with tag 7", v)
	}

	cfg, _, err = parseRIBImports(ribSections(`{"filters":["B"],"final":"reject"}`))
	if err != nil {
		t.Fatal(err)
	}
	snap, err = compileRIBSnapshot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if v := snap.Evaluate(ribpolicy.Route{Source: id, Prefix: netip.MustParsePrefix("198.51.100.0/24")}); v.Action != ribpolicy.ActionReject {
		t.Fatalf("final reject: action %v, want reject (a filter's accept only passes the route on)", v.Action)
	}
}

// TestRIBImportNoChainPublishesNil: a delivery with no import chain compiles
// to the nil snapshot, the gate's zero-cost path.
func TestRIBImportNoChainPublishesNil(t *testing.T) {
	cfg, present, err := parseRIBImports([]sdk.ConfigSection{{Root: RootName, Data: `{"policy":{}}`}})
	if err != nil || !present {
		t.Fatalf("parse: present=%v err=%v", present, err)
	}
	snap, err := compileRIBSnapshot(cfg)
	if err != nil || snap != nil {
		t.Fatalf("no chain: snap=%v err=%v, want nil, nil", snap, err)
	}
}

// TestRIBImportRefusals: each refusal names the attachment, the filter and
// the fix, and no refused chain reaches a snapshot.
func TestRIBImportRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, chain string
		want        []string
	}{
		{"unknown filter", `{"filters":["NOPE"],"final":"accept"}`, []string{testAttach, `"NOPE"`, "define it under policy"}},
		{"external reference", `{"filters":["bgp-filter-prefix:X"],"final":"accept"}`, []string{testAttach, "policy filter instance"}},
		{"no route evaluator", `{"filters":["N"],"final":"accept"}`, []string{testAttach, `"N"`, "never calls a plugin process", "test-bogon"}},
		{"field not carried", `{"filters":["A"],"final":"accept"}`, []string{testAttach, `"A"`, "as-path"}},
		{"missing final", `{"filters":["B"]}`, []string{testRoot + "/filter/import/final", "final accept"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseRIBImports(ribSections(tc.chain))
			if err == nil {
				t.Fatal("chain accepted, want a refusal")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("refusal %q does not name %q", err, w)
				}
			}
		})
	}
}

// TestRIBChainEvaluateNoAlloc: a compiled chain judges a route without
// allocating, because the gate runs it on every insert.
func TestRIBChainEvaluateNoAlloc(t *testing.T) {
	cfg, _, err := parseRIBImports(ribSections(`{"filters":["B"],"final":"accept"}`))
	if err != nil {
		t.Fatal(err)
	}
	chain, err := compileRIBChain(&cfg.chains[0], cfg.policy)
	if err != nil {
		t.Fatal(err)
	}
	route := ribpolicy.Route{Prefix: netip.MustParsePrefix("198.51.100.0/24")}
	if allocs := testing.AllocsPerRun(100, func() { _ = chain.Evaluate(route) }); allocs != 0 {
		t.Fatalf("Evaluate allocates %v per route, want 0", allocs)
	}
}
