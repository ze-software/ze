package policychain

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Prefix overlay test types: members that drop the NLRI entry at offset 0 (or
// at a stray offset) of a section as the view answers it, and a gate that
// passes only a route whose prefix reads exactly prefixGateBody and
// prefixGateMP.
const (
	typeDropFirstAnnounce = "policychain-test-drop-first-announce" // sets prefix
	typeDropFirstMPReach  = "policychain-test-drop-first-mpreach"  // sets prefix
	typeDropStray         = "policychain-test-drop-stray"          // sets prefix
	typePrefixGate        = "policychain-test-prefix-gate"         // reads prefix

	testAttachmentPrefix = "policychain-test prefix" // reads and sets prefix
)

// prefixGateBody and prefixGateMP are the body NLRI and the MP_REACH_NLRI
// attribute value the gate passes; each test sets them.
var prefixGateBody, prefixGateMP []byte

func init() {
	prefix := policy.FieldPrefix
	policy.MustRegisterAttachment("bgp", testAttachmentPrefix, []policy.Field{prefix}, []policy.Field{prefix})
	policy.MustRegisterFilterType(typeDropFirstAnnounce, nil, []policy.Field{prefix})
	policy.MustRegisterFilterType(typeDropFirstMPReach, nil, []policy.Field{prefix})
	policy.MustRegisterFilterType(typeDropStray, nil, []policy.Field{prefix})
	policy.MustRegisterFilterType(typePrefixGate, []policy.Field{prefix}, nil)
	dropAt := func(section NLRISection, offset uint16) EvaluatorCompiler {
		return func(string, map[string]any) (Evaluator, error) {
			return func(_ *policyview.View, sets *Sets) (Decision, error) {
				return DecisionPass, sets.DropNLRI(section, offset)
			}, nil
		}
	}
	MustRegisterEvaluator(typeDropFirstAnnounce, rpc.OnErrorReject, dropAt(NLRISectionAnnounce, 0))
	MustRegisterEvaluator(typeDropFirstMPReach, rpc.OnErrorReject, dropAt(NLRISectionMPReach, 0))
	MustRegisterEvaluator(typeDropStray, rpc.OnErrorReject, dropAt(NLRISectionAnnounce, 1))
	MustRegisterEvaluator(typePrefixGate, rpc.OnErrorReject, func(string, map[string]any) (Evaluator, error) {
		reader, err := policyview.ReaderFor(prefix)
		if err != nil {
			return nil, err
		}
		return func(view *policyview.View, _ *Sets) (Decision, error) {
			value, err := view.Read(reader)
			if err != nil {
				return DecisionUnspecified, err
			}
			if bytes.Equal(value.Wire(), prefixGateBody) && bytes.Equal(value.MPReach(), prefixGateMP) {
				return DecisionPass, nil
			}
			return DecisionReject, nil
		}, nil
	})
}

// The NLRI the overlay tests carry: two IPv4 prefixes in the body, and two
// IPv6 prefixes in MP_REACH_NLRI behind a 16-octet next hop.
var (
	nlriV4First  = []byte{24, 203, 0, 113}
	nlriV4Second = []byte{24, 198, 51, 100}
	nlriV6First  = []byte{32, 0x20, 0x01, 0x0d, 0xb8}
	nlriV6Second = []byte{32, 0x20, 0x01, 0x0d, 0xb9}
	mpReachHead  = append([]byte{0, 2, 1, 16, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, 0)
)

// prefixUpdate builds an UPDATE body carrying body as its NLRI and, when mp
// is not nil, an MP_REACH_NLRI attribute whose value is mp.
func prefixUpdate(t *testing.T, body, mp []byte) *wireu.WireUpdate {
	t.Helper()
	var attrs []byte
	if mp != nil {
		attrs = pathAttr(0x80, attribute.AttrMPReachNLRI, mp)
	}
	payload := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(attrs)))
	payload = append(payload, attrs...)
	payload = append(payload, body...)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(payload, id)
}

func prefixChain(t *testing.T, types ...string) *Chain {
	t.Helper()
	instances := make([]policy.Instance, len(types))
	members := make([]Member, len(types))
	for i, typ := range types {
		name := string(rune('a' + i))
		instances[i] = policy.Instance{Name: name, Type: typ}
		members[i] = Member{Name: name}
	}
	reg, err := policy.NewRegistry(instances...)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := Compile(testAttachmentPrefix, reg, members, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

func joined(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

// TestChainPrefixReadsSubsetAfterDrop proves a later member reads the NLRI an
// earlier member's drop left, as a later text filter reads the subset an
// earlier text filter's NLRI delta wrote (applyFilterDelta replaces the nlri
// slot, reactor/filter_chain.go). It also proves a later member's drop, given
// at an offset of the section as the view answers it, folds at the entry's
// offset in the received section, which is where the reactor applies it.
//
// Method: the gate passes only on the subset; a chain that answered the
// received NLRI after the drop makes it reject.
func TestChainPrefixReadsSubsetAfterDrop(t *testing.T) {
	mp := joined(mpReachHead, nlriV6First, nlriV6Second)
	tests := []struct {
		name      string
		types     []string
		body, mp  []byte
		wantBody  []byte
		wantMP    []byte
		announce  []uint16
		mpReach   []uint16
		wantCount int
	}{
		{name: "body-drop", types: []string{typeDropFirstAnnounce, typePrefixGate},
			body: joined(nlriV4First, nlriV4Second), wantBody: nlriV4Second, announce: []uint16{0}},
		{name: "mp-reach-drop", types: []string{typeDropFirstMPReach, typePrefixGate},
			mp: mp, wantMP: joined(mpReachHead, nlriV6Second), mpReach: []uint16{0}},
		{name: "second-drop-folds-at-received-offset",
			types: []string{typeDropFirstAnnounce, typeDropFirstAnnounce, typePrefixGate},
			body:  joined(nlriV4First, nlriV4Second), wantBody: []byte{}, announce: []uint16{0, 4}},
		{name: "both-sections", types: []string{typeDropFirstAnnounce, typeDropFirstMPReach, typePrefixGate},
			body: joined(nlriV4First, nlriV4Second), mp: mp,
			wantBody: nlriV4Second, wantMP: joined(mpReachHead, nlriV6Second), announce: []uint16{0}, mpReach: []uint16{0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prefixGateBody, prefixGateMP = tc.wantBody, tc.wantMP
			chain := prefixChain(t, tc.types...)
			outcome, sets := run(t, chain, prefixUpdate(t, tc.body, tc.mp))
			if outcome.Action != OutcomeAccept {
				t.Fatalf("outcome = %+v, want accept: the gate did not read the subset", outcome)
			}
			if got := sets.Folded().NLRIDrops(NLRISectionAnnounce); !slices.Equal(got, tc.announce) {
				t.Fatalf("announce drops = %v, want %v", got, tc.announce)
			}
			if got := sets.Folded().NLRIDrops(NLRISectionMPReach); !slices.Equal(got, tc.mpReach) {
				t.Fatalf("mp-reach drops = %v, want %v", got, tc.mpReach)
			}
		})
	}
}

// TestChainPrefixDropRefusesStrayOffset proves a drop at an offset that
// starts no entry of the section the view answers fails the member under its
// on-error (reject here) and counts it, rather than folding an offset the
// reactor would later refuse.
func TestChainPrefixDropRefusesStrayOffset(t *testing.T) {
	prefixGateBody, prefixGateMP = nlriV4Second, nil
	chain := prefixChain(t, typeDropFirstAnnounce, typeDropStray, typePrefixGate)
	outcome, _ := run(t, chain, prefixUpdate(t, joined(nlriV4First, nlriV4Second), nil))
	if outcome.Action != OutcomeReject || !outcome.Failed || outcome.Member != "b" {
		t.Fatalf("outcome = %+v, want member b failed and the route rejected", outcome)
	}
	if chain.Refused() != 1 {
		t.Fatalf("refused = %d, want 1", chain.Refused())
	}
}

// TestChainPrefixReadAllocationFree proves a chain whose members drop nothing
// reads prefix from the wire and allocates nothing for it.
func TestChainPrefixReadAllocationFree(t *testing.T) {
	prefixGateBody, prefixGateMP = joined(nlriV4First, nlriV4Second), nil
	chain := prefixChain(t, typePrefixGate, typePrefixGate)
	wire := prefixUpdate(t, prefixGateBody, nil)
	var view policyview.View
	var sets RunSets
	allocs := testing.AllocsPerRun(100, func() {
		view.Reset(wire, policyview.Session{})
		if outcome := chain.Run(&view, &sets); outcome.Action != OutcomeAccept {
			t.Fatalf("outcome = %+v", outcome)
		}
		view.Clear()
	})
	if allocs != 0 {
		t.Fatalf("allocs = %v, want 0", allocs)
	}
}
