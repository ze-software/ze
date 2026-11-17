// Design: docs/architecture/update-building.md -- canonical MP EOR completion.
package reactor

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func selectedRefreshIPv6(t *testing.T) (*selectedWriterFixture, rpc.SelectRefreshInput) {
	t.Helper()
	fixture, input := selectedRefreshFixture(t)
	selectedRefreshNegotiateFamily(t, fixture, family.IPv6Unicast)
	input.Family = "ipv6/unicast"
	return fixture, input
}

// Publish the captured OPEN capability intersection before deriving the
// encoding. Neither peer inspection state nor a shared encoding ID can supply
// the Session's family authority or its locally advertised GR support.
func selectedRefreshNegotiateFamily(t *testing.T, fixture *selectedWriterFixture, fam family.Family) {
	t.Helper()
	session := fixture.peer.session
	session.proposal = preparedOpenForTest(session, &message.Open{Version: 4, MyAS: 65000, HoldTime: 90, BGPIdentifier: 1})
	session.peerOpen = &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 2}
	local := []capability.Capability{&capability.GracefulRestart{},
		&capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI},
		&capability.RouteRefresh{}, &capability.EnhancedRouteRefresh{}}
	remote := []capability.Capability{&capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI},
		&capability.RouteRefresh{}, &capability.EnhancedRouteRefresh{}}
	session.negotiateWith(local, remote)
	require.NotNil(t, session.negotiated)
	require.Equal(t, []family.Family{fam}, session.negotiated.Families())
	require.Nil(t, session.negotiated.GracefulRestart, "the peer did not advertise GR")
	encoding := bgpctx.NewEncodingContext(session.negotiated.Identity, session.negotiated.Encoding, bgpctx.DirectionSend)
	contextID, err := bgpctx.Registry.Register(encoding)
	require.NoError(t, err)
	fixture.peer.sendCtx.Store(encoding)
	fixture.peer.sendCtxID, session.sendCtxID = contextID, contextID
	session.wireWriter.context.Store(uint32(contextID))
	fixture.peer.negotiated.Store(NewNegotiatedCapabilities(session.negotiated))
	fixture.peer.refreshForwardFacts()
}

func selectedRefreshIPv6Written(t *testing.T, fixture *selectedWriterFixture, input rpc.SelectRefreshInput) {
	t.Helper()
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	frames := splitWireMessages(t, fixture.conn.written())
	require.NotEmpty(t, frames)
	want := message.PackTo(&message.RouteRefresh{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast,
		Subtype: message.RouteRefreshBoRR}, nil)
	require.Equal(t, want, frames[len(frames)-1], "the useful result must reach the IPv6 peer")
}

// Both legal MP attribute-length forms reach the raw diagnostic writer and
// actually open the IPv6 BoRR gate, rather than merely setting helper state.
func TestSelectedRefreshIPv6CanonicalEORUnlocks(t *testing.T) {
	for _, body := range [][]byte{
		{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1},
		{0, 0, 0, 7, 0x90, 15, 0, 3, 0, 2, 1},
	} {
		t.Run(map[int]string{10: "normal", 11: "extended"}[len(body)], func(t *testing.T) {
			fixture, input := selectedRefreshIPv6(t)
			require.NoError(t, fixture.peer.session.SendRawMessage(2, body))
			frames := splitWireMessages(t, fixture.conn.written())
			require.Len(t, frames, 1)
			require.Equal(t, 19+len(body), len(frames[0]))
			require.Equal(t, body, frames[0][19:])
			selectedRefreshIPv6Written(t, fixture, input)
		})
	}
}

// Malformed Optional/Transitive flags remain observable raw diagnostic output,
// but do not become EOR evidence. A later valid EOR restores useful output.
func TestSelectedRefreshMalformedMPFlagsCannotUnlock(t *testing.T) {
	for _, flags := range []byte{0, 0x40, 0xc0, 0xa0, 0x10, 0x50, 0xd0, 0xb0} {
		t.Run(fmt.Sprintf("flags-%02x", flags), func(t *testing.T) {
			fixture, input := selectedRefreshIPv6(t)
			body := []byte{0, 0, 0, 6, flags, 15, 3, 0, 2, 1}
			if flags&0x10 != 0 {
				body = []byte{0, 0, 0, 7, flags, 15, 0, 3, 0, 2, 1}
			}
			require.NoError(t, fixture.peer.session.SendRawMessage(2, body))
			frames := splitWireMessages(t, fixture.conn.written())
			require.Len(t, frames, 1)
			require.Equal(t, body, frames[0][19:], "raw diagnostics must remain sendable")
			request, err := plugin.NewSelectedRefresh(input)
			require.NoError(t, err)
			_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
			require.ErrorContains(t, err, "EOR has not reached")
			require.Len(t, splitWireMessages(t, fixture.conn.written()), 1)
			require.NoError(t, fixture.peer.session.SendUpdate(message.BuildEOR(family.IPv6Unicast)))
			selectedRefreshIPv6Written(t, fixture, input)
		})
	}
}

// A complete EOR of another family cannot borrow IPv6 completion authority.
func TestSelectedRefreshWrongFamilyEORCannotUnlock(t *testing.T) {
	fixture, input := selectedRefreshIPv6(t)
	require.NoError(t, fixture.peer.session.SendUpdate(message.BuildEOR(family.IPv4Unicast)))
	request, err := plugin.NewSelectedRefresh(input)
	require.NoError(t, err)
	_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
	require.ErrorContains(t, err, "EOR has not reached")
	require.Len(t, splitWireMessages(t, fixture.conn.written()), 1)
	require.NoError(t, fixture.peer.session.SendUpdate(message.BuildEOR(family.IPv6Unicast)))
	selectedRefreshIPv6Written(t, fixture, input)
}

// Accepted bytes are not evidence until this exact writer has the whole frame.
func TestSelectedRefreshPartialMPEORCannotUnlock(t *testing.T) {
	fixture, input := selectedRefreshIPv6(t)
	session := fixture.peer.session
	encoding := bgpctx.Registry.Get(session.sendCtxID)
	frame := message.PackTo(message.BuildEOR(family.IPv6Unicast), encoding)
	require.Equal(t, len(frame), int(binary.BigEndian.Uint16(frame[16:18])))
	session.writeMu.Lock()
	accepted, err := session.wireWriter.Write(frame[:len(frame)-1])
	session.writeMu.Unlock()
	require.NoError(t, err)
	require.Equal(t, len(frame)-1, accepted)
	request, err := plugin.NewSelectedRefresh(input)
	require.NoError(t, err)
	_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
	require.ErrorContains(t, err, "EOR has not reached")
	require.Equal(t, frame[:len(frame)-1], fixture.conn.written())
	session.writeMu.Lock()
	accepted, err = session.wireWriter.Write(frame[len(frame)-1:])
	session.writeMu.Unlock()
	require.NoError(t, err)
	require.Equal(t, 1, accepted)
	selectedRefreshIPv6Written(t, fixture, input)
}
