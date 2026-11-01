package policychain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The test attachment and filter types, registered once for the package. The
// attachment carries and allows what a BGP attachment does for the fields
// these tests use; the types stand for library types with BGP evaluators.
const (
	testAttachment = "policychain-test import"
	typeSetLocal   = "policychain-test-set-local"    // reads local-preference, sets local-preference
	typeSetMED     = "policychain-test-set-med"      // reads med, sets med
	typeFailOpen   = "policychain-test-fail-open"    // reads local-preference, sets nothing, on-error accept
	typeUndeclared = "policychain-test-undeclared"   // declares local-preference, writes med
	typeCommunity  = "policychain-test-community"    // sets community
	typeNoEval     = "policychain-test-no-evaluator" // declared fields, no evaluator
	typeGate100    = "policychain-test-gate-100"     // reads local-preference, rejects unless 100
)

// numericConfig is the instance config the test evaluators read: an action
// and a value.
func numericConfig(action NumericAction, value uint32) map[string]any {
	return map[string]any{"action": action, "value": value}
}

func numericEvaluator(set func(*Sets, NumericAction, uint32) error, read policy.Field) EvaluatorCompiler {
	return func(_ string, instance map[string]any, _ policy.InstanceResolver) (Evaluator, error) {
		action, _ := instance["action"].(NumericAction)
		value, _ := instance["value"].(uint32)
		reader, err := policyview.ReaderFor(read)
		if err != nil {
			return nil, err
		}
		return func(view *policyview.View, sets *Sets) (Decision, error) {
			if _, err := view.Read(reader); err != nil {
				return DecisionUnspecified, err
			}
			if err := set(sets, action, value); err != nil {
				return DecisionUnspecified, err
			}
			return DecisionPass, nil
		}, nil
	}
}

func init() {
	lp, med, comm := policy.FieldLocalPreference, policy.FieldMED, policy.FieldCommunity
	policy.MustRegisterAttachment("bgp", testAttachment, []policy.Field{lp, med, comm}, []policy.Field{lp, med, comm})
	policy.MustRegisterFilterType(typeSetLocal, []policy.Field{lp}, []policy.Field{lp})
	policy.MustRegisterFilterType(typeSetMED, []policy.Field{med}, []policy.Field{med})
	policy.MustRegisterFilterType(typeFailOpen, []policy.Field{lp}, nil)
	policy.MustRegisterFilterType(typeUndeclared, nil, []policy.Field{lp})
	policy.MustRegisterFilterType(typeCommunity, nil, []policy.Field{comm})
	policy.MustRegisterFilterType(typeNoEval, nil, nil)
	policy.MustRegisterFilterType(typeGate100, []policy.Field{lp}, nil)
	MustRegisterEvaluator(typeGate100, rpc.OnErrorReject, func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		reader, err := policyview.ReaderFor(lp)
		if err != nil {
			return nil, err
		}
		return func(view *policyview.View, _ *Sets) (Decision, error) {
			value, err := view.Read(reader)
			if err != nil {
				return DecisionUnspecified, err
			}
			if number, _ := value.Number(); value.Present() && number == 100 {
				return DecisionPass, nil
			}
			return DecisionReject, nil
		}, nil
	})

	MustRegisterEvaluator(typeSetLocal, rpc.OnErrorReject, numericEvaluator((*Sets).LocalPreference, lp))
	MustRegisterEvaluator(typeSetMED, rpc.OnErrorReject, numericEvaluator((*Sets).MED, med))
	MustRegisterEvaluator(typeFailOpen, rpc.OnErrorAccept, numericEvaluator(func(*Sets, NumericAction, uint32) error { return nil }, lp))
	MustRegisterEvaluator(typeUndeclared, rpc.OnErrorReject, func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		return func(_ *policyview.View, sets *Sets) (Decision, error) {
			return DecisionPass, sets.MED(NumericActionSet, 5)
		}, nil
	})
	MustRegisterEvaluator(typeCommunity, rpc.OnErrorReject, func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		return func(_ *policyview.View, sets *Sets) (Decision, error) {
			if err := sets.AddCommunity(0xfde90064); err != nil {
				return DecisionUnspecified, err
			}
			return DecisionPass, sets.RemoveCommunity(0xfde90001)
		}, nil
	})
}

func pathAttr(flags byte, code attribute.AttributeCode, value []byte) []byte {
	return append([]byte{flags, byte(code), byte(len(value))}, value...)
}

// update builds an UPDATE body with the given attributes and one IPv4 prefix.
func update(t *testing.T, attrs ...[]byte) *wireu.WireUpdate {
	t.Helper()
	section := bytes.Join(attrs, nil)
	payload := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(section)))
	payload = append(payload, section...)
	payload = append(payload, 24, 203, 0, 113)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(payload, id)
}

func localPref(v uint32) []byte {
	return pathAttr(0x40, attribute.AttrLocalPref, binary.BigEndian.AppendUint32(nil, v))
}

func med(v uint32) []byte {
	return pathAttr(0x80, attribute.AttrMED, binary.BigEndian.AppendUint32(nil, v))
}

func compile(t *testing.T, final policy.Final, instances ...policy.Instance) *Chain {
	t.Helper()
	reg, err := policy.NewRegistry(instances...)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	members := make([]Member, len(instances))
	for i, inst := range instances {
		members[i] = Member{Name: inst.Name}
	}
	chain, err := Compile(testAttachment, reg, members, final, Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

func run(t *testing.T, chain *Chain, wire *wireu.WireUpdate) (Outcome, *RunSets) {
	t.Helper()
	var view policyview.View
	var sets RunSets
	view.Reset(wire, policyview.Session{})
	defer view.Clear()
	return chain.Run(&view, &sets), &sets
}

// applied answers the ModAccumulator ops the folded sets produce.
func applied(t *testing.T, sets *Sets, current Current) []filterapi.AttrOp {
	t.Helper()
	var acc filterapi.ModAccumulator
	if err := sets.ApplyTo(&acc, current); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return acc.Ops()
}

// TestChainSetsLocalPreference proves the in-process half of AC-4: a member
// that sets local-preference 200 makes the chain accept with one LOCAL_PREF
// set operation carrying 200, the op the text delta path emits for the same
// change (buildModifiedPayload writes both alike).
func TestChainSetsLocalPreference(t *testing.T) {
	chain := compile(t, policy.FinalAccept,
		policy.Instance{Name: "lp200", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 200)})
	outcome, sets := run(t, chain, update(t, localPref(100)))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}
	ops := applied(t, sets.Folded(), Current{LocalPreference: 100, HasLocalPreference: true})
	if len(ops) != 1 {
		t.Fatalf("ops = %d, want 1", len(ops))
	}
	op := ops[0]
	if op.Code != uint8(attribute.AttrLocalPref) || op.Action != filterapi.AttrModSet || !bytes.Equal(op.Buf, []byte{0, 0, 0, 200}) {
		t.Fatalf("op = code %d action %d buf %v, want LOCAL_PREF set 0,0,0,200", op.Code, op.Action, op.Buf)
	}
}

// TestChainFoldsNumericSets proves AC-25's arithmetic in process: members fold
// in chain order, an increment adds to the route's current value or to an
// earlier member's set, a decrement clamps at zero, and arithmetic on an
// absent attribute starts from the configured base and is refused without one.
func TestChainFoldsNumericSets(t *testing.T) {
	chain := compile(t, policy.FinalAccept,
		policy.Instance{Name: "lp-set", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 100)},
		policy.Instance{Name: "med-dec", Type: typeSetMED, Config: numericConfig(NumericActionDecrement, 30)})
	outcome, sets := run(t, chain, update(t, localPref(7), med(20)))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}
	ops := applied(t, sets.Folded(), Current{LocalPreference: 7, HasLocalPreference: true, MED: 20, HasMED: true})
	if len(ops) != 2 || !bytes.Equal(ops[0].Buf, []byte{0, 0, 0, 100}) || !bytes.Equal(ops[1].Buf, []byte{0, 0, 0, 0}) {
		t.Fatalf("ops = %+v, want local-preference 100 and med clamped to 0", ops)
	}

	var folded Sets
	if err := folded.LocalPreference(NumericActionSet, 100); err != nil {
		t.Fatal(err)
	}
	var later Sets
	if err := later.LocalPreference(NumericActionIncrement, 50); err != nil {
		t.Fatal(err)
	}
	if err := folded.merge(&later); err != nil {
		t.Fatal(err)
	}
	if ops := applied(t, &folded, Current{LocalPreference: 7, HasLocalPreference: true}); !bytes.Equal(ops[0].Buf, []byte{0, 0, 0, 150}) {
		t.Fatalf("set 100 then increment 50 = %v, want 150", ops[0].Buf)
	}

	var absent Sets
	if err := absent.MED(NumericActionIncrement, 10); err != nil {
		t.Fatal(err)
	}
	var acc filterapi.ModAccumulator
	if err := absent.ApplyTo(&acc, Current{}); err == nil {
		t.Fatal("increment of an absent med with no base applied, want refused")
	}
	if ops := applied(t, &absent, Current{MEDBase: 40, HasMEDBase: true}); !bytes.Equal(ops[0].Buf, []byte{0, 0, 0, 50}) {
		t.Fatalf("increment 10 of an absent med with base 40 = %v, want 50", ops[0].Buf)
	}

	// The upper clamp: an increment past 4294967295 stops there, never wraps.
	var high Sets
	if err := high.LocalPreference(NumericActionIncrement, 100); err != nil {
		t.Fatal(err)
	}
	if ops := applied(t, &high, Current{LocalPreference: 4294967290, HasLocalPreference: true}); !bytes.Equal(ops[0].Buf, []byte{0xff, 0xff, 0xff, 0xff}) {
		t.Fatalf("increment 100 of local-preference 4294967290 = %v, want clamped to 4294967295", ops[0].Buf)
	}
}

// TestChainCommunityOps proves the community half of AC-25 in process: an add
// and a remove become one COMMUNITY add and one remove op of four octets each.
func TestChainCommunityOps(t *testing.T) {
	chain := compile(t, policy.FinalAccept, policy.Instance{Name: "comm", Type: typeCommunity})
	outcome, sets := run(t, chain, update(t, localPref(100)))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}
	ops := applied(t, sets.Folded(), Current{})
	want := []filterapi.AttrOp{
		{Code: uint8(attribute.AttrCommunity), Action: filterapi.AttrModAdd, Buf: []byte{0xfd, 0xe9, 0, 0x64}},
		{Code: uint8(attribute.AttrCommunity), Action: filterapi.AttrModRemove, Buf: []byte{0xfd, 0xe9, 0, 1}},
	}
	if len(ops) != len(want) {
		t.Fatalf("ops = %+v, want %+v", ops, want)
	}
	for i := range want {
		if ops[i].Code != want[i].Code || ops[i].Action != want[i].Action || !bytes.Equal(ops[i].Buf, want[i].Buf) {
			t.Fatalf("op %d = %+v, want %+v", i, ops[i], want[i])
		}
	}
}

// TestChainMalformedAttributeAppliesOnError proves the chain half of AC-28: a
// member whose read meets a malformed LOCAL_PREF fails the route under its
// on-error. Fail-closed rejects as a failure, not a policy decision; fail-open
// passes the route on with none of that member's sets, and no value read from
// the malformed attribute reaches the evaluator.
func TestChainMalformedAttributeAppliesOnError(t *testing.T) {
	malformed := pathAttr(0x40, attribute.AttrLocalPref, []byte{0, 0, 0, 0, 1})

	closed := compile(t, policy.FinalAccept,
		policy.Instance{Name: "closed", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 200)})
	outcome, sets := run(t, closed, update(t, malformed))
	if outcome != (Outcome{Action: OutcomeReject, Failed: true, Member: "closed"}) {
		t.Fatalf("fail-closed outcome = %+v, want a failed reject naming the member", outcome)
	}
	if !sets.Folded().Empty() {
		t.Fatal("a failed member's sets were kept")
	}
	if closed.Refused() != 1 {
		t.Fatalf("refused = %d, want 1", closed.Refused())
	}

	open := compile(t, policy.FinalAccept, policy.Instance{Name: "open", Type: typeFailOpen})
	if outcome, _ := run(t, open, update(t, malformed)); outcome.Action != OutcomeAccept || outcome.Failed {
		t.Fatalf("fail-open outcome = %+v, want accept", outcome)
	}
	if open.Refused() != 1 {
		t.Fatalf("fail-open refused = %d, want 1 (counted though accepted)", open.Refused())
	}
}

// TestChainRefusesUndeclaredSet proves D-9 in process: a member that writes a
// field it did not declare fails its route under on-error, nothing of its sets
// applies, and the failure is counted.
func TestChainRefusesUndeclaredSet(t *testing.T) {
	chain := compile(t, policy.FinalAccept, policy.Instance{Name: "liar", Type: typeUndeclared})
	outcome, sets := run(t, chain, update(t, localPref(100)))
	if outcome != (Outcome{Action: OutcomeReject, Failed: true, Member: "liar"}) {
		t.Fatalf("outcome = %+v, want a failed reject naming the member", outcome)
	}
	if !sets.Folded().Empty() {
		t.Fatal("the undeclared set was applied")
	}
	if chain.Refused() != 1 {
		t.Fatalf("refused = %d, want 1", chain.Refused())
	}
}

// TestChainFinalReject proves the final decides after the last member, and a
// final reject is a policy decision (not Failed) that drops the sets.
func TestChainFinalReject(t *testing.T) {
	chain := compile(t, policy.FinalReject,
		policy.Instance{Name: "lp", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 1)})
	outcome, sets := run(t, chain, update(t, localPref(100)))
	if outcome != (Outcome{Action: OutcomeReject}) {
		t.Fatalf("outcome = %+v, want a policy reject", outcome)
	}
	if !sets.Folded().Empty() {
		t.Fatal("sets kept under final reject")
	}
}

// TestCompileRefuses proves Compile refuses, by name, a type with no BGP
// evaluator, a missing final, an unknown name, and a chain CheckChain refuses.
func TestCompileRefuses(t *testing.T) {
	reg, err := policy.NewRegistry(
		policy.Instance{Name: "noeval", Type: typeNoEval},
		policy.Instance{Name: "lp", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 1)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want string
		names      []string
		final      policy.Final
		attachment string
	}{
		{"no evaluator", "no in-process BGP evaluator", []string{"noeval"}, policy.FinalAccept, testAttachment},
		{"no final", "missing or invalid final", []string{"lp"}, 0, testAttachment},
		{"unknown name", "is not a filter defined", []string{"missing"}, policy.FinalAccept, testAttachment},
		{"unregistered attachment", "no attachment", []string{"lp"}, policy.FinalAccept, "policychain-test nowhere"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			members := make([]Member, len(tc.names))
			for i, name := range tc.names {
				members[i] = Member{Name: name}
			}
			_, err := Compile(tc.attachment, reg, members, tc.final, Bases{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to say %q", err, tc.want)
			}
		})
	}
}

// TestChainRunAllocationFree proves a pass run, with sets folded, allocates
// nothing: one view and one RunSets serve every route of a goroutine.
func TestChainRunAllocationFree(t *testing.T) {
	chain := compile(t, policy.FinalAccept,
		policy.Instance{Name: "lp", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 200)},
		policy.Instance{Name: "med", Type: typeSetMED, Config: numericConfig(NumericActionIncrement, 1)},
		policy.Instance{Name: "comm", Type: typeCommunity})
	wire := update(t, localPref(100), med(5))
	var view policyview.View
	var sets RunSets
	var acc filterapi.ModAccumulator
	current := Current{LocalPreference: 100, HasLocalPreference: true, MED: 5, HasMED: true}
	allocs := testing.AllocsPerRun(100, func() {
		view.Reset(wire, policyview.Session{})
		if outcome := chain.Run(&view, &sets); outcome.Action != OutcomeAccept {
			t.Fatalf("outcome = %+v", outcome)
		}
		acc.Reset()
		if err := sets.Folded().ApplyTo(&acc, current); err != nil {
			t.Fatal(err)
		}
		view.Clear()
	})
	if allocs != 0 {
		t.Fatalf("a chain run allocated %.1f times, want 0", allocs)
	}
}

// TestSetsCommunityBound proves the community lists are bounded and the bound
// is an error, never a silent drop.
func TestSetsCommunityBound(t *testing.T) {
	var sets Sets
	for i := range communitiesMax {
		if err := sets.AddCommunity(uint32(i)); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	if err := sets.AddCommunity(99); !errors.Is(err, ErrTooManyCommunities) {
		t.Fatalf("add past the bound: err = %v, want ErrTooManyCommunities", err)
	}
}

// TestChainReadAfterSet proves read-after-set: a later member reads the value
// an earlier member set, not the received one. The route carries
// LOCAL_PREF 7; the first member sets 100 and increments resolve against it;
// the gate passes only a LOCAL_PREF of 100.
func TestChainReadAfterSet(t *testing.T) {
	chain := compile(t, policy.FinalAccept,
		policy.Instance{Name: "lp-set", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 100)},
		policy.Instance{Name: "gate", Type: typeGate100})
	if outcome, _ := run(t, chain, update(t, localPref(7))); outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept: the gate must read the 100 the first member set", outcome)
	}

	chain = compile(t, policy.FinalAccept,
		policy.Instance{Name: "lp-inc", Type: typeSetLocal, Config: numericConfig(NumericActionIncrement, 93)},
		policy.Instance{Name: "gate", Type: typeGate100})
	outcome, sets := run(t, chain, update(t, localPref(7)))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept: 7 incremented by 93 reads as 100", outcome)
	}
	if ops := applied(t, sets.Folded(), Current{}); len(ops) != 1 || !bytes.Equal(ops[0].Buf, []byte{0, 0, 0, 100}) {
		t.Fatalf("ops = %+v, want one local-preference set of 100", ops)
	}
}
