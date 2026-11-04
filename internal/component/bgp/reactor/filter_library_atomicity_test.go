package reactor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const libraryAtomicType = "reactor-library-atomicity"
const libraryAtomicAttachment = "reactor-library-atomicity import"

func init() {
	fields := []routepolicy.Field{routepolicy.FieldLocalPreference, routepolicy.FieldMED, routepolicy.FieldCommunity}
	routepolicy.MustRegisterAttachment("bgp", libraryAtomicAttachment, fields, fields)
	routepolicy.MustRegisterFilterType(libraryAtomicType, fields, fields)
	policychain.MustRegisterEvaluator(libraryAtomicType, rpc.OnErrorAccept,
		func(_ string, config map[string]any, _ routepolicy.InstanceResolver) (policychain.Evaluator, error) {
			evaluate, ok := config["evaluate"].(policychain.Evaluator)
			if !ok {
				return nil, errors.New("atomicity fixture has no evaluator")
			}
			return evaluate, nil
		})
}

// TestLibraryFailOpenMergeKeepsAcceptedWire runs Chain.Run through the import
// step and the existing modified-payload builder. The failed middle member
// must contribute no partial LOCAL_PREF/community edit to the actual UPDATE;
// the earlier communities and the succeeding MED remain on the wire.
func TestLibraryFailOpenMergeKeepsAcceptedWire(t *testing.T) {
	prior := policychain.Evaluator(func(_ *policyview.View, sets *policychain.Sets) (policychain.Decision, error) {
		for value := uint32(1); value <= 15; value++ {
			if err := sets.AddCommunity(value); err != nil {
				return policychain.DecisionUnspecified, err
			}
		}
		return policychain.DecisionPass, sets.LocalPreference(policychain.NumericActionSet, 150)
	})
	failed := policychain.Evaluator(func(_ *policyview.View, sets *policychain.Sets) (policychain.Decision, error) {
		if err := sets.LocalPreference(policychain.NumericActionSet, 200); err != nil {
			return policychain.DecisionUnspecified, err
		}
		if err := sets.AddCommunity(100); err != nil {
			return policychain.DecisionUnspecified, err
		}
		return policychain.DecisionPass, sets.AddCommunity(101)
	})
	succeeding := policychain.Evaluator(func(_ *policyview.View, sets *policychain.Sets) (policychain.Decision, error) {
		return policychain.DecisionPass, sets.MED(policychain.NumericActionSet, 77)
	})
	var instances []routepolicy.Instance
	var members []policychain.Member
	var refs []routepolicy.Ref
	for i, evaluate := range []policychain.Evaluator{prior, failed, failed, failed, succeeding} {
		name := string(rune('a' + i))
		instances = append(instances, routepolicy.Instance{Name: name, Type: libraryAtomicType,
			Config: map[string]any{"evaluate": evaluate}})
		members = append(members, policychain.Member{Name: name})
		refs = append(refs, routepolicy.Ref{Name: name})
	}
	reg, err := routepolicy.NewRegistry(instances...)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := policychain.Compile(libraryAtomicAttachment, reg, members, routepolicy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatal(err)
	}
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	received := libraryTestBody(100)
	want := textBody(t, r, received, func(acc *filterapi.ModAccumulator) {
		acc.OpCopy(uint8(attribute.AttrLocalPref), filterapi.AttrModSet, []byte{0, 0, 0, 150})
		for value := uint32(1); value <= 15; value++ {
			var community [4]byte
			binary.BigEndian.PutUint32(community[:], value)
			acc.OpCopy(uint8(attribute.AttrCommunity), filterapi.AttrModAdd, community[:])
		}
		acc.OpCopy(uint8(attribute.AttrMED), filterapi.AttrModSet, []byte{0, 0, 0, 77})
	})
	addr := netip.MustParseAddr("192.0.2.1")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: mustPolicy(t, refs, routepolicy.FinalAccept).WithLibrary(chain)})
	got := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(received), received)
	if !got.accept || !bytes.Equal(got.modifiedPayload, want) {
		t.Errorf("import accept=%v body=%x, want accepted body=%x", got.accept, got.modifiedPayload, want)
	}
}
