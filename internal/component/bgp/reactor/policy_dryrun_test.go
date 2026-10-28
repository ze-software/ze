package reactor

import (
	"encoding/binary"
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
	for _, final := range []filterapi.FinalDisposition{filterapi.FinalAccept, filterapi.FinalReject} {
		for _, tc := range []struct {
			name     string
			refs     []filterapi.FilterRef
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
				if final == filterapi.FinalReject {
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

// VALIDATES: computeChangedAttrs detects attribute changes between before/after text.
// PREVENTS: Missing or spurious changed attribute detection.
func TestComputeChangedAttrs(t *testing.T) {
	tests := []struct {
		name   string
		before string
		after  string
		want   []string
	}{
		{"no_change", "origin igp med 100", "origin igp med 100", nil},
		{"med_changed", "origin igp med 100", "origin igp med 200", []string{"med"}},
		{"attr_removed", "origin igp med 100", "origin igp", []string{"med"}},
		{"attr_added", "origin igp", "origin igp med 200", []string{"med"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeChangedAttrs(parseFilterAttrs(tt.before), parseFilterAttrs(tt.after))
			slices.Sort(got)
			slices.Sort(tt.want)
			if len(got) != len(tt.want) {
				t.Fatalf("changed = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("changed[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// VALIDATES: PolicyTraceEntry and PolicyDryRunResult satisfy ResponseData.
func TestPolicyDryRunResultIsResponseData(t *testing.T) {
	var _ plugin.ResponseData = &plugin.PolicyDryRunResult{}
}

// VALIDATES: AC-10 -- remove-private-as suppresses/rewrites AS4_PATH (RFC 6793)
// and the dry-run surfaces it in the wire-changes section, even though the flat
// filter text only carries the merged "as-path" and never names AS4_PATH.
// PREVENTS: AS4_PATH effects being invisible to operators using policy test.
func TestComputeWireChangesAS4Path(t *testing.T) {
	// 2-byte session AS_PATH carrying only non-private ASNs (64500, AS_TRANS
	// 23456): no AS_PATH change. The private 4-byte ASN lives in AS4_PATH.
	asPathVal := []byte{
		byte(attribute.ASSequence), 2,
		0xFB, 0xF4, // 64500 (public, below the 64512 private floor)
		0x5B, 0xA0, // 23456 (AS_TRANS, RFC 6793)
	}

	t.Run("as4_path_suppressed_when_emptied", func(t *testing.T) {
		// AS4_PATH = [4200000000] -- a single Private Use ASN. Stripping it
		// leaves an empty path, so the attribute is suppressed entirely.
		as4Val := []byte{
			byte(attribute.ASSequence), 1,
			0xFA, 0x56, 0xEA, 0x00, // 4200000000 (private, RFC 6996)
		}
		packed := append(makeAttr(0x40, byte(attribute.AttrASPath), asPathVal),
			makeAttr(0xC0, byte(attribute.AttrAS4Path), as4Val)...)
		attrs := attribute.NewAttributesWire(packed, 0)

		before := "origin igp as-path [64500 23456]"
		after := "origin igp as-path [64500 23456] remove-private strip"

		changes := computeWireChanges(parseFilterAttrs(before), parseFilterAttrs(after), attrs, directionExport, false, 65001, 65000, false)
		if !slices.Contains(changes, "AS4_PATH suppressed") {
			t.Errorf("wire changes = %v, want to contain %q", changes, "AS4_PATH suppressed")
		}
	})

	// The med-remove directive of RFC 4271 Section 5.1.4 is converted on the
	// import chain alone, so the dry-run must report it on the direction the
	// runtime honors and on no other. Without this the operator is told the
	// metric survives an import policy that removes it.
	t.Run("med_remove_is_reported_on_import_and_not_on_export", func(t *testing.T) {
		attrs := attribute.NewAttributesWire(makeAttr(0x80, byte(attribute.AttrMED), []byte{0, 0, 0, 100}), 0)
		before := "origin igp med 100"
		after := "origin igp med 100 med-remove"

		onImport := computeWireChanges(parseFilterAttrs(before), parseFilterAttrs(after), attrs, directionImport, false, 65001, 65000, false)
		if !slices.Contains(onImport, "MULTI_EXIT_DISC suppressed") {
			t.Errorf("import wire changes = %v, want to contain %q", onImport, "MULTI_EXIT_DISC suppressed")
		}

		onExport := computeWireChanges(parseFilterAttrs(before), parseFilterAttrs(after), attrs, directionExport, false, 65001, 65000, false)
		if slices.Contains(onExport, "MULTI_EXIT_DISC suppressed") {
			t.Errorf("export wire changes = %v, must not promise a removal the export rail does not perform", onExport)
		}
	})

	t.Run("as4_path_set_when_partially_stripped", func(t *testing.T) {
		// AS4_PATH = [4200000000 131072] -- one private, one public 4-byte ASN.
		// Stripping the private one leaves a non-empty path, so AS4_PATH is reset.
		as4Val := []byte{
			byte(attribute.ASSequence), 2,
			0xFA, 0x56, 0xEA, 0x00, // 4200000000 (private)
			0x00, 0x02, 0x00, 0x00, // 131072 (public 4-byte)
		}
		packed := append(makeAttr(0x40, byte(attribute.AttrASPath), asPathVal),
			makeAttr(0xC0, byte(attribute.AttrAS4Path), as4Val)...)
		attrs := attribute.NewAttributesWire(packed, 0)

		before := "origin igp as-path [64500 23456]"
		after := "origin igp as-path [64500 23456] remove-private strip"

		changes := computeWireChanges(parseFilterAttrs(before), parseFilterAttrs(after), attrs, directionExport, false, 65001, 65000, false)
		if !slices.Contains(changes, "AS4_PATH set") {
			t.Errorf("wire changes = %v, want to contain %q", changes, "AS4_PATH set")
		}
	})

	t.Run("no_directive_no_wire_changes", func(t *testing.T) {
		as4Val := []byte{
			byte(attribute.ASSequence), 1,
			0xFA, 0x56, 0xEA, 0x00,
		}
		packed := append(makeAttr(0x40, byte(attribute.AttrASPath), asPathVal),
			makeAttr(0xC0, byte(attribute.AttrAS4Path), as4Val)...)
		attrs := attribute.NewAttributesWire(packed, 0)

		// A plain MED change with no remove-private directive must not emit
		// any AS4_PATH wire change.
		before := "origin igp as-path [64500 23456] med 100"
		after := "origin igp as-path [64500 23456] med 200"

		changes := computeWireChanges(parseFilterAttrs(before), parseFilterAttrs(after), attrs, directionExport, false, 65001, 65000, false)
		for _, c := range changes {
			if c == "AS4_PATH suppressed" || c == "AS4_PATH set" {
				t.Errorf("unexpected AS4_PATH wire change %q in %v", c, changes)
			}
		}
	})
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

// TestPolicyDryRunPrependReportsAtTheSessionWidth is the third call site of
// ExtractASPathPrependOps, and the one an operator reads rather than a peer.
// computeWireChanges reports the operations `policy test peer` will perform, so
// a site that did not pass its own width would describe a rewrite the runtime
// does not do.
//
// The width is not printed, so the assertion reads the operation it decides.
// RFC 6793 Section 4.2.2 obliges an AS4_PATH beside a two-octet AS_PATH that
// carries AS_TRANS, and Section 4.1 forbids one toward a four-octet peer, so
// "AS4_PATH set" is present on exactly one of the two widths. Against an
// extractor that took no width, neither width reported it.
//
// VALIDATES: the wiring row for computeWireChanges -- the dry-run passes its
// own asn4 through to the extractor.
// PREVENTS: `policy test peer` telling an operator the AS-path family is
// untouched on the session where the prepend rewrites two attributes.
func TestPolicyDryRunPrependReportsAtTheSessionWidth(t *testing.T) {
	// AS_SEQUENCE [64496]. The two encodings of one path, so the only thing
	// that changes between the subtests is the width the session negotiated.
	asPath2 := makeAttr(0x40, byte(attribute.AttrASPath), []byte{byte(attribute.ASSequence), 1, 0xFB, 0xF0})
	asPath4 := makeAttr(0x40, byte(attribute.AttrASPath), []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFB, 0xF0})

	before := "origin igp"
	after := "origin igp as-path-prepend 2"

	tests := []struct {
		name       string
		asn4       bool
		packed     []byte
		wantAS4Set bool
	}{
		{name: "two_octet_session_owes_an_as4_path", asn4: false, packed: asPath2, wantAS4Set: true},
		{name: "four_octet_session_owes_none", asn4: true, packed: asPath4, wantAS4Set: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := attribute.NewAttributesWire(tt.packed, 0)

			// localAS 131072 is above 65535, so it is the AS_TRANS case at two
			// octets and the plain four-octet case above.
			changes := computeWireChanges(parseFilterAttrs(before), parseFilterAttrs(after), attrs, directionExport, tt.asn4, 65001, 131072, false)

			assert.True(t, slices.Contains(changes, "AS_PATH prepend"),
				"wire changes = %v, want the prepend reported at both widths", changes)
			assert.Equal(t, tt.wantAS4Set, slices.Contains(changes, "AS4_PATH set"),
				"wire changes = %v, AS4_PATH is owed at two octets and forbidden at four", changes)
		})
	}
}

// TestPolicyDryRunEmptyFinal exercises both direction snapshots through the
// operational adapter without a filter server; empty policies still decide.
func TestPolicyDryRunEmptyFinal(t *testing.T) {
	for _, direction := range []string{directionImport, directionExport} {
		for _, final := range []filterapi.FinalDisposition{filterapi.FinalAccept, filterapi.FinalReject} {
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
				if final == filterapi.FinalReject {
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

// TestPolicyDryRunOverrideKeepsFinal proves selecting one filter cannot remove
// the peer's final, including when that selected filter returns a raw rewrite.
func TestPolicyDryRunOverrideKeepsFinal(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	policy, err := filterapi.NewPolicy(frefs("plug:FIRST", "plug:LAST"), filterapi.FinalReject)
	require.NoError(t, err)
	settings.ExportPolicy = policy
	r.peers[settings.PeerKey()] = NewPeer(settings)
	calls := 0
	r.policyFilterSeam = func(_, filter, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		require.Equal(t, "FIRST", filter)
		return PolicyResponse{Action: PolicyModify, Raw: []byte{0, 0, 0, 0}}
	}
	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
		"192.0.2.1", directionExport, "FIRST", dryRunUpdateBody(t), true)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Equal(t, dryRunActionReject, result.Action)
	require.Empty(t, result.TextAfter)
	require.Len(t, result.Trace, 1)
	require.Equal(t, dryRunActionModify, result.Trace[0].Action)
}

// TestPolicyDryRunFinalRejectPreservesWithdrawals checks the operational answer,
// including its resulting route text, for pure and mixed UPDATEs in both directions.
// Filter execution failures and teardown must not become successful withdrawals.
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
				name     string
				refs     []filterapi.FilterRef
				response PolicyResponse
				blocked  bool
			}{
				{name: "empty"},
				{name: "inactive", refs: frefs("inactive:plug:FILTER")},
				{name: "accept", refs: frefs("plug:FILTER"), response: PolicyResponse{Action: PolicyAccept}},
				{name: "reject", refs: frefs("plug:FILTER"), response: PolicyResponse{Action: PolicyReject}},
				{name: "raw", refs: frefs("plug:FILTER"), response: PolicyResponse{Action: PolicyModify, Raw: announcement}},
				{name: "failure", refs: frefs("plug:FILTER"), response: PolicyResponse{Action: PolicyReject, Failed: true}, blocked: true},
				{name: "teardown", refs: frefs("plug:FILTER"), response: PolicyResponse{Action: PolicyAccept, Teardown: true}, blocked: true},
			} {
				t.Run(direction+"/"+body.name+"/"+chain.name, func(t *testing.T) {
					r := New(&Config{})
					settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
					policy, err := filterapi.NewPolicy(chain.refs, filterapi.FinalReject)
					if err != nil {
						t.Fatal(err)
					}
					if direction == directionImport {
						settings.ImportPolicy = policy
					} else {
						settings.ExportPolicy = policy
					}
					r.peers[settings.PeerKey()] = NewPeer(settings)
					r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
						return chain.response
					}
					result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
						"192.0.2.1", direction, "", body.update, true)
					if err != nil {
						t.Fatal(err)
					}
					if chain.blocked {
						if result.Action != dryRunActionReject || result.TextAfter != "" {
							t.Fatalf("failed execution or teardown = %q, %q; want reject without output",
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
