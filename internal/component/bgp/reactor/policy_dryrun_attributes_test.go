package reactor

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const libraryTraceAttributeType = "reactor-library-trace-attribute"
const libraryTraceAttributeAttachment = "reactor-library-trace-attribute import"

func init() {
	fields := []routepolicy.Field{routepolicy.FieldOrigin, routepolicy.FieldNextHop,
		routepolicy.FieldAIGP, routepolicy.FieldLargeCommunity, routepolicy.FieldExtendedCommunity}
	routepolicy.MustRegisterAttachment("bgp", libraryTraceAttributeAttachment, fields, fields)
	routepolicy.MustRegisterFilterType(libraryTraceAttributeType, nil, fields)
	policychain.MustRegisterEvaluator(libraryTraceAttributeType, rpc.OnErrorReject,
		func(_ string, config map[string]any, _ routepolicy.InstanceResolver) (policychain.Evaluator, error) {
			evaluate, ok := config["evaluate"].(policychain.Evaluator)
			if !ok {
				return nil, errors.New("trace attribute fixture has no evaluator")
			}
			return evaluate, nil
		})
}

func attributeTracePolicy(t *testing.T, set func(*policychain.Sets) error) *policychain.Chain {
	t.Helper()
	evaluate := policychain.Evaluator(func(_ *policyview.View, sets *policychain.Sets) (policychain.Decision, error) {
		return policychain.DecisionPass, set(sets)
	})
	reg, err := routepolicy.NewRegistry(
		routepolicy.Instance{Name: "first", Type: libraryTypeReject},
		routepolicy.Instance{Name: "second", Type: libraryTraceAttributeType, Config: map[string]any{"evaluate": evaluate}})
	if err != nil {
		t.Fatal(err)
	}
	chain, err := policychain.Compile(libraryTraceAttributeAttachment, reg,
		[]policychain.Member{{Name: "first"}, {Name: "second"}}, routepolicy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

// TestPolicyDryRunAttributeOverride proves the operator's single-filter path
// retains the compiled readers for all five attribute overlays. The first
// member rejects, so only the requested second member can produce a rewrite.
// The AIGP case additionally pins Delta to the metric in TextAfter, not zero.
func TestPolicyDryRunAttributeOverride(t *testing.T) {
	for _, tc := range []struct {
		name  string
		set   func(*policychain.Sets) error
		delta string
	}{
		{"origin", func(s *policychain.Sets) error { return s.Origin(attribute.OriginEGP) }, "origin " + attribute.OriginEGP.String()},
		{"next-hop", func(s *policychain.Sets) error { return s.NextHop(netip.MustParseAddr("192.0.2.9")) }, "next-hop 192.0.2.9"},
		{"aigp", func(s *policychain.Sets) error { return s.AIGP(policychain.NumericActionSet, 100) }, "aigp 100"},
		{"large-community", func(s *policychain.Sets) error {
			return s.AddLargeCommunity(attribute.LargeCommunity{GlobalAdmin: 1, LocalData1: 2, LocalData2: 3})
		}, "large-community add 1:2:3"},
		{"extended-community", func(s *policychain.Sets) error {
			return s.AddExtendedCommunity(attribute.ExtendedCommunity{0, 2, 0, 0, 0, 0, 0, 1})
		}, "extended-community add 0x0002000000000001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New(&Config{})
			settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
			chain := attributeTracePolicy(t, tc.set)
			settings.ImportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "first"}, {Name: "second"}},
				routepolicy.FinalAccept).WithLibrary(chain)
			r.peers[settings.PeerKey()] = NewPeer(settings)
			result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
				"192.0.2.1", directionImport, "second", libraryTestBody(100), true)
			if err != nil {
				t.Fatal(err)
			}
			if result.Action != dryRunActionModify || len(result.Trace) != 1 {
				t.Fatalf("operator result action=%s trace=%+v, want one modifying member", result.Action, result.Trace)
			}
			entry := result.Trace[0]
			if entry.Action != dryRunActionModify || entry.Delta != tc.delta || entry.TextAfter != result.TextAfter {
				t.Errorf("operator entry=%+v, want delta=%q and final rewritten text", entry, tc.delta)
			}
			if tc.name == "aigp" && !strings.Contains(result.TextAfter, "aigp 100") {
				t.Errorf("operator AIGP text=%q, want metric 100 coherent with Delta", result.TextAfter)
			}
			if chain.Refused() != 0 {
				t.Errorf("operator display changed live refused counter=%d", chain.Refused())
			}
		})
	}
}

// TestPolicyDryRunAIGPDeltaMatchesRewrite proves the whole-chain operator
// trace, independently of Only, displays the resolved metric actually written.
func TestPolicyDryRunAIGPDeltaMatchesRewrite(t *testing.T) {
	// Compile a fresh one-member chain instead of using Only for this proof:
	// the renderer must fail independently of the reader-copy defect.
	reg, err := routepolicy.NewRegistry(routepolicy.Instance{Name: "second", Type: libraryTraceAttributeType,
		Config: map[string]any{"evaluate": policychain.Evaluator(func(_ *policyview.View, s *policychain.Sets) (policychain.Decision, error) {
			return policychain.DecisionPass, s.AIGP(policychain.NumericActionSet, 100)
		})}})
	if err != nil {
		t.Fatal(err)
	}
	only, err := policychain.Compile(libraryTraceAttributeAttachment, reg, []policychain.Member{{Name: "second"}}, routepolicy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatal(err)
	}
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	settings.ImportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "second"}}, routepolicy.FinalAccept).WithLibrary(only)
	r.peers[settings.PeerKey()] = NewPeer(settings)
	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun("192.0.2.1", directionImport, "", libraryTestBody(100), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != dryRunActionModify || len(result.Trace) != 1 {
		t.Fatalf("action=%s trace=%+v, want modifying AIGP member", result.Action, result.Trace)
	}
	if result.Trace[0].Delta != "aigp 100" || !strings.Contains(result.TextAfter, "aigp 100") {
		t.Errorf("operator delta=%q text=%q, want coherent metric100", result.Trace[0].Delta, result.TextAfter)
	}
}
