package reactor

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: TracePolicyFilterChain records per-filter trace entries.
// PREVENTS: Trace helper diverging from PolicyFilterChain semantics.
func TestTracePolicyFilterChain(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "ibgp", true: "ebgp"}[external], func(t *testing.T) {
			wantAction := PolicyAccept
			if external {
				wantAction = PolicyReject
			}
			wantText := func(text string) string {
				if external {
					return ""
				}
				return text
			}
			t.Run("empty_chain_accepts", func(t *testing.T) {
				result, trace := tracePolicyFilterChain(filterapi.Policy{}, external, "export", "10.0.0.1", 65001, "origin igp", nil)
				if result.Action != wantAction {
					t.Errorf("action = %v, want %v", result.Action, wantAction)
				}
				if result.Text != wantText("origin igp") {
					t.Errorf("text = %q, want %q", result.Text, wantText("origin igp"))
				}
				if len(trace) != 0 {
					t.Errorf("trace len = %d, want 0", len(trace))
				}
			})

			t.Run("accept_passthrough", func(t *testing.T) {
				call := func(_, _, _, _ string, _ uint32, text string) PolicyResponse {
					return PolicyResponse{Action: PolicyAccept}
				}
				result, trace := tracePolicyFilterChain(
					filterapi.InternalPolicy(frefs("plug:FILTER1")), external, "export", "10.0.0.1", 65001, "origin igp", call,
				)
				if result.Action != wantAction {
					t.Errorf("action = %v, want %v", result.Action, wantAction)
				}
				if result.Text != wantText("origin igp") {
					t.Errorf("text = %q, want %q", result.Text, wantText("origin igp"))
				}
				if len(trace) != 1 {
					t.Fatalf("trace len = %d, want 1", len(trace))
				}
				if trace[0].Action != dryRunActionAccept {
					t.Errorf("trace[0].Action = %q, want %q", trace[0].Action, dryRunActionAccept)
				}
				if trace[0].Filter != "FILTER1" {
					t.Errorf("trace[0].Filter = %q, want %q", trace[0].Filter, "FILTER1")
				}
				if trace[0].Canonical != "plug:FILTER1" {
					t.Errorf("trace[0].Canonical = %q, want %q", trace[0].Canonical, "plug:FILTER1")
				}
			})

			t.Run("reject_short_circuits", func(t *testing.T) {
				callCount := 0
				call := func(_, filterName, _, _ string, _ uint32, _ string) PolicyResponse {
					callCount++
					if filterName == "DENY" {
						return PolicyResponse{Action: PolicyReject}
					}
					return PolicyResponse{Action: PolicyAccept}
				}
				result, trace := tracePolicyFilterChain(
					filterapi.InternalPolicy(frefs("plug:DENY", "plug:AFTER")), external, "import", "10.0.0.1", 65001, "origin igp", call,
				)
				if result.Action != PolicyReject {
					t.Errorf("action = %v, want PolicyReject", result.Action)
				}
				if result.Text != "" {
					t.Errorf("text = %q, want empty", result.Text)
				}
				if callCount != 1 {
					t.Errorf("callCount = %d, want 1 (short-circuit)", callCount)
				}
				if len(trace) != 1 {
					t.Fatalf("trace len = %d, want 1", len(trace))
				}
				if trace[0].Action != dryRunActionReject {
					t.Errorf("trace[0].Action = %q, want %q", trace[0].Action, dryRunActionReject)
				}
			})

			t.Run("modify_applies_delta", func(t *testing.T) {
				call := func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
					return PolicyResponse{Action: PolicyModify, Delta: "med 200"}
				}
				result, trace := tracePolicyFilterChain(
					filterapi.InternalPolicy(frefs("plug:SET_MED")), external, "export", "10.0.0.1", 65001, "origin igp med 100", call,
				)
				if result.Action != wantAction {
					t.Errorf("action = %v, want %v", result.Action, wantAction)
				}
				if result.Text != wantText("origin igp med 200") {
					t.Errorf("text = %q, want %q", result.Text, wantText("origin igp med 200"))
				}
				if len(trace) != 1 {
					t.Fatalf("trace len = %d, want 1", len(trace))
				}
				if trace[0].Action != dryRunActionModify {
					t.Errorf("trace[0].Action = %q, want %q", trace[0].Action, dryRunActionModify)
				}
				if trace[0].Delta != "med 200" {
					t.Errorf("trace[0].Delta = %q, want %q", trace[0].Delta, "med 200")
				}
				if trace[0].TextAfter != "origin igp med 200" {
					t.Errorf("trace modified text = %q", trace[0].TextAfter)
				}
			})

			t.Run("inactive_skipped", func(t *testing.T) {
				callCount := 0
				call := func(_, _, _, _ string, _ uint32, text string) PolicyResponse {
					callCount++
					return PolicyResponse{Action: PolicyAccept}
				}
				result, trace := tracePolicyFilterChain(
					filterapi.InternalPolicy(frefs("inactive:plug:SKIP", "plug:KEEP")), external, "export", "10.0.0.1", 65001, "origin igp", call,
				)
				if result.Action != wantAction {
					t.Errorf("action = %v, want %v", result.Action, wantAction)
				}
				if callCount != 1 {
					t.Errorf("callCount = %d, want 1 (inactive skipped)", callCount)
				}
				if len(trace) != 1 {
					t.Fatalf("trace len = %d, want 1", len(trace))
				}
				if trace[0].Filter != "KEEP" {
					t.Errorf("trace[0].Filter = %q, want %q", trace[0].Filter, "KEEP")
				}
			})

			t.Run("multi_filter_chain", func(t *testing.T) {
				call := func(_, filterName, _, _ string, _ uint32, text string) PolicyResponse {
					if filterName == "SET_MED" {
						return PolicyResponse{Action: PolicyModify, Delta: "med 200"}
					}
					return PolicyResponse{Action: PolicyAccept}
				}
				result, trace := tracePolicyFilterChain(
					filterapi.InternalPolicy(frefs("plug:PASS", "plug:SET_MED")), external, "export", "10.0.0.1", 65001, "origin igp med 100", call,
				)
				if result.Action != wantAction {
					t.Errorf("action = %v, want %v", result.Action, wantAction)
				}
				if result.Text != wantText("origin igp med 200") {
					t.Errorf("text = %q, want %q", result.Text, wantText("origin igp med 200"))
				}
				if len(trace) != 2 {
					t.Fatalf("trace len = %d, want 2", len(trace))
				}
				if trace[0].Action != dryRunActionAccept {
					t.Errorf("trace[0].Action = %q, want %q", trace[0].Action, dryRunActionAccept)
				}
				if trace[1].Action != dryRunActionModify {
					t.Errorf("trace[1].Action = %q, want %q", trace[1].Action, dryRunActionModify)
				}
				if trace[0].TextAfter != "origin igp med 100" || trace[1].TextAfter != "origin igp med 200" {
					t.Errorf("piped trace changed: %+v", trace)
				}
			})
		})
	}
}

// TestTracePolicyFinal compares dry-run and runtime results for exhaustion,
// terminal raw rewrites, and failures that must short-circuit either final.
func TestTracePolicyFinal(t *testing.T) {
	for _, final := range []routepolicy.Final{routepolicy.FinalAccept, routepolicy.FinalReject} {
		for _, tc := range []struct {
			name     string
			refs     []routepolicy.Ref
			response PolicyResponse
			calls    int
		}{
			{name: "empty"},
			{name: "inactive", refs: frefs("inactive:plug:SKIP")},
			{name: "accept", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyAccept}, calls: 2},
			{name: "modify", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyModify, Delta: "med 200"}, calls: 2},
			{name: "raw", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyModify, Raw: []byte{0, 0, 0, 0}}, calls: 1},
			{name: "reject", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyReject}, calls: 1},
			{name: "error", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyReject, Failed: true}, calls: 1},
			{name: "teardown", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyAccept, Teardown: true}, calls: 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				policy, err := filterapi.NewPolicy(tc.refs, final)
				require.NoError(t, err)
				calls := 0
				call := func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
					calls++
					return tc.response
				}
				result, trace := tracePolicyFilterChain(policy, true, directionExport, "192.0.2.1", 65001, "origin igp", call)
				require.Equal(t, tc.calls, calls)
				require.Len(t, trace, tc.calls)
				runtime := PolicyFilterChain(policy, true, directionExport, "192.0.2.1", 65001, "origin igp", call)
				require.Equal(t, runtime, result)
				if final == routepolicy.FinalReject {
					require.Equal(t, PolicyReject, result.Action)
					require.Empty(t, result.Text)
				}
			})
		}
	}
}

// VALIDATES: resolveFilterOverride finds canonical ref by plain name.
// PREVENTS: Single-filter override failing to match.
func TestResolveFilterOverride(t *testing.T) {
	chain := frefs(
		"bgp-filter-prefix:CUSTOMERS",
		"bgp-filter-remove-private-as:STRIP",
		"inactive:bgp-filter-prefix:DENY",
	)

	tests := []struct {
		name    string
		input   string
		wantRef string
	}{
		{"plain_name", "CUSTOMERS", "bgp-filter-prefix:CUSTOMERS"},
		{"plain_name_rpa", "STRIP", "bgp-filter-remove-private-as:STRIP"},
		{"canonical_exact", "bgp-filter-prefix:CUSTOMERS", "bgp-filter-prefix:CUSTOMERS"},
		{"not_found", "UNKNOWN", ""},
		// Inactive refs must not resolve: TracePolicyFilterChain skips them, so
		// matching one would yield a misleading silent "accept" (security #4).
		{"inactive_plain_skipped", "DENY", ""},
		{"inactive_canonical_skipped", "inactive:bgp-filter-prefix:DENY", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFilterOverride(tt.input, chain)
			if got != tt.wantRef {
				t.Errorf("resolveFilterOverride(%q) = %q, want %q", tt.input, got, tt.wantRef)
			}
		})
	}
}

// TestRouteDisplayChanged proves changed-attrs names exactly the attributes
// whose shown pair differs between two displays, and nlri when the NLRI
// blocks differ.
//
// VALIDATES: routeDisplay.changed over a changed, a removed and an added MED,
// an unchanged display and changed NLRI.
// PREVENTS: changed-attrs naming an attribute that did not change, or
// missing one the chain set or dropped.
func TestRouteDisplayChanged(t *testing.T) {
	med := slices.Index(displayAttrOrder[:], attribute.AttrMED)
	origin := slices.Index(displayAttrOrder[:], attribute.AttrOrigin)
	display := func(medPair, nlri string) *routeDisplay {
		var d routeDisplay
		d.attrs[origin] = []byte("origin igp")
		if medPair != "" {
			d.attrs[med] = []byte(medPair)
		}
		d.nlri = []byte(nlri)
		return &d
	}
	tests := []struct {
		name          string
		before, after *routeDisplay
		want          []string
	}{
		{"no_change", display("med 100", "nlri add 10.0.0.0/24"), display("med 100", "nlri add 10.0.0.0/24"), nil},
		{"med_changed", display("med 100", ""), display("med 200", ""), []string{"med"}},
		{"attr_removed", display("med 100", ""), display("", ""), []string{"med"}},
		{"attr_added", display("", ""), display("med 200", ""), []string{"med"}},
		{"nlri_changed", display("", "nlri add 10.0.0.0/24"), display("", "nlri del 10.0.0.0/24"), []string{"nlri"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.before.changed(tt.after))
		})
	}
}

// VALIDATES: PolicyTraceEntry and PolicyDryRunResult satisfy ResponseData.
func TestPolicyDryRunResultIsResponseData(t *testing.T) {
	var _ plugin.ResponseData = &plugin.PolicyDryRunResult{}
}

// dryRunUpdateBody builds the body of an UPDATE carrying the five attributes
// the renderer used to drop, beside the AS_PATH and NEXT_HOP every route
// carries and one NLRI. This is the body an operator pastes into
// `ze policy test`, so the test reads what that operator is shown.
func dryRunUpdateBody(t *testing.T) []byte {
	t.Helper()

	var attrs []byte
	attrs = append(attrs, as4FilterAttr(attribute.FlagTransitive, attribute.AttrOrigin, []byte{2})...)
	attrs = append(attrs, as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
		as4FilterPath4(65001, 65002))...)
	attrs = append(attrs, as4FilterNextHop()...)
	attrs = append(attrs, as4FilterAttr(attribute.FlagOptional, attribute.AttrMED,
		[]byte{0, 0, 0, 100})...)
	attrs = append(attrs, as4FilterAttr(attribute.FlagTransitive, attribute.AttrLocalPref,
		[]byte{0, 0, 0, 150})...)
	attrs = append(attrs, as4FilterAttr(attribute.FlagTransitive, attribute.AttrAtomicAggregate, nil)...)
	attrs = append(attrs, as4FilterAttr(attribute.FlagOptional, attribute.AttrClusterList,
		[]byte{1, 1, 1, 1, 2, 2, 2, 2})...)

	// No withdrawn routes, then the attribute length, the attributes, and one
	// NLRI prefix.
	body := []byte{0, 0}
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs))) //nolint:gosec // test data, bounded
	body = append(body, attrs...)
	body = append(body, 24, 10, 0, 0)

	return body
}

// TestPolicyDryRunSubjectNamesEveryAttribute runs `ze policy test` over an
// UPDATE carrying ORIGIN, MULTI_EXIT_DISC, LOCAL_PREF, ATOMIC_AGGREGATE and
// CLUSTER_LIST, on the session those last two belong on (RFC 4271 Section 5.1.5
// and RFC 4456 Section 8 both scope them to an internal session).
//
// VALIDATES: AC-13 -- the before text the operator reads names all five, and
// the JSON rendering of the same result carries that string unchanged, so the
// answer is the same whichever surface the operator asked through.
// PREVENTS: the dry-run explaining a policy over a route it renders wrong. This
// is the only place an operator can inspect the subject before configuring a
// chain, so a missing attribute here teaches them the filter never sees it.
func TestPolicyDryRunSubjectNamesEveryAttribute(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65000, 0x01010101)
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	r.peers[settings.PeerKey()] = peer

	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
		"192.0.2.1", directionImport, "", dryRunUpdateBody(t), true)
	require.NoError(t, err)

	for _, name := range []string{
		policyAttrOrigin,
		policyAttrMED,
		policyAttrLocalPreference,
		policyAttrAtomicAggregate,
		policyAttrClusterList,
	} {
		assert.Contains(t, result.TextBefore, name,
			"the operator must be shown every attribute the route carries")
	}
	assert.Equal(t, "origin incomplete as-path [65001 65002] next-hop 10.0.0.1 med 100 "+
		"local-preference 150 atomic-aggregate cluster-list 1.1.1.1 2.2.2.2 "+
		"nlri add 10.0.0.0/24", result.TextBefore)

	rendered, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(rendered), result.TextBefore,
		"`| json` must carry the same subject the text rendering shows")
}

// TestPolicyDryRunEmptyFinal exercises both direction snapshots through the
// operational adapter without a filter server; empty policies still decide.
func TestPolicyDryRunEmptyFinal(t *testing.T) {
	for _, direction := range []string{directionImport, directionExport} {
		for _, final := range []routepolicy.Final{routepolicy.FinalAccept, routepolicy.FinalReject} {
			t.Run(direction, func(t *testing.T) {
				r := New(&Config{})
				settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
				policy, err := filterapi.NewPolicy(nil, final)
				require.NoError(t, err)
				if direction == directionImport {
					settings.ImportPolicy = policy
				} else {
					settings.ExportPolicy = policy
				}
				r.peers[settings.PeerKey()] = NewPeer(settings)
				result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
					"192.0.2.1", direction, "", dryRunUpdateBody(t), true)
				require.NoError(t, err)
				require.Empty(t, result.Trace)
				if final == routepolicy.FinalReject {
					require.Equal(t, dryRunActionReject, result.Action)
					require.Empty(t, result.TextAfter)
				} else {
					require.Equal(t, dryRunActionAccept, result.Action)
					require.Equal(t, result.TextBefore, result.TextAfter)
				}
			})
		}
	}
}

// The dry-run's own library filter types: one that passes with no sets, one
// whose evaluation fails, so its on-error (reject) decides.
const (
	libraryTypePass = "reactor-library-test-pass"
	libraryTypeFail = "reactor-library-test-fail"
)

// errLibraryTestFail is libraryTypeFail's evaluation failure.
var errLibraryTestFail = errors.New("reactor library test: evaluation failed")

func init() {
	routepolicy.MustRegisterFilterType(libraryTypePass, nil, nil)
	routepolicy.MustRegisterFilterType(libraryTypeFail, nil, nil)
	policychain.MustRegisterEvaluator(libraryTypePass, rpc.OnErrorReject, func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
		return func(*policyview.View, *policychain.Sets) (policychain.Decision, error) {
			return policychain.DecisionPass, nil
		}, nil
	})
	policychain.MustRegisterEvaluator(libraryTypeFail, rpc.OnErrorReject, func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
		return func(*policyview.View, *policychain.Sets) (policychain.Decision, error) {
			return policychain.DecisionReject, errLibraryTestFail
		}, nil
	})
}

// TestPolicyDryRunOverrideKeepsFinal proves selecting one filter cannot remove
// the peer's final, including when that selected filter rewrites the route.
//
// Method: an export library chain of FIRST (sets local-preference 200) then
// LAST (rejects), final reject. The override names FIRST, so its one entry
// modifies, and the final still rejects with no resulting UPDATE.
func TestPolicyDryRunOverrideKeepsFinal(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	reg, err := routepolicy.NewRegistry(routepolicy.Instance{Name: "FIRST", Type: libraryTypeLP200}, routepolicy.Instance{Name: "LAST", Type: libraryTypeReject})
	require.NoError(t, err)
	chain, err := policychain.Compile(libraryTestExport, reg, []policychain.Member{{Name: "FIRST"}, {Name: "LAST"}},
		routepolicy.FinalReject, policychain.Bases{LocalPreference: 100})
	require.NoError(t, err)
	settings.ExportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "FIRST"}, {Name: "LAST"}}, routepolicy.FinalReject).WithLibrary(chain)
	r.peers[settings.PeerKey()] = NewPeer(settings)
	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
		"192.0.2.1", directionExport, "FIRST", dryRunUpdateBody(t), true)
	require.NoError(t, err)
	require.Equal(t, dryRunActionReject, result.Action)
	require.Empty(t, result.TextAfter)
	require.Len(t, result.Trace, 1)
	require.Equal(t, "FIRST", result.Trace[0].Canonical)
	require.Equal(t, dryRunActionModify, result.Trace[0].Action)
}

// TestPolicyDryRunUncompiledChainFailsClosed proves a plugin filter on a
// policy that compiled no library chain is never called and fails the route
// closed, as the session does, with its trace entry saying why.
//
// Method: an export policy naming plug:FIRST, final accept, with no library;
// the filter seam counts calls. Against a dry-run that still reached the
// plugin, or accepted on the final, the run is not a reject with no call.
func TestPolicyDryRunUncompiledChainFailsClosed(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	policy, err := filterapi.NewPolicy(frefs("plug:FIRST"), routepolicy.FinalAccept)
	require.NoError(t, err)
	settings.ExportPolicy = policy
	r.peers[settings.PeerKey()] = NewPeer(settings)
	calls := 0
	r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		return PolicyResponse{Action: PolicyAccept}
	}
	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
		"192.0.2.1", directionExport, "", dryRunUpdateBody(t), true)
	require.NoError(t, err)
	require.Zero(t, calls, "no plugin filter runs outside a compiled chain")
	require.Equal(t, dryRunActionReject, result.Action)
	require.Empty(t, result.TextAfter)
	require.Equal(t, []plugin.PolicyTraceEntry{{Filter: "FIRST", Canonical: "plug:FIRST",
		Action: dryRunActionReject, Error: errUncompiledChain.Error()}}, result.Trace)
}

// TestPolicyDryRunFinalRejectPreservesWithdrawals checks the operational answer,
// including its resulting route text, for pure and mixed UPDATEs in both directions.
// A failed filter, a plugin filter outside a compiled chain and an import
// teardown must not become successful withdrawals, and a raw rewrite does not
// get past the final.
//
// Method: each row is a final-reject policy, its member in process, external
// (probe:f, answered by the externalTransport stand-in) or uncompiled. The
// external teardown row answers accept with teardown set: the chain honors a
// teardown on import only (policychain Chain.run), so on export the member
// passes and the final rejects as for any passing member. The raw row answers
// a whole-body rewrite of the announcement, which the final discards.
func TestPolicyDryRunFinalRejectPreservesWithdrawals(t *testing.T) {
	withdrawal := []byte{0, 4, 24, 192, 0, 2, 0, 0}
	announcement := dryRunUpdateBody(t)
	mixed := append([]byte{0, 4, 24, 192, 0, 2}, announcement[2:]...)
	for _, direction := range []string{directionImport, directionExport} {
		for _, body := range []struct {
			name   string
			update []byte
			action string
		}{
			{name: "pure", update: withdrawal, action: dryRunActionAccept},
			{name: "mixed", update: mixed, action: dryRunActionModify},
		} {
			for _, chain := range []struct {
				name    string
				refs    []routepolicy.Ref
				library string // a library member's filter type, or none
				// external, when set, is the probe:f member's reply.
				external *rpc.FilterUpdateOutput
				reads    []string // what probe:f declares it reads
				blocked  bool
				// blockedImport is blocked on import only.
				blockedImport bool
			}{
				{name: "empty"},
				{name: "inactive", refs: frefs("inactive:plug:FILTER")},
				{name: "accept", library: libraryTypePass},
				{name: "reject", library: libraryTypeReject},
				{name: "modify", library: libraryTypeLP200},
				{name: "failure", library: libraryTypeFail, blocked: true},
				{name: "uncompiled", refs: frefs("plug:FILTER"), blocked: true},
				{name: "teardown", reads: []string{"local-preference"}, blockedImport: true,
					external: &rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Teardown: true}},
				{name: "raw", reads: []string{rpc.FilterFieldRaw},
					external: &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: announcement}},
			} {
				t.Run(direction+"/"+body.name+"/"+chain.name, func(t *testing.T) {
					r := New(&Config{})
					settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
					policy, err := filterapi.NewPolicy(chain.refs, routepolicy.FinalReject)
					if err != nil {
						t.Fatal(err)
					}
					if chain.library != "" {
						attachment := libraryTestImport
						if direction == directionExport {
							attachment = libraryTestExport
						}
						policy = libraryPolicy(t, attachment, chain.library, routepolicy.FinalReject)
					}
					var transport *externalTransport
					if chain.external != nil {
						transport = &externalTransport{reads: chain.reads, reply: *chain.external}
						r.filterTransportSeam = transport
						policy = externalFinalPolicy(t, routepolicy.FinalReject)
					}
					if direction == directionImport {
						settings.ImportPolicy = policy
					} else {
						settings.ExportPolicy = policy
					}
					r.peers[settings.PeerKey()] = NewPeer(settings)
					result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
						"192.0.2.1", direction, "", body.update, true)
					if err != nil {
						t.Fatal(err)
					}
					if transport != nil && len(transport.sent) != 1 {
						t.Fatalf("probe:f was asked %d times, want once", len(transport.sent))
					}
					if chain.blocked || (chain.blockedImport && direction == directionImport) {
						if result.Action != dryRunActionReject || result.TextAfter != "" {
							t.Fatalf("failed, uncompiled or teardown filter = %q, %q; want reject without output",
								result.Action, result.TextAfter)
						}
						return
					}
					if result.Action != body.action {
						t.Errorf("action = %q, want %q", result.Action, body.action)
					}
					if result.TextAfter != "nlri del 192.0.2.0/24" {
						t.Errorf("resulting text = %q, want only the explicit withdrawal", result.TextAfter)
					}
					if body.name == "pure" && result.TextBefore != result.TextAfter {
						t.Errorf("pure withdrawal changed: before %q, after %q", result.TextBefore, result.TextAfter)
					}
					if body.name == "mixed" && result.TextBefore == result.TextAfter {
						t.Error("mixed UPDATE kept its denied announcement")
					}
				})
			}
		}
	}
}

// externalFinalPolicy answers a policy over the one external member probe:f,
// compiled at libraryTestMixed under final, as bgp/config builds a chain
// naming a plugin filter.
func externalFinalPolicy(t *testing.T, final routepolicy.Final) filterapi.Policy {
	t.Helper()
	reg, err := routepolicy.NewRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(libraryTestMixed, reg, []policychain.Member{{Name: "f", Plugin: "probe"}},
		final, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return mustPolicy(t, frefs("probe:f"), final).WithLibrary(chain)
}
