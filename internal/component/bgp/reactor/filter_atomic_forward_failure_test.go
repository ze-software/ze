// Design: docs/architecture/bgp/structural-forwarding.md -- accepted source retirements survive survivor encoding failure.
// Related: forward_policy_removal.go -- negotiated removal materialization and grouped ownership.
// RFC: rfc/short/rfc4271.md -- Section 5.1.6; RFC 4760 family negotiation bounds emitted withdrawals.
package reactor

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAtomicForwardFailureRetiresSource distinguishes an admitted subset from
// an unencodable surviving route. A failed B must still retire its source's A;
// the same cleanup cannot remove A when another source owns it.
func TestAtomicForwardFailureRetiresSource(t *testing.T) {
	a, b := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	input := rfc4271ProofRelayBody(append(bytes.Clone(a), b...), true, 10)
	for _, rs := range []bool{false, true} {
		for _, foreignOwner := range []bool{false, true} {
			name := "source-owner"
			if foreignOwner {
				name = "foreign-owner"
			}
			t.Run(atomicAdmissionRailName(rs)+"/"+name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				atomicFailureExtendedReceive(t, f)
				f.destination.session.adjOut = &f.destination.adjOut
				foreign, _ := newAnnouncePeer(t, "198.18.233.3")
				foreign.recvCtxID = f.ctxID
				if foreignOwner {
					rfc4271AtomicPolicyRelay(t, f, rfc4271ProofRelayBody(b, true, 10), rs)
					require.NoError(t, ownershipWriterForward(t, f.destination, foreign, rfc4271ProofRelayBody(a, true, 10), false))
				} else {
					rfc4271AtomicPolicyRelay(t, f, input, rs)
				}
				before := len(aigpSocketBodies(t, f.conn))
				base, err := message.UnpackUpdate(rfc4271ProofRelayBody(b, true, 77))
				require.NoError(t, err)
				attrs := append(bytes.Clone(base.PathAttributes), atomicFailureExtendedAttr(0xc0, 230, make([]byte, 5000))...)
				output := makeUpdateBody(nil, attrs, b)
				require.Greater(t, len(output)+message.HeaderLen, message.MaxMsgLen)
				exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
					return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
				}, f.destination)
				id := f.receive(t, input)
				removals := atomicFailureAssertAdmitted(t, f, id, output)
				require.Len(t, removals, 1)
				removal, err := message.UnpackUpdate(removals[0])
				require.NoError(t, err)
				require.Equal(t, a, removal.WithdrawnRoutes, "admitted policy must owe exactly source A before B encoding fails")
				require.Empty(t, removal.NLRI)
				atomicFailureDispatch(t, f, id, rs, true)
				written := aigpSocketBodies(t, f.conn)[before:]
				if foreignOwner {
					require.Empty(t, written, "failed survivor must not remove a foreign route or invent an EOR")
				} else {
					require.Len(t, written, 1, "accepted source retirement must survive failed B encoding")
					update, err := message.UnpackUpdate(written[0])
					require.NoError(t, err)
					require.Equal(t, a, update.WithdrawnRoutes)
					require.Empty(t, update.NLRI)
					require.Empty(t, update.PathAttributes)
				}
				f.destination.settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
				f.destination.refreshForwardFacts()
				if foreignOwner {
					require.NoError(t, ownershipWriterForward(t, f.destination, foreign, makeUpdateBody(a, nil, nil), false))
					last := aigpSocketBodies(t, f.conn)
					update, err := message.UnpackUpdate(last[len(last)-1])
					require.NoError(t, err)
					require.Equal(t, a, update.WithdrawnRoutes, "foreign owner retains its authority after cleanup")
				}
				c := []byte{24, 10, 44, 0}
				rfc4271AtomicPolicyRelay(t, f, rfc4271ProofRelayBody(c, true, 77), rs)
				last := aigpSocketBodies(t, f.conn)
				update, err := message.UnpackUpdate(last[len(last)-1])
				require.NoError(t, err)
				require.Equal(t, c, update.NLRI, "a failed survivor must leave the forwarding rail useful")
			})
		}
	}
}

// TestAtomicForwardRemovalNegotiatedFamilies exercises small and indivisible
// large unsupported siblings. The unsupported MP withdrawal cannot prevent a
// useful negotiated survivor or swallow a legacy removal sharing its body.
func TestAtomicForwardRemovalNegotiatedFamilies(t *testing.T) {
	bgpls := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	a, b := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}
	legacy := []byte{24, 10, 42, 0}
	for _, rs := range []bool{false, true} {
		for _, large := range []bool{false, true} {
			for _, mixed := range []bool{false, true} {
				name := "small/mp-only"
				if large {
					name = "large/mp-only"
				}
				if mixed {
					name += "/legacy"
				}
				t.Run(atomicAdmissionRailName(rs)+"/"+name, func(t *testing.T) {
					f := atomicAdmissionMPFixture(t)
					atomicFailureExtendedReceive(t, f)
					negotiateForwardTestFamilies(f.source, family.IPv4Unicast, family.IPv6Unicast, bgpls)
					if mixed {
						negotiateForwardTestFamilies(f.destination, family.IPv4Unicast, family.IPv6Unicast)
					} else {
						negotiateForwardTestFamilies(f.destination, family.IPv6Unicast)
					}
					f.source.refreshForwardFacts()
					f.destination.refreshForwardFacts()
					f.destination.session.adjOut = &f.destination.adjOut
					var prior []byte
					if mixed {
						prior = atomicAdmissionMPBody(t, legacy, b, nil, true, 10)
					} else {
						prior = atomicAdmissionMPBody(t, nil, append(bytes.Clone(a), b...), nil, true, 10)
					}
					rfc4271AtomicPolicyRelay(t, f, prior, rs)
					before := len(aigpSocketBodies(t, f.conn))
					base, err := message.UnpackUpdate(prior)
					require.NoError(t, err)
					padding := 8
					if large {
						padding = 5000
					}
					ls := atomicFailureLinkStateNode(padding)
					unreach := append([]byte{0x40, 4, 71}, ls...)
					attrs := append(bytes.Clone(base.PathAttributes), atomicFailureExtendedAttr(0x80, byte(attribute.AttrMPUnreachNLRI), unreach)...)
					input := makeUpdateBody(nil, attrs, base.NLRI)
					output := atomicAdmissionMPBody(t, nil, b, nil, true, 77)
					exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
					}, f.destination)
					id := f.receive(t, input)
					atomicFailureAssertAdmitted(t, f, id, output)
					atomicFailureDispatch(t, f, id, rs, false)
					var removed, kept []byte
					for _, body := range aigpSocketBodies(t, f.conn)[before:] {
						update := wireu.NewWireUpdate(body, f.destination.sendCtxID)
						_, eor := update.IsEOR()
						require.False(t, eor, "unsupported family removal cannot become a generic EOR")
						plain, err := message.UnpackUpdate(body)
						require.NoError(t, err)
						removed = append(removed, plain.WithdrawnRoutes...)
						mp, err := update.MPUnreach()
						require.NoError(t, err)
						if mp != nil {
							require.Equal(t, family.IPv6Unicast, mp.Family())
							removed = append(removed, mp.WithdrawnBytes()...)
						}
						reach, err := update.MPReach()
						require.NoError(t, err)
						if reach != nil {
							kept = append(kept, reach.NLRIBytes()...)
							metric := payloadMED(body)
							require.True(t, metric.present)
							require.Equal(t, uint32(77), binary.BigEndian.Uint32(metric.raw))
						}
					}
					if mixed {
						require.Equal(t, legacy, removed, "projecting unsupported MP must preserve the co-resident legacy removal")
					} else {
						require.Equal(t, a, removed)
					}
					require.Equal(t, b, kept, "negotiated useful B must survive unsupported sibling materialization")
				})
			}
		}
	}
}

func atomicFailureExtendedReceive(t *testing.T, f *aigpReplayFixture) {
	t.Helper()
	ctx := bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{ASN4: true, ExtendedMessageRecv: true}, bgpctx.DirectionRecv)
	id, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	f.ctxID, f.source.recvCtxID = id, id
}

func atomicFailureExtendedAttr(flags, code byte, value []byte) []byte {
	attr := []byte{flags | 0x10, code, byte(len(value) >> 8), byte(len(value))}
	return append(attr, value...)
}

// RFC 9552 Section 5.2.1 permits unknown descriptor sub-TLVs to remain opaque.
// One well-framed Local Node Descriptor contains one unknown sub-TLV, so the
// large fixture is one indivisible native key, rather than many splittable keys.
func atomicFailureLinkStateNode(padding int) []byte {
	nlri := make([]byte, 21+padding)
	binary.BigEndian.PutUint16(nlri, 1)
	binary.BigEndian.PutUint16(nlri[2:], uint16(len(nlri)-4))
	nlri[4] = 1
	binary.BigEndian.PutUint16(nlri[13:], 256)
	binary.BigEndian.PutUint16(nlri[15:], uint16(4+padding))
	binary.BigEndian.PutUint16(nlri[17:], 65000)
	binary.BigEndian.PutUint16(nlri[19:], uint16(padding))
	return nlri
}

func atomicFailureAssertAdmitted(t *testing.T, f *aigpReplayFixture, id uint64, output []byte) [][]byte {
	t.Helper()
	received, present := f.r.recentUpdates.Get(id)
	require.True(t, present)
	result := f.r.runEgressPolicyChainASN4(f.destination.settings.ExportPolicy, f.destination.addrString,
		f.destination.PeerAS(), f.destination.settings.LocalAS, false, policyview.Session{EBGP: true}, received.WireUpdate, true)
	require.True(t, result.accept, "the policy must accept before the independent destination encoding failure")
	require.False(t, result.failed)
	require.NotNil(t, result.wireOverride)
	require.Equal(t, output, result.wireOverride.Payload())
	require.NotEmpty(t, result.removals, "accepted subset must actually owe a source retirement")
	return result.removals
}

func atomicFailureDispatch(t *testing.T, f *aigpReplayFixture, id uint64, rs, failure bool) {
	t.Helper()
	if rs {
		update, present := f.r.recentUpdates.Get(id)
		require.True(t, present)
		skipped, dispatched := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
		require.Empty(t, skipped)
		if failure {
			require.Zero(t, dispatched, "cleanup alone cannot report delivery of the unencodable survivor")
		} else {
			require.Equal(t, 1, dispatched)
		}
		f.source.session.flushFwdDirty()
	} else {
		sel, err := selector.Parse(f.destination.Settings().Address.String())
		require.NoError(t, err)
		err = (&reactorAPIAdapter{r: f.r}).ForwardUpdate(sel, id, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder"))
		if failure {
			require.Error(t, err, "cleanup alone cannot report successful survivor forwarding")
		} else {
			require.NoError(t, err)
		}
	}
	f.drain(t)
}
