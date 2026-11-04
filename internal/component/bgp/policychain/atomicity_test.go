package policychain

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// atomicChain uses the compiled chain's real readers and run/apply machinery;
// closures name the exact member changes without adding test-only filter types.
func atomicChain(t *testing.T, evaluators ...Evaluator) *Chain {
	t.Helper()
	chain := compile(t, policy.FinalAccept)
	for i, evaluate := range evaluators {
		chain.members = append(chain.members, member{name: string(rune('a' + i)),
			evaluate: evaluate, onError: rpc.OnErrorAccept, settable: ^setMask(0)})
	}
	return chain
}

func atomicRead(t *testing.T, view *policyview.View, field policy.Field) policyview.Value {
	t.Helper()
	reader, err := policyview.ReaderFor(field)
	if err != nil {
		t.Fatal(err)
	}
	value, err := view.Read(reader)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// TestChainFailOpenMergeAtomicity proves merge overflow contributes neither
// numeric, community nor metadata changes to the next member or final apply,
// while the preceding member and a succeeding member both still apply.
func TestChainFailOpenMergeAtomicity(t *testing.T) {
	metaField, ok := policyview.MetadataNamed("rebuilt")
	if !ok {
		t.Fatal("rebuilt metadata is not registered")
	}
	var want []byte
	for value := uint32(1); value <= 15; value++ {
		want = binary.BigEndian.AppendUint32(want, value)
	}
	meta := map[string]any{"rebuilt": false}
	prior := func(_ *policyview.View, sets *Sets) (Decision, error) {
		for value := uint32(1); value <= 15; value++ {
			if err := sets.AddCommunity(value); err != nil {
				return DecisionUnspecified, err
			}
		}
		return DecisionPass, sets.LocalPreference(NumericActionSet, 150)
	}
	failed := func(_ *policyview.View, sets *Sets) (Decision, error) {
		if err := sets.LocalPreference(NumericActionSet, 200); err != nil {
			return DecisionUnspecified, err
		}
		if err := sets.AddCommunity(100); err != nil {
			return DecisionUnspecified, err
		}
		if err := sets.AddCommunity(101); err != nil {
			return DecisionUnspecified, err
		}
		return DecisionPass, sets.SetMetadata(metaField, true)
	}
	reads := 0
	gate := func(view *policyview.View, sets *Sets) (Decision, error) {
		reads++
		number, ok := atomicRead(t, view, policy.FieldLocalPreference).Number()
		if !ok || number != 150 {
			t.Errorf("next member local preference = (%d, %v), want (150, true)", number, ok)
		}
		if got := atomicRead(t, view, policy.FieldCommunity).Wire(); !bytes.Equal(got, want) {
			t.Errorf("next member community = %x, want %x", got, want)
		}
		if meta["rebuilt"] != false {
			t.Errorf("failed member wrote metadata: %v", meta)
		}
		return DecisionPass, sets.MED(NumericActionSet, 77)
	}
	chain := atomicChain(t, prior, failed, failed, failed, gate)
	var view policyview.View
	var sets RunSets
	view.Reset(update(t, localPref(100)), policyview.Session{}.WithImportMetadata(meta))
	defer view.Clear()
	outcome := chain.Run(&view, &sets)
	if outcome.Action != OutcomeAccept || reads != 1 || len(sets.Failures()) != 3 {
		t.Fatalf("outcome=%+v reads=%d failures=%v", outcome, reads, sets.Failures())
	}
	ops := applied(t, sets.Folded(), Current{})
	var gotCommunity []byte
	for _, op := range ops {
		switch attribute.AttributeCode(op.Code) {
		case attribute.AttrCommunity:
			gotCommunity = append(gotCommunity, op.Buf...)
		case attribute.AttrLocalPref:
			if got := binary.BigEndian.Uint32(op.Buf); got != 150 {
				t.Errorf("applied local preference=%d, want 150", got)
			}
		case attribute.AttrMED:
			if got := binary.BigEndian.Uint32(op.Buf); got != 77 {
				t.Errorf("succeeding member MED=%d, want 77", got)
			}
		}
	}
	if !bytes.Equal(gotCommunity, want) || len(ops) != 17 {
		t.Errorf("applied communities=%x operations=%d, want %x and 17", gotCommunity, len(ops), want)
	}
}

// TestChainFailOpenOverlayAtomicity exercises failures after successful
// numeric/list/scalar overlays and after body/MP prefix rewrites. Repeated
// failed members reuse the alternating buffers; the accepted subsets, scalar
// values and metadata must survive before a succeeding drop is translated.
func TestChainFailOpenOverlayAtomicity(t *testing.T) {
	for _, failure := range []string{"stray-mp-drop", "metadata-read-only"} {
		t.Run(failure, func(t *testing.T) {
			metaField, ok := policyview.MetadataNamed("rebuilt")
			if !ok {
				t.Fatal("rebuilt metadata is not registered")
			}
			large := attribute.LargeCommunity{GlobalAdmin: 1, LocalData1: 2, LocalData2: 3}
			extended := attribute.ExtendedCommunity{0, 2, 0, 0, 0, 0, 0, 1}
			setValues := func(sets *Sets, value uint32) error {
				if err := sets.LocalPreference(NumericActionSet, value); err != nil {
					return err
				}
				if err := sets.AddCommunity(value); err != nil {
					return err
				}
				if err := sets.Origin(attribute.Origin(value % 3)); err != nil {
					return err
				}
				if err := sets.NextHop(netip.AddrFrom4([4]byte{192, 0, 2, byte(value)})); err != nil {
					return err
				}
				if err := sets.AIGP(NumericActionSet, value); err != nil {
					return err
				}
				if err := sets.AddLargeCommunity(large); err != nil {
					return err
				}
				return sets.AddExtendedCommunity(extended)
			}
			prior := func(_ *policyview.View, sets *Sets) (Decision, error) {
				if err := setValues(sets, 150); err != nil {
					return DecisionUnspecified, err
				}
				if err := sets.DropNLRI(NLRISectionAnnounce, 0); err != nil {
					return DecisionUnspecified, err
				}
				return DecisionPass, sets.DropNLRI(NLRISectionMPReach, 0)
			}
			failed := func(_ *policyview.View, sets *Sets) (Decision, error) {
				if err := setValues(sets, 200); err != nil {
					return DecisionUnspecified, err
				}
				if err := sets.RemoveCommunity(150); err != nil {
					return DecisionUnspecified, err
				}
				if err := sets.DropNLRI(NLRISectionAnnounce, 0); err != nil {
					return DecisionUnspecified, err
				}
				offset := uint16(0)
				if failure == "stray-mp-drop" {
					offset = 1
				}
				if err := sets.DropNLRI(NLRISectionMPReach, offset); err != nil {
					return DecisionUnspecified, err
				}
				return DecisionPass, sets.SetMetadata(metaField, true)
			}
			body := joined(nlriV4First, nlriV4Second, []byte{24, 192, 0, 2}, []byte{24, 192, 0, 3})
			mp := joined(mpReachHead, nlriV6First, nlriV6Second, []byte{32, 0x20, 1, 0xdb, 0xba})
			var aigp [attribute.AIGPWireLen]byte
			attribute.WriteAIGPMetric(aigp[:], 0, 150)
			reads := 0
			gate := func(view *policyview.View, sets *Sets) (Decision, error) {
				reads++
				checks := []struct {
					field policy.Field
					want  []byte
				}{
					{policy.FieldCommunity, []byte{0, 0, 0, 150}},
					{policy.FieldOrigin, []byte{0}},
					{policy.FieldNextHop, []byte{192, 0, 2, 150}},
					{policy.FieldLargeCommunity, largeAppend(nil, large)},
					{policy.FieldExtendedCommunity, extended[:]},
					{policy.FieldAIGP, aigp[:]},
				}
				for _, check := range checks {
					if got := atomicRead(t, view, check.field).Wire(); !bytes.Equal(got, check.want) {
						t.Errorf("next member %s=%x, want %x", check.field, got, check.want)
					}
				}
				for _, field := range []policy.Field{policy.FieldLocalPreference} {
					if number, ok := atomicRead(t, view, field).Number(); !ok || number != 150 {
						t.Errorf("next member %s=(%d,%v), want (150,true)", field, number, ok)
					}
				}
				prefix := atomicRead(t, view, policy.FieldPrefix)
				if !bytes.Equal(prefix.Wire(), body[4:]) || !bytes.Equal(prefix.MPReach(), joined(mpReachHead, mp[len(mpReachHead)+5:])) {
					t.Errorf("next member prefix body=%x MP=%x, want accepted subsets", prefix.Wire(), prefix.MPReach())
				}
				if err := sets.DropNLRI(NLRISectionAnnounce, 0); err != nil {
					return DecisionUnspecified, err
				}
				return DecisionPass, sets.MED(NumericActionSet, 77)
			}
			chain := atomicChain(t, prior, failed, failed, failed, gate)
			meta := map[string]any{"rebuilt": false}
			var view policyview.View
			var sets RunSets
			wire := prefixUpdate(t, body, mp)
			view.Reset(wire, policyview.Session{}.WithExportMetadata(meta))
			defer view.Clear()
			outcome := chain.Run(&view, &sets)
			if outcome.Action != OutcomeAccept || reads != 1 || len(sets.Failures()) != 3 {
				t.Fatalf("outcome=%+v reads=%d failures=%v", outcome, reads, sets.Failures())
			}
			if got := sets.Folded().NLRIDrops(NLRISectionAnnounce); !slices.Equal(got, []uint16{0, 4}) {
				t.Errorf("folded body drops=%v, want [0 4]", got)
			}
			if got := sets.Folded().NLRIDrops(NLRISectionMPReach); !slices.Equal(got, []uint16{0}) {
				t.Errorf("folded MP drops=%v, want [0]", got)
			}
			if meta["rebuilt"] != false {
				t.Errorf("failed member metadata=%v, want rebuilt false", meta)
			}
			var acc filterapi.ModAccumulator
			if err := sets.Folded().ApplyTo(&acc, Current{}); err != nil {
				t.Fatal(err)
			}
			if len(acc.Ops()) != 8 {
				t.Errorf("applied operations=%+v, want seven accepted attributes and succeeding MED", acc.Ops())
			}
		})
	}
}

// TestChainFailOpenIntentMergeAtomicity proves a later intent-merge refusal
// cannot retain earlier scalar/community folds or poison translated offsets.
// The succeeding drop still names the accepted subset, not the failed one.
func TestChainFailOpenIntentMergeAtomicity(t *testing.T) {
	body := joined(nlriV4First, nlriV4Second, []byte{24, 192, 0, 2}, []byte{24, 192, 0, 3})
	prior := func(_ *policyview.View, sets *Sets) (Decision, error) {
		if err := sets.PrependASPath(ASPathPrependMax); err != nil {
			return DecisionUnspecified, err
		}
		return DecisionPass, sets.DropNLRI(NLRISectionAnnounce, 0)
	}
	failed := func(_ *policyview.View, sets *Sets) (Decision, error) {
		if err := sets.LocalPreference(NumericActionSet, 200); err != nil {
			return DecisionUnspecified, err
		}
		if err := sets.AddCommunity(100); err != nil {
			return DecisionUnspecified, err
		}
		if err := sets.PrependASPath(1); err != nil {
			return DecisionUnspecified, err
		}
		return DecisionPass, sets.DropNLRI(NLRISectionAnnounce, 0)
	}
	gate := func(view *policyview.View, sets *Sets) (Decision, error) {
		if value := atomicRead(t, view, policy.FieldLocalPreference); value.Present() {
			t.Error("failed member made LOCAL_PREF present")
		}
		if value := atomicRead(t, view, policy.FieldCommunity); value.Present() {
			t.Errorf("failed member community=%x", value.Wire())
		}
		if got := atomicRead(t, view, policy.FieldPrefix).Wire(); !bytes.Equal(got, body[4:]) {
			t.Errorf("accepted subset=%x, want %x", got, body[4:])
		}
		return DecisionPass, sets.DropNLRI(NLRISectionAnnounce, 0)
	}
	chain := atomicChain(t, prior, failed, failed, failed, gate)
	outcome, sets := run(t, chain, prefixUpdate(t, body, nil))
	if outcome.Action != OutcomeAccept || len(sets.Failures()) != 3 {
		t.Fatalf("outcome=%+v failures=%v", outcome, sets.Failures())
	}
	if got := sets.Folded().NLRIDrops(NLRISectionAnnounce); !slices.Equal(got, []uint16{0, 4}) {
		t.Errorf("received offsets=%v, want [0 4]", got)
	}
	if got := sets.Folded().ASPathPrepend(); got != ASPathPrependMax {
		t.Errorf("prepend=%d, want accepted %d", got, ASPathPrependMax)
	}
	if got := applied(t, sets.Folded(), Current{}); len(got) != 0 {
		t.Errorf("failed member applied attribute operations: %+v", got)
	}
}
