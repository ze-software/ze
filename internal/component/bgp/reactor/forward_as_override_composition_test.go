// Design: docs/architecture/bgp/fanout-dedup.md -- AS override composes with egress edits.
// Related: rfc7311_aigp_readvertise_test.go -- actual Session writer and recording connection.
package reactor

import (
	"fmt"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestForwardASOverrideComposition checks the bytes emitted by the real Session
// writer on both rails. Override must preserve the ordinary eBGP prepend and the
// effective policy path, while an RS client remains free of the protocol prepend.
// The absent-target case and immutable receive body prevent unconditional edits.
func TestForwardASOverrideComposition(t *testing.T) {
	for _, fast := range []bool{false, true} {
		for _, asn4 := range []bool{false, true} {
			for _, tc := range []struct {
				name   string
				rs     bool
				target uint32
				prepend bool
				generated bool
				policy []uint32
				want   []uint32
			}{
				{name: "ordinary", target: 65002, want: []uint32{65000, 65001, 65000}},
				{name: "rs-client", rs: true, target: 65002, want: []uint32{65001, 65000}},
				{name: "target-absent", target: 65003, want: []uint32{65000, 65001, 65002}},
				{name: "policy-set", target: 65002, policy: []uint32{64496, 65002}, want: []uint32{65000, 64496, 65000}},
				{name: "policy-prepend", target: 65002, policy: []uint32{64496, 65002}, prepend: true, want: []uint32{65000, 64496, 65000, 65001, 65000}},
				{name: "policy-generator", target: 65002, policy: []uint32{64496, 65002}, generated: true, want: []uint32{65000, 64496, 65000}},
			} {
				t.Run(fmt.Sprintf("fast=%v/asn4=%v/%s", fast, asn4, tc.name), func(t *testing.T) {
					f := newAIGPReplayFixture(t, nil)
					f.source.settings.RSClient = true
					f.source.refreshForwardFacts()
					peer := f.destination
					peer.settings.LocalAS = 65000
					peer.settings.PeerAS = tc.target
					peer.settings.RSClient = tc.rs
					peer.settings.ASOverride = true
					peer.settings.NextHopMode = NextHopUnchanged
					ctx := bgpctx.EncodingContextForASN4(asn4)
					ctxID, err := bgpctx.Registry.Register(ctx)
					if err != nil {
						t.Fatal(err)
					}
					peer.sendCtx.Store(ctx)
					peer.sendCtxID = ctxID
					peer.session.sendCtxID = ctxID
					peer.negotiated.Load().ASN4 = asn4
					peer.session.negotiated.ASN4 = asn4
					peer.refreshForwardFacts()
					if tc.policy != nil {
						value := asSequence4(tc.policy...)
						edit := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
							switch {
							case tc.prepend:
								mods.Op(byte(attribute.AttrASPath), filterapi.AttrModPrepend, value)
							case tc.generated:
								mods.OpGen(byte(attribute.AttrASPath), overridePolicyGenerator(value))
							default:
								mods.Op(byte(attribute.AttrASPath), filterapi.AttrModSet, value)
							}
							return true
						}
						f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(edit)
						f.r.egressFilters = []filterapi.EgressFilterFunc{edit}
					}
					attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
					attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), asSequence4(65001, 65002))...)
					attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
					body := buildUpdatePayload(attrs, a2InlinePrefix)
					original := slices.Clone(body)
					id := f.receive(t, body)
					if fast {
						update, ok := f.r.recentUpdates.Get(id)
						if !ok {
							t.Fatal("received UPDATE missing")
						}
						skipped, sent := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
						if len(skipped) != 0 || sent != 1 {
							t.Fatalf("forwarded=%d skipped=%v", sent, skipped)
						}
						f.source.session.flushFwdDirty()
					} else {
						f.forward(t, id)
					}
					forwardSocketBarrier(t, f.r)
					bodies := aigpSocketBodies(t, f.conn)
					if len(bodies) != 1 {
						t.Fatalf("recipient UPDATEs=%d, want one", len(bodies))
					}
					update, err := message.UnpackUpdate(bodies[0])
					if err != nil {
						t.Fatal(err)
					}
					if !slices.Equal(update.NLRI, a2InlinePrefix) {
						t.Fatalf("NLRI=%x, want %x", update.NLRI, a2InlinePrefix)
					}
					_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
					if !found {
						t.Fatal("recipient AS_PATH missing")
					}
					path, err := attribute.ParseASPath(value, asn4)
					if err != nil {
						t.Fatal(err)
					}
					if got := flatASNs(path); !slices.Equal(got, tc.want) {
						t.Errorf("recipient AS_PATH=%v, want %v", got, tc.want)
					}
					if !slices.Equal(body, original) {
						t.Error("forwarding changed shared received bytes")
					}
				})
			}
		}
	}
}

type overridePolicyGenerator []byte

func (g overridePolicyGenerator) GenLen() int { return len(g) }

func (g overridePolicyGenerator) GenWrite(buf []byte, off int) int {
	return copy(buf[off:], g)
}

// TestASOverrideComposesLegacyAS4Projection preserves the real ASN before
// override, including a policy replacement of AS4_PATH, then checks both wire
// attributes rather than accepting AS_TRANS as the semantic result.
func TestASOverrideComposesLegacyAS4Projection(t *testing.T) {
	for _, asn4 := range []bool{false, true} {
		for _, policy := range []bool{false, true} {
			t.Run(fmt.Sprintf("asn4=%v/policy=%v", asn4, policy), func(t *testing.T) {
				attrs := makeAttr(0x40, byte(attribute.AttrASPath), []byte{2, 3, 0xfd, 0xe9, 0x5b, 0xa0, 0xfd, 0xea})
				attrs = append(attrs, makeAttr(0xc0, byte(attribute.AttrAS4Path), asSequence4(65001, 70000, 65002))...)
				body := buildUpdatePayload(attrs, a2InlinePrefix)
				original := slices.Clone(body)
				target := uint32(70000)
				var mods filterapi.ModAccumulator
				if policy {
					target = 90000
					mods.OpGen(byte(attribute.AttrAS4Path), overridePolicyGenerator(asSequence4(65001, target, 65002)))
				}
				var edit wireu.ASPathEdit
				changed, err := edit.Record(&mods, body, wireu.ASPathIntent{
					Prepend: []uint32{65000}, DstASN4: asn4,
					OverridePeerAS: target, OverrideLocalAS: 80000,
				})
				if err != nil || !changed {
					t.Fatalf("composition changed=%v error=%v", changed, err)
				}
				result, _, failure := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
				if failure.failed() || result == nil {
					t.Fatalf("composition failed: %v", failure)
				}
				path, err := attribute.ParseASPath(payloadASPathValue(t, result), asn4)
				if err != nil {
					t.Fatal(err)
				}
				companion, found := attrValueFromPayload(t, result, attribute.AttrAS4Path)
				if asn4 {
					if found {
						t.Fatal("AS4_PATH sent to a NEW speaker")
					}
				} else {
					if !found {
						t.Fatal("AS4_PATH missing for non-mappable overridden ASN")
					}
					if got := flatASNs(path); !slices.Equal(got, []uint32{65000, 65001, 23456, 65002}) {
						t.Fatalf("two-octet AS_PATH=%v", got)
					}
					as4Path, err := attribute.ParseAS4Path(companion)
					if err != nil {
						t.Fatal(err)
					}
					path = attribute.MergeAS4Path(path, as4Path)
				}
				if got := flatASNs(path); !slices.Equal(got, []uint32{65000, 65001, 80000, 65002}) {
					t.Errorf("reconstructed AS_PATH=%v", got)
				}
				if !slices.Equal(body, original) {
					t.Error("legacy composition changed shared received bytes")
				}
			})
		}
	}
}
