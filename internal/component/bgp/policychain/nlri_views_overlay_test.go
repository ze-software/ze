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
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// NLRI-views overlay test type: a member that records what it reads for
// family and next-hop, under an attachment that reads prefix, family and
// next-hop and lets a member drop prefixes and strip families.
const (
	typeNLRIRecorder = "policychain-test-nlri-recorder" // reads family and next-hop

	testAttachmentNLRIViews = "policychain-test nlri-views" // reads prefix, family, next-hop; sets prefix and family
)

// nlriRead is what one recorder member read: the family field and the
// next-hop field, reduced to what a text filter would see of each.
type nlriRead struct {
	familyPresent  bool
	bodyFamily     family.Family // zero when family names no body NLRI
	mpFamily       family.Family // zero when family carries no MP_REACH_NLRI
	nextHopPresent bool
	nextHopWire    []byte // the NEXT_HOP attribute value
	nextHopMP      bool   // next-hop still carries an MP_REACH_NLRI next hop
}

// nlriReads collects each recorder member's read, in chain order.
var nlriReads []nlriRead

func init() {
	prefix, fam, nextHop := policy.FieldPrefix, policy.FieldFamily, policy.FieldNextHop
	policy.MustRegisterAttachment("bgp", testAttachmentNLRIViews,
		[]policy.Field{prefix, fam, nextHop}, []policy.Field{prefix, fam})
	policy.MustRegisterFilterType(typeNLRIRecorder, []policy.Field{fam, nextHop}, nil)
	MustRegisterEvaluator(typeNLRIRecorder, rpc.OnErrorReject, func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		famReader, err := policyview.ReaderFor(fam)
		if err != nil {
			return nil, err
		}
		nhReader, err := policyview.ReaderFor(nextHop)
		if err != nil {
			return nil, err
		}
		return func(view *policyview.View, _ *Sets) (Decision, error) {
			famValue, err := view.Read(famReader)
			if err != nil {
				return DecisionUnspecified, err
			}
			nhValue, err := view.Read(nhReader)
			if err != nil {
				return DecisionUnspecified, err
			}
			read := nlriRead{
				familyPresent:  famValue.Present(),
				bodyFamily:     famValue.Family(),
				nextHopPresent: nhValue.Present(),
				nextHopWire:    nhValue.Wire(),
				nextHopMP:      nhValue.MPReach() != nil,
			}
			if mp := famValue.MPReach(); mp != nil {
				read.mpFamily = mp.Family()
			}
			nlriReads = append(nlriReads, read)
			return DecisionPass, nil
		}, nil
	})
}

// nextHopValue is the NEXT_HOP the NLRI-views UPDATE carries.
var nextHopValue = []byte{192, 0, 2, 1}

// nlriViewsUpdate builds an UPDATE body carrying body as its NLRI, a NEXT_HOP
// attribute when nextHop is set, and an MP_REACH_NLRI attribute whose value
// is mp when mp is not nil.
func nlriViewsUpdate(t *testing.T, nextHop bool, body, mp []byte) *wireu.WireUpdate {
	t.Helper()
	var attrs []byte
	if nextHop {
		attrs = pathAttr(0x40, attribute.AttrNextHop, nextHopValue)
	}
	if mp != nil {
		attrs = append(attrs, pathAttr(0x80, attribute.AttrMPReachNLRI, mp)...)
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

// nlriViewsChain compiles types, in order, on the NLRI-views attachment.
func nlriViewsChain(t *testing.T, types ...string) *Chain {
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
	chain, err := Compile(testAttachmentNLRIViews, reg, members, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

// TestChainFamilyAndNextHopFollowPrefix proves a later member's family and
// next-hop agree with the prefix an earlier member's drops or family strip
// left, as a later text filter's do: applyFilterDelta replaces the nlri slot
// (reactor/filter_chain.go), so the next text filter sees no block of a
// family whose NLRI are gone, while the next-hop token renders the NEXT_HOP
// attribute alone (AppendAttrsForFilter, reactor/filter_format.go), which no
// NLRI change removes. An MP_REACH_NLRI left with no NLRI leaves the UPDATE
// (libraryNLRIOps), so its family and its next hop leave both reads.
//
// Method: a recorder member reads family and next-hop before the change (so
// the view caches both) and after it; a case with no earlier recorder proves
// the read that decodes after the change. The view's decode counts must show
// the NEXT_HOP attribute decoded once.
func TestChainFamilyAndNextHopFollowPrefix(t *testing.T) {
	body := joined(nlriV4First, nlriV4Second)
	mp := joined(mpReachHead, nlriV6First, nlriV6Second)
	received := nlriRead{familyPresent: true, bodyFamily: family.IPv4Unicast, mpFamily: family.IPv6Unicast,
		nextHopPresent: true, nextHopWire: nextHopValue, nextHopMP: true}
	tests := []struct {
		name    string
		nextHop bool
		body    []byte
		mp      []byte
		types   []string
		want    []nlriRead
	}{
		{name: "strip-ipv6", nextHop: true, body: body, mp: mp,
			types: []string{typeNLRIRecorder, typeStripIPv6, typeNLRIRecorder},
			want: []nlriRead{received, {familyPresent: true, bodyFamily: family.IPv4Unicast,
				nextHopPresent: true, nextHopWire: nextHopValue}}},
		{name: "strip-ipv6-read-after-only", nextHop: true, body: body, mp: mp,
			types: []string{typeStripIPv6, typeNLRIRecorder},
			want: []nlriRead{{familyPresent: true, bodyFamily: family.IPv4Unicast,
				nextHopPresent: true, nextHopWire: nextHopValue}}},
		{name: "strip-ipv4", nextHop: true, body: body, mp: mp,
			types: []string{typeNLRIRecorder, typeStripIPv4, typeNLRIRecorder},
			want: []nlriRead{received, {familyPresent: true, mpFamily: family.IPv6Unicast,
				nextHopPresent: true, nextHopWire: nextHopValue, nextHopMP: true}}},
		{name: "drop-every-mp-reach-nlri", nextHop: true, body: body, mp: mp,
			types: []string{typeNLRIRecorder, typeDropFirstMPReach, typeDropFirstMPReach, typeNLRIRecorder},
			want: []nlriRead{received, {familyPresent: true, bodyFamily: family.IPv4Unicast,
				nextHopPresent: true, nextHopWire: nextHopValue}}},
		{name: "drop-every-body-nlri", nextHop: true, body: body, mp: mp,
			types: []string{typeNLRIRecorder, typeDropFirstAnnounce, typeDropFirstAnnounce, typeNLRIRecorder},
			want: []nlriRead{received, {familyPresent: true, mpFamily: family.IPv6Unicast,
				nextHopPresent: true, nextHopWire: nextHopValue, nextHopMP: true}}},
		{name: "strip-both-keeps-next-hop-attribute", nextHop: true, body: body, mp: mp,
			types: []string{typeNLRIRecorder, typeStripIPv4, typeStripIPv6, typeNLRIRecorder},
			want:  []nlriRead{received, {nextHopPresent: true, nextHopWire: nextHopValue}}},
		{name: "strip-mp-only-update", mp: mp,
			types: []string{typeNLRIRecorder, typeStripIPv6, typeNLRIRecorder},
			want: []nlriRead{{familyPresent: true, mpFamily: family.IPv6Unicast, nextHopPresent: true, nextHopMP: true},
				{}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nlriReads = nil
			chain := nlriViewsChain(t, tc.types...)
			var view policyview.View
			var sets RunSets
			view.Reset(nlriViewsUpdate(t, tc.nextHop, tc.body, tc.mp), policyview.Session{})
			defer view.Clear()
			if outcome := chain.Run(&view, &sets); outcome.Action != OutcomeAccept {
				t.Fatalf("outcome = %+v, want accept", outcome)
			}
			if !slices.EqualFunc(nlriReads, tc.want, nlriReadEqual) {
				t.Fatalf("reads = %+v\nwant    %+v", nlriReads, tc.want)
			}
			if decodes := view.Decodes(); decodes.Attributes() != decodes.Distinct() {
				t.Fatalf("attribute decodes = %d over %d distinct: an overlay decoded an attribute twice",
					decodes.Attributes(), decodes.Distinct())
			}
		})
	}
}

func nlriReadEqual(a, b nlriRead) bool {
	return a.familyPresent == b.familyPresent && a.bodyFamily == b.bodyFamily && a.mpFamily == b.mpFamily &&
		a.nextHopPresent == b.nextHopPresent && bytes.Equal(a.nextHopWire, b.nextHopWire) && a.nextHopMP == b.nextHopMP
}
