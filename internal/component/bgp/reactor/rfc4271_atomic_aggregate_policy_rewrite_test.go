// Design: docs/architecture/bgp/structural-forwarding.md -- policy output reaches the final relay writer.
// Related: rfc4271_atomic_aggregate_relay_proof_test.go -- received UPDATE and live relay fixture.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC4271AtomicAggregatePolicyRewriteDoesNotDeaggregate sends a valid
// received /24 through an export member that replaces its NLRI with a /25,
// both retaining and stripping ATOMIC_AGGREGATE in the policy output. The
// prohibition binds the received route even if policy removes the marker.
// Each rail also sends the same rewrite from an unmarked received route,
// proving the member and final writer do real work.
// Safe suppression or preservation of the atomic route is permitted; a
// more-specific announcement is not. Assertions read the final socket bytes.
//
// RFC 4271 Section 5.1.6: "A BGP speaker that receives a route with the ATOMIC_AGGREGATE
// attribute MUST NOT make any NLRI of that route more specific (as
// defined in 9.1.4) when advertising this route to other BGP speakers."
// RFC 4271 Section 9.1.4: "In particular, a route that carries the
// ATOMIC_AGGREGATE attribute MUST NOT be de-aggregated."
// RFC requirement: RFC4271-5.1.6-1 negative — raw export-policy /24-to-/25 rewrites of a received marked legacy IPv4 route do not reach the final socket on ordinary and native route-server rails, whether the answer retains or strips ATOMIC_AGGREGATE; the same rewrite from unmarked input remains advertisable.
func TestRFC4271AtomicAggregatePolicyRewriteDoesNotDeaggregate(t *testing.T) {
	original := []byte{24, 10, 42, 0}
	specific := []byte{25, 10, 42, 0, 0}
	cover := netip.MustParsePrefix("10.42.0.0/24")
	for _, rail := range []struct {
		name string
		rs   bool
	}{{"ordinary", false}, {"native-rs", true}} {
		t.Run(rail.name, func(t *testing.T) {
			for _, tc := range []struct {
				name         string
				atomic       bool
				outputAtomic bool
			}{
				{"unmarked-control", false, false},
				{"atomic-aggregate", true, true},
				{"atomic-aggregate-stripped", true, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					f := newAIGPReplayFixture(t, nil)
					received := rfc4271ProofRelayBody(original, tc.atomic, 10)
					rewritten := rfc4271ProofRelayBody(specific, tc.outputAtomic, 10)
					exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rewritten}
					}, f.destination)
					rfc4271AtomicPolicyRelay(t, f, received, rail.rs)
					transport, ok := f.r.filterTransportSeam.(*answerTransport)
					require.True(t, ok)
					require.Equal(t, 1, transport.calls(), "the export member must receive the announcement")
					bodies := aigpSocketBodies(t, f.conn)
					if !tc.atomic {
						require.Len(t, bodies, 1, "the unmarked control must reach the socket")
						update, err := message.UnpackUpdate(bodies[0])
						require.NoError(t, err)
						require.Equal(t, specific, update.NLRI, "the configured NLRI rewrite must work")
						require.Empty(t, update.WithdrawnRoutes)
						return
					}
					for _, body := range bodies {
						update, err := message.UnpackUpdate(body)
						require.NoError(t, err)
						announced := update.NLRI
						for len(announced) > 0 {
							bits := int(announced[0])
							require.LessOrEqual(t, bits, 32, "valid IPv4 prefix length")
							octets := (bits + 7) / 8
							require.GreaterOrEqual(t, len(announced), 1+octets, "complete IPv4 prefix")
							var address [4]byte
							copy(address[:], announced[1:1+octets])
							prefix := netip.PrefixFrom(netip.AddrFrom4(address), bits).Masked()
							if cover.Contains(prefix.Addr()) {
								require.LessOrEqual(t, prefix.Bits(), cover.Bits(),
									"received ATOMIC_AGGREGATE route %s was deaggregated to %s; UPDATE=%x", cover, prefix, body)
							}
							announced = announced[1+octets:]
						}
					}
				})
			}
		})
	}
}

// rfc4271AtomicPolicyRelay uses the existing live relay fixture while permitting
// a correct ATOMIC_AGGREGATE guard to suppress before native dispatch. Unlike
// rfc4271ProofRelay, it therefore does not require one dispatched announcement.
// The native read-boundary flush and the destination FIFO barrier remain real.
func rfc4271AtomicPolicyRelay(t *testing.T, f *aigpReplayFixture, body []byte, rs bool) {
	t.Helper()
	f.r.rsForwardingEnabled = false
	id := f.receive(t, body)
	if rs {
		update, present := f.r.recentUpdates.Get(id)
		require.True(t, present)
		f.r.rsForwardingEnabled = true
		skipped, _ := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
		f.r.rsForwardingEnabled = false
		require.Empty(t, skipped, "native relay must decide the destination")
		f.source.session.flushFwdDirty()
		require.Empty(t, f.source.session.fwdDirty)
	} else {
		f.forward(t, id)
	}
	f.drain(t)
}
