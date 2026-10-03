package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// newAddPathSessionForOne builds a validation session that negotiates
// Multiprotocol for IPv4 unicast and IPv6 unicast and ADD-PATH (send and
// receive) for addPath alone, with the matching receive encoding context.
func newAddPathSessionForOne(t *testing.T, addPath capability.Family) *Session {
	t.Helper()
	s := newValidateSession()
	caps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		&capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: addPath.AFI, SAFI: addPath.SAFI, Mode: capability.AddPathBoth},
		}},
	}
	s.negotiated = capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002})
	ctxID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedRecv(s.negotiated))
	require.NoError(t, err)
	s.setRecvCtxID(ctxID)
	return s
}

// addPathIPv4Body returns an UPDATE whose body NLRI is 10.0.0.0/24, with the
// 4-octet Path Identifier 33 in front of it when withPathID is set. Read
// without ADD-PATH, the path-id octet 0x21 is prefix length 33, beyond 32.
func addPathIPv4Body(withPathID bool) []byte {
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 10, 0, 0, 1}
	nlri := []byte{24, 10, 0, 0}
	if withPathID {
		nlri = []byte{0, 0, 0, 0x21, 24, 10, 0, 0}
	}
	return makeUpdateBody(nil, attrs, nlri)
}

// addPathIPv6Body returns an UPDATE whose MP_REACH_NLRI carries 2001:db8::/32,
// with the 4-octet Path Identifier 129 in front of it when withPathID is set.
// Read without ADD-PATH, the path-id octet 0x81 is prefix length 129, beyond 128.
func addPathIPv6Body(withPathID bool) []byte {
	mp := []byte{0, 2, 1, 16, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0}
	if withPathID {
		mp = append(mp, 0, 0, 0, 0x81)
	}
	mp = append(mp, 32, 0x20, 0x01, 0x0d, 0xb8)
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00}
	attrs = append(attrs, 0x80, 0x0e, byte(len(mp)))
	attrs = append(attrs, mp...)
	return makeUpdateBody(nil, attrs, nil)
}

// addPathAction runs one UPDATE body through the session's RFC 7606 receive
// enforcement and returns the action it chose.
func addPathAction(s *Session, body []byte) message.RFC7606Action {
	_, action, _ := s.enforceRFC7606(wireu.NewWireUpdate(body, 0)) //nolint:errcheck // the action carries the verdict under test
	return action
}

// Goal: prove the receiver applies the ADD-PATH state of each <AFI, SAFI> to
// that family alone, so a session that negotiated ADD-PATH for one family
// still reads another family's NLRI without a Path Identifier.
// Method: on a session with Multiprotocol for IPv4 and IPv6 unicast and
// ADD-PATH for exactly one of them, feed the path-id and the plain encoding of
// both families to enforceRFC7606 and read the RFC 7606 action.
//
// VALIDATES: RFC 7911 Section 5, processing per <AFI, SAFI>.
// PREVENTS: one family's ADD-PATH state applied to every family.
//
// RFC requirement: RFC7911-5-5 positive -- with ADD-PATH negotiated for IPv6 unicast only, an IPv6 MP_REACH_NLRI carrying a Path Identifier and a plain IPv4 body NLRI are both accepted (RFC7606ActionNone); with ADD-PATH for IPv4 unicast only, an IPv4 body NLRI carrying a Path Identifier and a plain IPv6 MP_REACH_NLRI are both accepted.
// RFC requirement: RFC7911-5-5 negative -- on the same sessions the family that did not negotiate ADD-PATH is read without a Path Identifier: an IPv4 body NLRI carrying one (IPv6-only session) and an IPv6 MP_REACH_NLRI carrying one (IPv4-only session) each reset the session, and the plain encoding of the negotiated family is refused the same way.
func TestRFC7911AddPathStateAppliesPerFamily(t *testing.T) {
	ipv4 := capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}
	ipv6 := capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}

	v6Only := newAddPathSessionForOne(t, ipv6)
	require.Equal(t, message.RFC7606ActionNone, addPathAction(v6Only, addPathIPv6Body(true)))
	require.Equal(t, message.RFC7606ActionNone, addPathAction(v6Only, addPathIPv4Body(false)))
	require.Equal(t, message.RFC7606ActionSessionReset, addPathAction(v6Only, addPathIPv4Body(true)))
	require.Equal(t, message.RFC7606ActionSessionReset, addPathAction(v6Only, addPathIPv6Body(false)),
		"2001:db8::/32 read as a Path Identifier leaves a truncated prefix")

	v4Only := newAddPathSessionForOne(t, ipv4)
	require.Equal(t, message.RFC7606ActionNone, addPathAction(v4Only, addPathIPv4Body(true)))
	require.Equal(t, message.RFC7606ActionNone, addPathAction(v4Only, addPathIPv6Body(false)))
	require.Equal(t, message.RFC7606ActionSessionReset, addPathAction(v4Only, addPathIPv6Body(true)))
	require.Equal(t, message.RFC7606ActionSessionReset, addPathAction(v4Only, addPathIPv4Body(false)),
		"10.0.0.0/24 read as a Path Identifier leaves no prefix length")
}
