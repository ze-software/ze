// Design: docs/architecture/route-selection.md -- AIGP session policy.
// Related: forward_aigp.go -- source-link lookup and next-hop-self accumulation.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestRFC7311SourceLinkCostReachesRecipient proves source-cost provenance through
// the general API and the route server's native-entry/cached-forward sequence.
// No IGP route supplies a distance. Source and destination settings deliberately
// differ, and the recipient frame must contain the source cost exactly once.
// RFC 7311 Section 3.4.3: "Then, when R1 changes the next hop of a route from R2
// to R1, the AIGP TLV value MUST be increased by a non-zero amount."
// RFC 7311 Section 3.4.3: "If R1 does not change the next hop of the route, then
// R1 MUST NOT change the AIGP attribute value of the route."
// RFC requirement: RFC7311-3.4.3-6 positive -- general and RS cached forwarding read the source peer's configured direct-link cost and emit the exact accumulated metric at the recipient.
// RFC requirement: RFC7311-3.4.3-6 negative -- a different destination-link cost cannot replace the source cost, and an unchanged next hop does not accumulate either link's cost.
// MUTATION: Substitute destination Settings().AIGPLinkMetric for the source-link
// lookup in forwardUpdateSection; the ordinary sums must fail on both entries.
func TestRFC7311SourceLinkCostReachesRecipient(t *testing.T) {
	for _, rail := range []string{"general", "rs-cached"} {
		for _, tc := range []struct {
			name       string
			received   uint64
			sourceCost uint64
			want       uint64
			self       bool
		}{
			{"source-seven", 100, 7, 107, true},
			{"source-nineteen", 100, 19, 119, true},
			{"unchanged", 100, 7, 100, false},
			{"unknown-cost-unchanged", 100, 0, 100, false},
			{"maximum-exact", ^uint64(0) - 7, 7, ^uint64(0), true},
			{"maximum-overflow", ^uint64(0) - 5, 7, ^uint64(0), true},
		} {
			t.Run(rail+"/"+tc.name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				f.source.settings.AIGPLinkMetric = tc.sourceCost
				f.source.settings.RSClient = rail == "rs-cached"
				f.source.refreshForwardFacts()
				f.destination.settings.AIGPLinkMetric = 43
				f.destination.settings.RSClient = rail == "rs-cached"
				f.destination.settings.NextHopMode = NextHopUnchanged
				if tc.self {
					f.destination.settings.NextHopMode = NextHopSelf
				}
				f.destination.refreshForwardFacts()
				if distance := igpcost.Lookup(f.source.Settings().Address); distance.Resolved {
					t.Fatalf("direct-link fixture unexpectedly has an interior route: %+v", distance)
				}
				body := f.body(t, tc.received)
				original := bytes.Clone(body)
				id := f.receive(t, body)
				if rail == "rs-cached" {
					update, ok := f.r.recentUpdates.Get(id)
					if !ok {
						t.Fatal("received UPDATE missing from forwarding cache")
					}
					// AIGP must join the source FIFO rather than bypass it on
					// the direct-write rail. This is the live RS fallback, not
					// a call below reactorForwardRS's cached-source guard.
					skipped, delivered := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
					if len(skipped) != 0 || delivered != 0 {
						t.Fatalf("AIGP bypassed cached forwarding: skipped=%v delivered=%d", skipped, delivered)
					}
					if !f.source.forwardCached.Load() {
						t.Fatal("AIGP source did not retain cached-forward ordering")
					}
					if len(f.conn.written()) != 0 {
						t.Fatal("RS entry wrote before its cached source worker")
					}
					api := &reactorAPIAdapter{r: f.r}
					// ForwardCached's reactor endpoint applies RFC 7311 Section 3.4.3.
					if err := api.ForwardUpdatesDirect([]uint64{id}, []netip.AddrPort{f.destination.Settings().PeerKey()}, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder")); err != nil {
						t.Fatal(err)
					}
				} else {
					// ForwardUpdate applies RFC 7311 Section 3.4.3 after peer policy.
					f.forward(t, id)
				}
				forwardSocketBarrier(t, f.r)
				bodies := aigpSocketBodies(t, f.conn)
				if len(bodies) != 1 {
					t.Fatalf("recipient UPDATE count=%d, want 1", len(bodies))
				}
				sections, err := wire.ParseUpdateSections(bodies[0])
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(sections.NLRI(bodies[0]), []byte{24, 10, 20, 0}) {
					t.Fatalf("recipient NLRI=%x", sections.NLRI(bodies[0]))
				}
				if len(sections.Withdrawn(bodies[0])) != 0 {
					t.Fatal("configured direct-link announcement became a withdrawal")
				}
				attrs := sections.Attrs(bodies[0])
				_, flags, metric, present := attribute.AttrFind(attrs, attribute.AttrAIGP)
				wantMetric := [11]byte{1, 0, 11}
				binary.BigEndian.PutUint64(wantMetric[3:], tc.want)
				if !present || flags != 0x80 || !bytes.Equal(metric, wantMetric[:]) {
					t.Fatalf("recipient AIGP=%x flags=%x present=%v, want %x flags=80", metric, flags, present, wantMetric)
				}
				wantNextHop := f.source.Settings().Address
				if tc.self {
					wantNextHop = f.destination.Settings().LocalAddress
				}
				_, _, nextHop, present := attribute.AttrFind(attrs, attribute.AttrNextHop)
				if !present || !bytes.Equal(nextHop, wantNextHop.AsSlice()) {
					t.Fatalf("recipient NEXT_HOP=%x present=%v, want %s", nextHop, present, wantNextHop)
				}
				if !bytes.Equal(body, original) {
					t.Fatal("forwarding changed the received source bytes")
				}
			})
		}
	}
}

// TestRFC7311UnknownCostWithdrawsUntilMetricRecovers checks the recipient's
// route, not only attribute absence: an unavailable increment cannot turn an
// AIGP route into an attribute-free next-hop-self announcement. A later distance
// update must recover the retained route without a new received UPDATE.
// RFC 7311 Section 3.4.3: "However, A MUST be increased by a non-zero amount."
// RFC requirement: RFC7311-3.4.3-6 negative -- missing direct-link cost withdraws the route instead of announcing next-hop-self without its required AIGP increment.
// RFC requirement: RFC7311-3.4.3-6 positive -- a known nonzero distance restores the original received route and adds that distance exactly once.
func TestRFC7311UnknownCostWithdrawsUntilMetricRecovers(t *testing.T) {
	for _, rail := range []string{"general", "rs-cached"} {
		for _, unavailable := range []string{"unknown", "zero"} {
			for _, prior := range []bool{false, true} {
				name := rail + "/" + unavailable + "/initially-withheld"
				if prior {
					name = rail + "/" + unavailable + "/previously-advertised"
				}
				t.Run(name, func(t *testing.T) {
					f := newAIGPReplayFixture(t, nil)
					f.source.settings.RSClient = rail == "rs-cached"
					f.source.refreshForwardFacts()
					f.destination.settings.RSClient = rail == "rs-cached"
					f.destination.settings.AIGPLinkMetric = 43
					f.destination.refreshForwardFacts()
					if unavailable == "zero" {
						f.metric(0)
					}
					if prior {
						f.metric(7)
					}
					id := f.receive(t, f.body(t, 100))
					if rail == "rs-cached" {
						update, ok := f.r.recentUpdates.Get(id)
						if !ok {
							t.Fatal("received UPDATE missing from forwarding cache")
						}
						skipped, delivered := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
						if len(skipped) != 0 || delivered != 0 {
							t.Fatalf("AIGP bypassed cached forwarding: skipped=%v delivered=%d", skipped, delivered)
						}
						api := &reactorAPIAdapter{r: f.r}
						// RFC 7311 Section 3.4.3, through the RS cached endpoint.
						if err := api.ForwardUpdatesDirect([]uint64{id}, []netip.AddrPort{f.destination.Settings().PeerKey()}, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder")); err != nil {
							t.Fatal(err)
						}
					} else {
						// RFC 7311 Section 3.4.3, through the general endpoint.
						f.forward(t, id)
					}
					forwardSocketBarrier(t, f.r)
					wantFrames := 1
					if prior {
						bodies := aigpSocketBodies(t, f.conn)
						if len(bodies) != 1 {
							t.Fatalf("initial UPDATE count=%d, want 1", len(bodies))
						}
						metric, present := aigpReceivedMetric(t, bodies[0])
						if !present || metric != 107 {
							t.Fatalf("initial metric=%d present=%v, want 107", metric, present)
						}
						if unavailable == "zero" {
							f.metric(0)
						} else {
							locrib.Default().Remove(family.IPv4Unicast, netip.PrefixFrom(f.source.Settings().Address, 32), redistevents.RegisterProtocol("aigp-readvertise-test"), 0)
						}
						f.r.readvertiseAIGP()
						forwardSocketBarrier(t, f.r)
						wantFrames++
					}
					bodies := aigpSocketBodies(t, f.conn)
					if len(bodies) != wantFrames {
						t.Fatalf("withheld UPDATE count=%d, want %d", len(bodies), wantFrames)
					}
					withdraw := bodies[len(bodies)-1]
					sections, err := wire.ParseUpdateSections(withdraw)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(sections.Withdrawn(withdraw), []byte{24, 10, 20, 0}) {
						t.Fatalf("unavailable cost advertised instead of withdrawing: body=%x", withdraw)
					}
					if len(sections.NLRI(withdraw)) != 0 || len(sections.Attrs(withdraw)) != 0 {
						t.Fatalf("unavailable-cost withdrawal carries an announcement or attributes: %x", withdraw)
					}
					f.metric(11)
					f.r.readvertiseAIGP()
					forwardSocketBarrier(t, f.r)
					bodies = aigpSocketBodies(t, f.conn)
					if len(bodies) != wantFrames+1 {
						t.Fatalf("recovery UPDATE count=%d, want %d; withdrawal lost its retained recipient", len(bodies), wantFrames+1)
					}
					recovered := bodies[len(bodies)-1]
					sections, err = wire.ParseUpdateSections(recovered)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(sections.NLRI(recovered), []byte{24, 10, 20, 0}) || len(sections.Withdrawn(recovered)) != 0 {
						t.Fatalf("metric recovery did not restore the exact route: %x", recovered)
					}
					metric, present := aigpReceivedMetric(t, recovered)
					if !present || metric != 111 {
						t.Fatalf("recovered metric=%d present=%v, want received 100 plus distance 11", metric, present)
					}
				})
			}
		}
	}
}

// A cost-withheld candidate is not permission to recover a source withdrawal or
// an export-policy refusal. Compare the complete socket bytes after the metric
// recovers and the export filter is removed.
func TestAIGPCostRecoveryDoesNotReviveWithdrawnRoutes(t *testing.T) {
	for _, boundary := range []string{"source-withdrawal", "export-withdrawal"} {
		t.Run(boundary, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			if boundary == "export-withdrawal" {
				f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
					mods.SetWithdraw()
					return true
				})
			}
			f.forward(t, f.receive(t, f.body(t, 100)))
			forwardSocketBarrier(t, f.r)
			const wantFrames = 1
			if boundary == "source-withdrawal" {
				withheld := f.conn.written()
				f.forward(t, f.receive(t, makeUpdateBody([]byte{24, 10, 20, 0}, nil, nil)))
				forwardSocketBarrier(t, f.r)
				if !bytes.Equal(withheld, f.conn.written()) {
					t.Fatal("an original withdrawal without an advertised owner emitted bytes")
				}
			}
			bodies := aigpSocketBodies(t, f.conn)
			if len(bodies) != wantFrames {
				t.Fatalf("withdrawal frames=%d, want %d", len(bodies), wantFrames)
			}
			for _, body := range bodies {
				if !bytes.Equal(body, []byte{0, 4, 24, 10, 20, 0, 0, 0}) {
					t.Fatalf("expected exact route withdrawal, got %x", body)
				}
			}
			before := f.conn.written()
			f.r.orderedEgressSteps = nil
			f.metric(11)
			f.r.readvertiseAIGP()
			forwardSocketBarrier(t, f.r)
			if !bytes.Equal(before, f.conn.written()) {
				t.Fatal("metric recovery revived a withdrawn or policy-refused route")
			}
		})
	}
}
