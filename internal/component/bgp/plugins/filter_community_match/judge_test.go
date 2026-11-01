package filter_community_match

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// judgeList answers what a BGP chain's match instance, whose community
// condition names a community-match list of listCfg (the instance config as
// delivered), decides for an UPDATE carrying attrs and one IPv4 prefix:
// actionAccept when the chain accepts it, actionReject when it refuses it.
func judgeList(t *testing.T, listCfg map[string]any, attrs []byte) action {
	t.Helper()
	return judgeWire(t, listCfg, conditionUpdate(t, attrs))
}

// judgeWire is judgeList over an UPDATE the caller built, so a test can judge
// one that carries no IPv4 NLRI at all.
func judgeWire(t *testing.T, listCfg map[string]any, update *wireu.WireUpdate) action {
	t.Helper()
	family.RegisterTestFamilies()
	reg, err := policy.NewRegistry(
		policy.Instance{Name: "cl", Type: filterTypeName, Config: listCfg},
		policy.Instance{Name: "m", Type: policy.MatchFilterType,
			Config: map[string]any{policy.FieldCommunity.String(): "cl"}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(testAttachmentBGP, reg, []policychain.Member{{Name: "m"}}, policy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var view policyview.View
	view.Reset(update, policyview.Session{})
	defer view.Clear()
	outcome := chain.Run(&view, new(policychain.RunSets))
	if outcome.Failed {
		t.Fatalf("chain failed the route: %+v", outcome)
	}
	if outcome.Action == policychain.OutcomeAccept {
		return actionAccept
	}
	return actionReject
}

// standardCommunities is a COMMUNITIES attribute (RFC 1997) of values.
func standardCommunities(values ...uint32) []byte {
	var value []byte
	for _, v := range values {
		value = binary.BigEndian.AppendUint32(value, v)
	}
	return optionalTransitive(8, value)
}

// extendedCommunities is an EXTENDED_COMMUNITIES attribute (RFC 4360).
func extendedCommunities(values ...[8]byte) []byte {
	var value []byte
	for _, v := range values {
		value = append(value, v[:]...)
	}
	return optionalTransitive(16, value)
}

// largeCommunity is a LARGE_COMMUNITY attribute (RFC 8092) of one value.
func largeCommunity(global, local1, local2 uint32) []byte {
	value := binary.BigEndian.AppendUint32(nil, global)
	value = binary.BigEndian.AppendUint32(value, local1)
	value = binary.BigEndian.AppendUint32(value, local2)
	return optionalTransitive(32, value)
}

// flowSpecReach is an MP_REACH_NLRI (extended length) of IPv4 FlowSpec, SAFI
// 133 or, with the 8-octet route distinguisher first, 134 (RFC 8955 Section
// 4), carrying one rule whose destination component is 10.1.0.0/24.
func flowSpecReach(safi uint8) []byte {
	rule := []byte{1, 24, 10, 1, 0}
	if safi == 134 {
		rule = append(make([]byte, 8), rule...)
	}
	value := binary.BigEndian.AppendUint16(nil, 1)
	value = append(value, safi, 0, 0, byte(len(rule)))
	value = append(value, rule...)
	attr := []byte{0x90, 14}
	attr = binary.BigEndian.AppendUint16(attr, uint16(len(value))) //nolint:gosec // G115: a test attribute
	return append(attr, value...)
}

// flowSpecUpdate builds an UPDATE whose only NLRI is the FlowSpec rule in
// attrs' MP_REACH_NLRI: no withdrawn routes and no IPv4 unicast NLRI field, the
// shape a FlowSpec-only peer sends (RFC 8955 Section 4).
func flowSpecUpdate(t *testing.T, attrs []byte) *wireu.WireUpdate {
	t.Helper()
	payload := binary.BigEndian.AppendUint16(nil, 0)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(attrs))) //nolint:gosec // G115: a test attribute block
	payload = append(payload, attrs...)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(payload, id)
}
