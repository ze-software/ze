package policychain

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// AS-path read test types: a member that prepends twice, and a gate that
// passes only a route whose as-path reads exactly asPathGateWant.
const (
	typePrependTwice = "policychain-test-prepend-twice" // sets as-path
	typeASPathGate   = "policychain-test-aspath-gate"   // reads as-path

	testAttachmentASPath = "policychain-test as-path" // reads and sets as-path
)

// asPathGateWant is the flattened AS path the gate passes.
var asPathGateWant []uint32

func init() {
	asPath := policy.FieldASPath
	policy.MustRegisterAttachment("bgp", testAttachmentASPath, []policy.Field{asPath}, []policy.Field{asPath})
	policy.MustRegisterFilterType(typePrependTwice, nil, []policy.Field{asPath})
	policy.MustRegisterFilterType(typeASPathGate, []policy.Field{asPath}, nil)
	MustRegisterEvaluator(typePrependTwice, rpc.OnErrorReject, func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		return func(_ *policyview.View, sets *Sets) (Decision, error) {
			return DecisionPass, sets.PrependASPath(2)
		}, nil
	})
	MustRegisterEvaluator(typeASPathGate, rpc.OnErrorReject, func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		reader, err := policyview.ReaderFor(asPath)
		if err != nil {
			return nil, err
		}
		return func(view *policyview.View, _ *Sets) (Decision, error) {
			value, err := view.Read(reader)
			if err != nil {
				return DecisionUnspecified, err
			}
			var flat []uint32
			if path := value.ASPath(); path != nil {
				for _, seg := range path.Segments {
					flat = append(flat, seg.ASNs...)
				}
			}
			if slices.Equal(flat, asPathGateWant) {
				return DecisionPass, nil
			}
			return DecisionReject, nil
		}, nil
	})
}

// TestChainASPathReadsReceivedAfterPrepend pins what a later member reads for
// as-path once an earlier member prepended: the RECEIVED path, with no
// prepended AS. That is the text chain's answer too: a text filter's
// "as-path-prepend N" delta is a directive merged beside the as-path field
// (applyFilterDelta, reactor/filter_chain.go), and the field itself keeps the
// received value until the wire rebuild applies the prepend. The library
// chain answers the same, so a policy reads alike on both paths.
//
// Method: the gate passes only on the received path; a chain that overlaid
// the prepend would make it reject.
func TestChainASPathReadsReceivedAfterPrepend(t *testing.T) {
	asPathGateWant = []uint32{3356, 174}
	reg, err := policy.NewRegistry(
		policy.Instance{Name: "prepend", Type: typePrependTwice},
		policy.Instance{Name: "gate", Type: typeASPathGate})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := Compile(testAttachmentASPath, reg, []Member{{Name: "prepend"}, {Name: "gate"}}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	path := pathAttr(0x40, attribute.AttrASPath, []byte{2, 2, 0, 0, 0x0d, 0x1c, 0, 0, 0, 0xae})
	outcome, sets := run(t, chain, update(t, path))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept: the gate did not read the received as-path", outcome)
	}
	if got := sets.Folded().ASPathPrepend(); got != 2 {
		t.Fatalf("folded prepend = %d, want 2: the prepend itself still applies", got)
	}
}
