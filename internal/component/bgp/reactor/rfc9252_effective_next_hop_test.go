// Design: docs/architecture/wire/attributes.md -- Prefix-SID propagation.
package reactor

import (
	"bufio"
	"bytes"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC9252EffectiveNextHopControlsServiceTLVs observes the recipient Session's
// framed output after actual receive, cache publication, policy and forwarding.
// The next hop is a valid third party, not this speaker or the recipient. Raw
// export on an RS destination must take the production cached-forward fallback.
// Native IPv6 and VPNv6 L3 are the RFC 9252 Sections 5.2/5.4 service carriers.
// Legacy/mapped input and the extra L2 TLV are propagation robustness controls,
// not evidence for originating an SRv6 service on those carriers or for EVPN.
// RFC 9252 Section 2: "If the BGP next hop is unchanged during the advertisement,
// the SRv6 Service TLVs, including any unrecognized Types of Sub-TLV and
// Sub-Sub-TLV, SHOULD be propagated further." "In addition, all Reserved fields
// in the TLV, Sub-TLV, or Sub-Sub-TLV MUST be propagated unchanged."
// RFC 9252 Section 2: "Any received Sub-TLVs and Sub-Sub-TLVs that are
// unrecognized MUST be removed."
// RFC requirement: RFC9252-3.3-1 positive -- explicit third-party A-to-A and policy A-to-A retain Service TLVs, nonzero Reserved and unknown nested bytes in actual cached and RS recipient output.
// RFC requirement: RFC9252-3.3-1 negative -- explicit A-to-A and configured restoration of the received address must not strip Service TLVs or their Reserved/unknown nested bytes.
// RFC requirement: RFC9252-3.3-2 positive -- effective A-to-B changes remove Service TLVs and unknown nested fields, retaining route, unknown top-level TLVs, SRGB and normalized Label-Index on cached and RS output, including raw-policy fallback.
// RFC requirement: RFC9252-3.3-2 negative -- equal effective addresses do not remove received Service TLVs merely because configured next-hop rewriting is enabled.
// MUTATION: use configured mode as proof of change, compare policy output to
// itself, ignore raw replacement, or run policy again in the Session writer.
func TestRFC9252EffectiveNextHopControlsServiceTLVs(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, carrier := range []string{"legacy", "ipv6", "mapped", "vpn6", "pair-trim"} {
			for _, tc := range []rfc9252NextHopCase{
				{name: "auto-received", mode: NextHopAuto},
				{name: "unchanged-received", mode: NextHopUnchanged},
				{name: "explicit-equal", mode: NextHopExplicit},
				{name: "explicit-changed", mode: NextHopExplicit, explicitChange: true, changed: true},
				{name: "auto-policy-equal", mode: NextHopAuto, policy: true},
				{name: "auto-policy-changed", mode: NextHopAuto, policy: true, policyChanged: true, changed: true},
				{name: "unchanged-policy-equal", mode: NextHopUnchanged, policy: true},
				{name: "unchanged-policy-changed", mode: NextHopUnchanged, policy: true, policyChanged: true, changed: true},
				{name: "explicit-restores-original", mode: NextHopExplicit, policy: true, policyChanged: true},
			} {
				// Configured explicit VPN next-hop encoding is not the subject:
				// its policy operations below carry the complete valid zero-RD field.
				if carrier == "vpn6" && tc.mode == NextHopExplicit {
					continue
				}
				for _, rawPolicy := range []bool{false, true} {
					if rawPolicy && !tc.policy {
						continue
					}
					policy := "operation"
					if rawPolicy {
						policy = "raw"
					}
					t.Run(rail+"/"+carrier+"/"+tc.name+"/"+policy, func(t *testing.T) {
						tc.rawPolicy = rawPolicy
						rfc9252EffectiveNextHopCase(t, rail, carrier, tc)
					})
				}
			}
		}
		for _, rawPolicy := range []bool{false, true} {
			name := "operation"
			if rawPolicy {
				name = "raw"
			}
			t.Run(rail+"/service-only/"+name, func(t *testing.T) {
				rfc9252EffectiveNextHopCase(t, rail, "ipv6", rfc9252NextHopCase{
					mode: NextHopUnchanged, policy: true, policyChanged: true,
					changed: true, rawPolicy: rawPolicy, serviceOnly: true,
				})
			})
			for _, change := range []string{"neither", "legacy-only", "mp-only", "both"} {
				t.Run(rail+"/mixed/"+change+"/"+name, func(t *testing.T) {
					rfc9252MixedNextHopCase(t, rail, change, rawPolicy, nil)
				})
			}
		}
		t.Run(rail+"/legacy/absent-mp-companion", func(t *testing.T) {
			rfc9252EffectiveNextHopCase(t, rail, "legacy", rfc9252NextHopCase{
				mode: NextHopExplicit, explicit: netip.MustParseAddr("2001:db8:1::10"),
			})
		})
		for _, field := range []struct {
			name  string
			value []byte
		}{
			{name: "zero", value: []byte{}},
			{name: "eight", value: make([]byte, 8)},
		} {
			for _, tc := range []rfc9252NextHopCase{
				{name: "self-changed", mode: NextHopSelf, changed: true},
				{name: "explicit-changed", mode: NextHopExplicit, explicitChange: true, changed: true},
				{name: "explicit-equal", mode: NextHopExplicit},
			} {
				t.Run(rail+"/raw-mp-repair/"+field.name+"/"+tc.name, func(t *testing.T) {
					tc.policy, tc.rawPolicy = true, true
					tc.rawNextHop = field.value
					rfc9252EffectiveNextHopCase(t, rail, "ipv6", tc)
				})
			}
			for _, change := range []string{"neither", "mp-only"} {
				t.Run(rail+"/raw-mp-repair/"+field.name+"/mixed-"+change, func(t *testing.T) {
					rfc9252MixedNextHopCase(t, rail, change, true, field.value)
				})
			}
		}
	}
}

type rfc9252NextHopCase struct {
	name           string
	mode           uint8
	policy         bool
	policyChanged  bool
	explicitChange bool
	changed        bool
	rawPolicy      bool
	serviceOnly    bool
	explicit       netip.Addr
	rawNextHop     []byte // Non-nil replaces the intermediate raw policy field, including an empty one.
}

// rfc9252EffectiveNextHopCase keeps the expected wire bytes independent of the
// production next-hop readers and Prefix-SID removal handler.
func rfc9252EffectiveNextHopCase(t *testing.T, rail, carrier string, tc rfc9252NextHopCase) {
	t.Helper()
	f := rfc2545ReceiveFixture(t, false, false, true)
	f.source.settings.AcceptSRv6PrefixSID = true
	f.source.session.settings.AcceptSRv6PrefixSID = true
	f.destination.settings.PropagateSRv6PrefixSID = true
	f.destination.settings.NextHopMode = tc.mode
	f.destination.settings.LocalAddress = netip.MustParseAddr("2001:db8:1::1")
	conn := &rfc4659PeeringConn{
		recordingConn: f.conn,
		local:         f.destination.settings.LocalAddress,
		remote:        f.destination.settings.Address,
	}
	f.destination.session.conn = conn
	f.destination.session.bufWriter = bufio.NewWriterSize(conn, 4096)
	f.destination.session.transport.Store(connectedTransport(conn))
	original := netip.MustParseAddr("2001:db8:1::9")
	replacement := netip.MustParseAddr("2001:db8:1::10")
	if tc.mode == NextHopSelf {
		replacement = f.destination.settings.LocalAddress
	}
	fam := family.IPv6Unicast
	rawNLRI := mustHex(t, "4020010db800070000")
	code := attribute.AttrMPReachNLRI
	if carrier == "legacy" || carrier == "mapped" {
		original = netip.MustParseAddr("192.0.2.9")
		replacement = netip.MustParseAddr("192.0.2.10")
	}
	if carrier == "legacy" {
		// The captured receive scope is 198.18.231.254/24, with source
		// 198.18.231.1. Both third parties share that real source subnet and
		// neither is a session endpoint.
		original = netip.MustParseAddr("198.18.231.9")
		replacement = netip.MustParseAddr("198.18.231.10")
		fam = family.IPv4Unicast
		rawNLRI = mustHex(t, "18c63364")
		code = attribute.AttrNextHop
	}
	if carrier == "vpn6" {
		fam = family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
		// RFC 9252 Section 5.2: zero transposition uses Implicit NULL,
		// label 3 plus Bottom-of-Stack 1, encoded as 00 00 31.
		rawNLRI = mustHex(t, "980000310000ffff0001000020010db800070000")
		for _, peer := range []*Peer{f.source, f.destination} {
			caps := []capability.Capability{
				&capability.ASN4{ASN: peer.settings.LocalAS},
				&capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI},
			}
			neg := capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS})
			peer.session.negotiated = neg
			peer.negotiated.Store(&NegotiatedCapabilities{ASN4: neg.ASN4, families: map[family.Family]bool{fam: true}})
		}
	}
	field := func(addr netip.Addr) []byte {
		if carrier == "mapped" {
			mapped := addr.As16()
			return mapped[:]
		}
		if carrier == "vpn6" {
			return slices.Concat(make([]byte, 8), addr.AsSlice())
		}
		return addr.AsSlice()
	}
	f.destination.settings.NextHopAddress = original
	if tc.explicitChange {
		f.destination.settings.NextHopAddress = replacement
	}
	if tc.explicit.IsValid() {
		f.destination.settings.NextHopAddress = tc.explicit
	}
	// RFC 8669 Sections 3, 3.1 and 3.2: only Label-Index transmit fields
	// normalize; unknown top-level TLVs and Originator SRGB stay identical.
	label := bytes.Clone(rfc8669LabelIndexTLV)
	label[3], label[4], label[5] = 0xa1, 0xb2, 0xc3
	l3, l2 := rfc9252ReservedServiceTLV(5), rfc9252ReservedServiceTLV(6)
	// SID Information Behavior at bytes 25..26: End.DT46 and End.DT2U.
	l3[26], l2[26] = 0x14, 0x17
	services := slices.Concat(l3, l2)
	sid := slices.Concat(label, services, rfc8669UnknownTLV, rfc8669SRGBTLV)
	wantSID := slices.Concat(rfc8669LabelIndexTLV, services, rfc8669UnknownTLV, rfc8669SRGBTLV)
	if tc.changed {
		wantSID = slices.Concat(rfc8669LabelIndexTLV, rfc8669UnknownTLV, rfc8669SRGBTLV)
	}
	if tc.serviceOnly {
		sid = services
		wantSID = nil
	}
	bodyFor := func(hop []byte) []byte {
		attrs := mustHex(t, "4001010040020602010000fde9")
		var legacy []byte
		if carrier == "legacy" {
			attrs = append(attrs, 0x40, byte(code), byte(len(hop)))
			attrs = append(attrs, hop...)
			legacy = rawNLRI
		} else {
			reach := slices.Concat([]byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), byte(len(hop))}, hop, []byte{0}, rawNLRI)
			attrs = append(attrs, 0x80, byte(code), byte(len(reach)))
			attrs = append(attrs, reach...)
		}
		attrs = append(attrs, 0xc0, byte(attribute.AttrPrefixSID), byte(len(sid)))
		attrs = append(attrs, sid...)
		return buildUpdatePayload(attrs, legacy)
	}
	policyCalls := 0
	dispatch := rail
	if tc.policy {
		policyHop := field(original)
		if tc.policyChanged {
			policyHop = field(replacement)
		}
		if tc.rawPolicy {
			if tc.rawNextHop != nil {
				policyHop = tc.rawNextHop
			}
			override := bodyFor(policyHop)
			f.r.api = &pluginserver.Server{}
			f.r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				policyCalls++
				return PolicyResponse{Action: PolicyModify, Raw: override}
			}
			f.destination.settings.ExportFilters = []filterapi.FilterRef{{Name: "srv6:effective-next-hop"}}
			f.r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
			if rail == "rs" {
				dispatch = "rs-export"
			}
		} else {
			rewrite := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
				policyCalls++
				mods.Op(uint8(code), filterapi.AttrModSet, policyHop)
				return true
			}
			f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(rewrite)
			f.r.egressFilters = []filterapi.EgressFilterFunc{rewrite}
		}
	}
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	receivedHop := field(original)
	if carrier == "pair-trim" {
		receivedHop = append(receivedHop, netip.MustParseAddr("fe80::9").AsSlice()...)
	}
	// RFC 9252 Section 2: both identity comparisons are about the actual
	// received and emitted address, after policy and configured overrides.
	updates := rfc2545ReceiveForward(t, f, dispatch, bodyFor(receivedHop))
	wantCalls := 0
	if tc.policy {
		wantCalls = 1
	}
	if policyCalls != wantCalls {
		t.Errorf("policy calls=%d, want %d", policyCalls, wantCalls)
	}
	if len(updates) != 1 {
		t.Fatalf("recipient UPDATEs=%d, want one", len(updates))
	}
	update := updates[0]
	wantHop := field(original)
	if tc.changed {
		wantHop = field(replacement)
	}
	_, _, gotHop, found := attribute.AttrFind(update.PathAttributes, code)
	wantReach := wantHop
	if carrier != "legacy" {
		wantReach = slices.Concat([]byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), byte(len(wantHop))}, wantHop, []byte{0}, rawNLRI)
	} else if !bytes.Equal(update.NLRI, rawNLRI) {
		t.Errorf("legacy NLRI=%x, want %x", update.NLRI, rawNLRI)
	}
	if !found || !bytes.Equal(gotHop, wantReach) {
		t.Errorf("recipient next-hop/reachability=%x present=%v, want %x", gotHop, found, wantReach)
	}
	if len(update.WithdrawnRoutes) != 0 {
		t.Errorf("unexpected withdrawal=%x", update.WithdrawnRoutes)
	}
	if _, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI); found {
		t.Errorf("unexpected MP withdrawal=%x", value)
	}
	_, _, gotSID, found := attribute.AttrFind(update.PathAttributes, attribute.AttrPrefixSID)
	if found != (len(wantSID) != 0) || !bytes.Equal(gotSID, wantSID) {
		t.Errorf("recipient Prefix-SID=%x present=%v, want %x (effective address changed=%v)", gotSID, found, wantSID, tc.changed)
	}
}

// rfc9252MixedNextHopCase makes the applicable MP carrier compete with a
// legacy sibling. RFC 9252 Section 5.4: "The MP_REACH_NLRI over SRv6 core is
// encoded according to [RFC2545]." Legacy IPv4 is not the Section 5.3 MP
// carrier; its next-hop edit must not decide the MP service's propagation.
// RFC 8669 Section 3 still preserves its unrelated unknown top-level TLVs.
func rfc9252MixedNextHopCase(t *testing.T, rail, change string, rawPolicy bool, rawNextHop []byte) {
	t.Helper()
	f := rfc2545ReceiveFixture(t, false, false, true)
	f.source.settings.AcceptSRv6PrefixSID = true
	f.source.session.settings.AcceptSRv6PrefixSID = true
	f.destination.settings.PropagateSRv6PrefixSID = true
	legacy := mustHex(t, "18c63364")
	native := mustHex(t, "4020010db800070000")
	legacyHop := netip.MustParseAddr("198.18.231.9").AsSlice()
	mpHop := netip.MustParseAddr("2001:db8:1::9").AsSlice()
	service := rfc9252ReservedServiceTLV(5)
	// SID Information Behavior occupies bytes 25..26: End.DT46 supports
	// both IP versions, unlike the helper's original End.DT4.
	service[26] = 0x14
	sid := slices.Concat(rfc8669LabelIndexTLV, service, rfc8669UnknownTLV, rfc8669SRGBTLV)
	bodyFor := func(legacyNextHop, mpNextHop []byte) []byte {
		attrs := mixedAttrs(mixedReach(1, mpNextHop, native))
		attrs = append(attrs, 0x40, byte(attribute.AttrNextHop), 4)
		attrs = append(attrs, legacyNextHop...)
		attrs = append(attrs, 0xc0, byte(attribute.AttrPrefixSID), byte(len(sid)))
		attrs = append(attrs, sid...)
		return buildUpdatePayload(attrs, legacy)
	}
	received := bodyFor(legacyHop, mpHop)
	if change == "legacy-only" || change == "both" {
		legacyHop = netip.MustParseAddr("198.18.231.10").AsSlice()
	}
	mpChanged := change == "mp-only" || change == "both"
	if mpChanged {
		mpHop = netip.MustParseAddr("2001:db8:1::10").AsSlice()
	}
	calls := 0
	dispatch := rail
	if rawPolicy {
		policyHop := mpHop
		if rawNextHop != nil {
			// The raw policy leaves an existing MP carrier with no usable
			// intermediate address. Configured rewriting repairs it before
			// admission; the repaired MP entity must still govern its services.
			policyHop = rawNextHop
			f.destination.settings.NextHopMode = NextHopExplicit
			f.destination.settings.NextHopAddress = netip.AddrFrom16([16]byte(mpHop))
		}
		override := bodyFor(legacyHop, policyHop)
		f.r.api = &pluginserver.Server{}
		f.r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
			calls++
			return PolicyResponse{Action: PolicyModify, Raw: override}
		}
		f.destination.settings.ExportFilters = []filterapi.FilterRef{{Name: "srv6:mixed-next-hop"}}
		f.r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
		if rail == "rs" {
			dispatch = "rs-export"
		}
	} else {
		rewrite := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
			calls++
			mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, legacyHop)
			mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, mpHop)
			return true
		}
		f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(rewrite)
		f.r.egressFilters = []filterapi.EgressFilterFunc{rewrite}
	}
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	// RFC 9252 Section 2 and RFC 8669 Section 3: one policy decision,
	// actual Session output, one native and one legacy announcement.
	updates := rfc2545ReceiveForward(t, f, dispatch, received)
	if calls != 1 {
		t.Errorf("policy calls=%d, want one for the mixed received UPDATE", calls)
	}
	if len(updates) != 2 {
		t.Fatalf("recipient UPDATEs=%d, want one native and one legacy", len(updates))
	}
	nativeCount, legacyCount := 0, 0
	for _, update := range updates {
		_, _, gotSID, found := attribute.AttrFind(update.PathAttributes, attribute.AttrPrefixSID)
		if !found {
			t.Error("Prefix-SID lost unrelated TLVs")
		}
		if len(update.NLRI) == 0 {
			nativeCount++
			rfc2545AssertNative(t, update, mpHop, native, true)
			wantSID := sid
			if mpChanged {
				wantSID = slices.Concat(rfc8669LabelIndexTLV, rfc8669UnknownTLV, rfc8669SRGBTLV)
			}
			if !bytes.Equal(gotSID, wantSID) {
				t.Errorf("MP Service TLVs followed the wrong next-hop field: got %x, want %x", gotSID, wantSID)
			}
			continue
		}
		legacyCount++
		_, _, gotHop, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
		if !found || !bytes.Equal(gotHop, legacyHop) || !bytes.Equal(update.NLRI, legacy) || len(update.WithdrawnRoutes) != 0 {
			t.Errorf("legacy sibling changed: next-hop=%x NLRI=%x withdrawn=%x", gotHop, update.NLRI, update.WithdrawnRoutes)
		}
		for _, tlv := range [][]byte{rfc8669LabelIndexTLV, rfc8669UnknownTLV, rfc8669SRGBTLV} {
			if !bytes.Contains(gotSID, tlv) {
				t.Errorf("legacy sibling lost unrelated TLV %x from %x", tlv, gotSID)
			}
		}
	}
	if nativeCount != 1 || legacyCount != 1 {
		t.Errorf("native count=%d legacy count=%d, want one each", nativeCount, legacyCount)
	}
}
