// Design: docs/architecture/bgp/structural-forwarding.md -- accepted source removal cannot leave an obsolete advertisement.
// Related: forward_policy_removal.go -- preparation before destination dispatch.
// Related: filter_atomic_forward_failure_test.go -- native-key encoding fixtures and forwarding rails.
package reactor

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAtomicPreparationFailureRetiresDestination drives a real accepted A+B
// subset whose original negotiated BGP-LS withdrawal cannot fit the recipient.
// A prior A must not remain on an open session when its removal cannot be
// prepared. Small withdrawals keep B useful and preserve a foreign owner of A.
// MUTATION: release and skip a failed removal group without retiring its
// captured destination session; the large rows leave that session open.
func TestAtomicPreparationFailureRetiresDestination(t *testing.T) {
	legacy := []byte{24, 10, 42, 0}
	native := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}
	bgpls := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	for _, rs := range []bool{false, true} {
		for _, large := range []bool{false, true} {
			for _, foreignOwner := range []bool{false, true} {
				name := atomicAdmissionRailName(rs) + "/small/"
				if large {
					name = atomicAdmissionRailName(rs) + "/large/"
				}
				if foreignOwner {
					name += "foreign-owner"
				} else {
					name += "source-owner"
				}
				t.Run(name, func(t *testing.T) {
					f := atomicAdmissionMPFixture(t)
					atomicFailureExtendedReceive(t, f)
					for _, peer := range []*Peer{f.source, f.destination} {
						negotiateForwardTestFamilies(peer, family.IPv4Unicast, family.IPv6Unicast, bgpls)
						peer.refreshForwardFacts()
					}
					f.source.session.negotiated.ExtendedMessageRecv = true
					f.source.negotiated.Store(NewNegotiatedCapabilities(f.source.session.negotiated))
					f.source.refreshForwardFacts()
					session := f.destination.currentSession()
					session.adjOut = &f.destination.adjOut
					prior := atomicAdmissionMPBody(t, legacy, native, nil, true, 10)
					if foreignOwner {
						rfc4271AtomicPolicyRelay(t, f, atomicAdmissionMPBody(t, nil, native, nil, true, 10), rs)
						foreign, _ := newAnnouncePeer(t, "198.18.233.3")
						foreign.recvCtxID = f.ctxID
						require.NoError(t, ownershipWriterForward(t, f.destination, foreign, rfc4271ProofRelayBody(legacy, true, 10), false))
					} else {
						rfc4271AtomicPolicyRelay(t, f, prior, rs)
					}
					var seededLegacy, seededNative []byte
					for _, body := range aigpSocketBodies(t, f.conn) {
						plain, err := message.UnpackUpdate(body)
						require.NoError(t, err)
						seededLegacy = append(seededLegacy, plain.NLRI...)
						reach, err := wireu.NewWireUpdate(body, session.sendCtxID).MPReach()
						require.NoError(t, err)
						if reach != nil {
							seededNative = append(seededNative, reach.NLRIBytes()...)
						}
					}
					require.Equal(t, legacy, seededLegacy, "the recipient must actually hold A before the failed preparation")
					require.Equal(t, native, seededNative, "the recipient must actually hold useful B")
					before := len(f.conn.written())
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
					require.True(t, f.source.session.negotiated.ExtendedMessageRecv, "the source session permits the extended received UPDATE")
					require.LessOrEqual(t, len(input)+message.HeaderLen, message.ExtMsgLen)
					require.False(t, session.negotiated.ExtendedMessageSend, "the recipient still requires standard-size UPDATEs")
					output := atomicAdmissionMPBody(t, nil, native, nil, true, 77)
					exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
					}, f.destination)
					id := f.receive(t, input)
					removals := atomicFailureAssertAdmitted(t, f, id, output)
					var owedLegacy, owedLS []byte
					for _, body := range removals {
						plain, err := message.UnpackUpdate(body)
						require.NoError(t, err)
						owedLegacy = append(owedLegacy, plain.WithdrawnRoutes...)
						withdrawn, err := wireu.NewWireUpdate(body, f.ctxID).MPUnreach()
						require.NoError(t, err)
						if withdrawn != nil {
							require.Equal(t, bgpls, withdrawn.Family())
							owedLS = append(owedLS, withdrawn.WithdrawnBytes()...)
						}
					}
					require.Equal(t, legacy, owedLegacy, "accepted policy owes exactly the original legacy A")
					require.Equal(t, ls, owedLS, "the original negotiated withdrawal must not disappear")
					received, present := f.r.recentUpdates.Get(id)
					require.True(t, present)
					probe := fwdItem{peer: f.destination, session: session, authority: adjOutForwarded,
						receivedPeer: received.receivedPeer, receivedGeneration: received.receivedGeneration,
						sourceMessageID: id, sourcePolicyEpoch: received.SourcePolicyEpoch}
					err = prepareFwdPolicyRemovals(&probe, received, received.WireUpdate, removals, f.destination.forwardFacts())
					f.r.fwdPool.releaseItem(&probe)
					if large {
						require.Len(t, ls, 5021)
						require.ErrorIs(t, err, errPolicyRemovalEncoding, "setup must reach the real indivisible-key encoding failure")
					} else {
						require.NoError(t, err, "the negotiated small withdrawal remains encodable")
					}
					atomicFailureDispatch(t, f, id, rs, large)
					frames := bgpFrames(t, f.conn.written()[before:])
					if large {
						require.True(t, session.tearingDown.Load(), "unprepared source retirement must not leave an obsolete A on a live session")
						session.mu.RLock()
						closed := session.conn == nil
						session.mu.RUnlock()
						require.True(t, closed, "the captured connection must actually close")
						require.Len(t, frames, 1, "fatal preparation failure closes the session without a forged selective withdrawal or EOR")
						require.Equal(t, byte(msgtype.TypeNOTIFICATION), frames[0][18])
						require.Equal(t, []byte{byte(message.NotifyCease), message.NotifyCeaseOutOfResources}, frames[0][message.HeaderLen:])
						return
					}
					require.False(t, session.tearingDown.Load(), "a useful encodable subset must keep its session")
					var removedLegacy, kept []byte
					for _, frame := range frames {
						require.Equal(t, byte(msgtype.TypeUPDATE), frame[18])
						body := frame[message.HeaderLen:]
						wire := wireu.NewWireUpdate(body, session.sendCtxID)
						_, eor := wire.IsEOR()
						require.False(t, eor)
						plain, err := message.UnpackUpdate(body)
						require.NoError(t, err)
						removedLegacy = append(removedLegacy, plain.WithdrawnRoutes...)
						reach, err := wire.MPReach()
						require.NoError(t, err)
						if reach != nil {
							kept = append(kept, reach.NLRIBytes()...)
							metric := payloadMED(body)
							require.True(t, metric.present)
							require.Equal(t, uint32(77), binary.BigEndian.Uint32(metric.raw))
						}
					}
					if foreignOwner {
						require.Empty(t, removedLegacy, "an encodable source cleanup cannot remove foreign-owned A")
					} else {
						require.Equal(t, legacy, removedLegacy)
					}
					require.Equal(t, native, kept, "small negotiated withdrawals must preserve useful B")
				})
			}
		}
	}
}
