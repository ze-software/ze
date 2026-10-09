// Design: docs/architecture/bgp/structural-forwarding.md -- effective next-hop scope.
package reactor

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC2545ForwardPolicyOwnGlobalJointSubnet reads actual recipient UPDATEs
// after policy replaces a received third-party pair with this speaker's global.
// Both forwarding rails, including raw-export fallback, must preserve the
// independent legacy sibling and apply policy only once to the IPv6 route.
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to."
// RFC requirement: RFC2545-3-3 positive -- a policy-written speaker-owned global receives its available own link-local on the common subnet, through both forwarding rails and raw-export fallback.
// RFC requirement: RFC2545-3-4 negative -- a common-link recipient keeps the required own global-plus-link-local pair rather than receiving global-only output; native NLRI and the legacy sibling remain unchanged.
// MUTATION: omit post-policy own-link-local inclusion, reuse the received entity's
// link-local, append on every subnet, or run export policy again in the writer.
func TestRFC2545ForwardPolicyOwnGlobalJointSubnet(t *testing.T) {
	ownGlobal := netip.MustParseAddr("2001:db8:1::1")
	ownLL := netip.MustParseAddr("fe80::1")
	received := append(netip.MustParseAddr("2001:db8:1::9").AsSlice(), netip.MustParseAddr("fe80::9").AsSlice()...)
	for _, rail := range []string{"cached", "rs"} {
		for _, mode := range []struct {
			name string
			mode uint8
		}{{"auto", NextHopAuto}, {"unchanged", NextHopUnchanged}} {
			for _, onLink := range []bool{true, false} {
				subnet := "common-link"
				if !onLink {
					subnet = "recipient-off-link"
				}
				for _, rawPolicy := range []bool{false, true} {
					policy := "operation"
					if rawPolicy {
						policy = "raw"
					}
					t.Run(rail+"/"+mode.name+"/"+subnet+"/"+policy, func(t *testing.T) {
						f := rfc2545ReceiveFixture(t, false, onLink, true)
						f.destination.settings.NextHopMode = mode.mode
						f.destination.settings.LocalAddress = ownGlobal
						f.destination.settings.LinkLocal = ownLL
						f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
						raw := mustHex(t, "4020010db800070000")
						legacy := mustHex(t, "180a0900")
						bodyFor := func(hop []byte) []byte {
							attrs := mixedAttrs(mixedReach(1, hop, raw))
							attrs = append(attrs, 0x40, 3, 4)
							attrs = append(attrs, f.source.Settings().Address.AsSlice()...)
							return buildUpdatePayload(attrs, legacy)
						}
						policyCalls := 0
						dispatch := rail
						if rawPolicy {
							override := bodyFor(ownGlobal.AsSlice())
							f.r.api = &pluginserver.Server{}
							f.r.policyFilterSeam = func(_, _, _, _ string, _ uint32, text string) PolicyResponse {
								if strings.Contains(text, "ipv6/unicast") {
									policyCalls++
									return PolicyResponse{Action: PolicyModify, Raw: override}
								}
								return PolicyResponse{Action: PolicyAccept}
							}
							f.destination.settings.ExportFilters = []filterapi.FilterRef{{Name: "ipv6:own-global"}}
							f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
							f.r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
							if rail == "rs" {
								dispatch = "rs-export"
							}
						} else {
							rewrite := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
								policyCalls++
								mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, ownGlobal.AsSlice())
								return true
							}
							f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(rewrite)
							f.r.egressFilters = []filterapi.EgressFilterFunc{rewrite}
						}
						updates := rfc2545ReceiveForward(t, f, dispatch, bodyFor(received))
						if policyCalls != 1 {
							t.Errorf("IPv6 policy calls=%d, want exactly one", policyCalls)
						}
						if len(updates) != 2 {
							t.Fatalf("recipient UPDATEs=%d, want the native and legacy sections", len(updates))
						}
						wantHop := ownGlobal.AsSlice()
						if onLink {
							wantHop = append(wantHop, ownLL.AsSlice()...)
						}
						nativeCount, legacyCount := 0, 0
						for _, update := range updates {
							if len(update.NLRI) == 0 {
								nativeCount++
								// RFC 2545 Section 3: inspect the complete emitted field.
								rfc2545AssertNative(t, update, wantHop, raw, true)
								continue
							}
							legacyCount++
							_, _, hop, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
							if !bytes.Equal(update.NLRI, legacy) || len(update.WithdrawnRoutes) != 0 || !found || !bytes.Equal(hop, f.source.Settings().Address.AsSlice()) {
								t.Errorf("legacy sibling changed: NLRI=%x withdrawn=%x next-hop=%x present=%v", update.NLRI, update.WithdrawnRoutes, hop, found)
							}
							for _, code := range []attribute.AttributeCode{attribute.AttrMPReachNLRI, attribute.AttrMPUnreachNLRI} {
								if _, _, _, found := attribute.AttrFind(update.PathAttributes, code); found {
									t.Error("native reachability leaked into the legacy sibling")
								}
							}
						}
						if nativeCount != 1 || legacyCount != 1 {
							t.Errorf("native sections=%d legacy sections=%d, want one of each", nativeCount, legacyCount)
						}
					})
				}
			}
		}
	}
}
