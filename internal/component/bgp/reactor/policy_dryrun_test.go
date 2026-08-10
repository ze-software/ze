package reactor

import (
	"slices"
	"sort"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// VALIDATES: TracePolicyFilterChain records per-filter trace entries.
// PREVENTS: Trace helper diverging from PolicyFilterChain semantics.
func TestTracePolicyFilterChain(t *testing.T) {
	t.Run("empty_chain_accepts", func(t *testing.T) {
		action, text, trace := tracePolicyFilterChain(nil, "export", "10.0.0.1", 65001, "origin igp", nil)
		if action != PolicyAccept {
			t.Errorf("action = %v, want PolicyAccept", action)
		}
		if text != "origin igp" {
			t.Errorf("text = %q, want %q", text, "origin igp")
		}
		if len(trace) != 0 {
			t.Errorf("trace len = %d, want 0", len(trace))
		}
	})

	t.Run("accept_passthrough", func(t *testing.T) {
		call := func(_, _, _, _ string, _ uint32, text string) PolicyResponse {
			return PolicyResponse{Action: PolicyAccept}
		}
		action, text, trace := tracePolicyFilterChain(
			frefs("plug:FILTER1"), "export", "10.0.0.1", 65001, "origin igp", call,
		)
		if action != PolicyAccept {
			t.Errorf("action = %v, want PolicyAccept", action)
		}
		if text != "origin igp" {
			t.Errorf("text = %q, want %q", text, "origin igp")
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
		action, text, trace := tracePolicyFilterChain(
			frefs("plug:DENY", "plug:AFTER"), "import", "10.0.0.1", 65001, "origin igp", call,
		)
		if action != PolicyReject {
			t.Errorf("action = %v, want PolicyReject", action)
		}
		if text != "" {
			t.Errorf("text = %q, want empty", text)
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
		action, text, trace := tracePolicyFilterChain(
			frefs("plug:SET_MED"), "export", "10.0.0.1", 65001, "origin igp med 100", call,
		)
		if action != PolicyAccept {
			t.Errorf("action = %v, want PolicyAccept", action)
		}
		if text != "origin igp med 200" {
			t.Errorf("text = %q, want %q", text, "origin igp med 200")
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
	})

	t.Run("inactive_skipped", func(t *testing.T) {
		callCount := 0
		call := func(_, _, _, _ string, _ uint32, text string) PolicyResponse {
			callCount++
			return PolicyResponse{Action: PolicyAccept}
		}
		action, _, trace := tracePolicyFilterChain(
			frefs("inactive:plug:SKIP", "plug:KEEP"), "export", "10.0.0.1", 65001, "origin igp", call,
		)
		if action != PolicyAccept {
			t.Errorf("action = %v, want PolicyAccept", action)
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
		action, text, trace := tracePolicyFilterChain(
			frefs("plug:PASS", "plug:SET_MED"), "export", "10.0.0.1", 65001, "origin igp med 100", call,
		)
		if action != PolicyAccept {
			t.Errorf("action = %v, want PolicyAccept", action)
		}
		if text != "origin igp med 200" {
			t.Errorf("text = %q, want %q", text, "origin igp med 200")
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
	})
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
			sort.Strings(got)
			sort.Strings(tt.want)
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

		changes := computeWireChanges(parseFilterAttrs(before), parseFilterAttrs(after), attrs, false, 65001, 65000)
		if !slices.Contains(changes, "AS4_PATH suppressed") {
			t.Errorf("wire changes = %v, want to contain %q", changes, "AS4_PATH suppressed")
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

		changes := computeWireChanges(parseFilterAttrs(before), parseFilterAttrs(after), attrs, false, 65001, 65000)
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

		changes := computeWireChanges(parseFilterAttrs(before), parseFilterAttrs(after), attrs, false, 65001, 65000)
		for _, c := range changes {
			if c == "AS4_PATH suppressed" || c == "AS4_PATH set" {
				t.Errorf("unexpected AS4_PATH wire change %q in %v", c, changes)
			}
		}
	})
}
