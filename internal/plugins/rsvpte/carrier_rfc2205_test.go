// RFC 2205 conformance tests for the carrier each message leaves on and for
// the received object-length alignment check.

package rsvpte

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC2205-x-1 negative -- no PATH or PathTear the engine emits ever leaves through Transport.Send, the carrier that writes no Router Alert option: over an ingress PATH, a transit PATH relay, a RESV relay and a PathTear relay, every PATH and PathTear goes through SendPath and the RESV through Send.
// RFC requirement: RFC3209-x-1 negative -- an LSP_TUNNEL PATH, originated or relayed along its ERO, never leaves through Transport.Send, which carries no Router Alert option; only SendPath carries it.
func TestRFC2205PathNeverLeavesWithoutRouterAlertCarrier(t *testing.T) {
	ingress, ingressTransport, _ := testEngine(t, rfc2205Ingress.String(), nil)
	psb := rfc2205PSB()
	setupTunnel(ingress.log, ingress.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO, Bandwidth: 3.2e9}, ingress.cfg(), ingress)

	transit, transitTransport, psb := rfc2205TransitWithPath(t)
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit}
	transit.handlePacket(Packet{Src: rfc2205Egress, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Egress)})
	transit.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPathTear(psb, rfc2205Ingress)})

	seen := map[uint8]int{}
	for _, ft := range []*fakeTransport{ingressTransport, transitTransport} {
		ft.mu.Lock()
		sent := append([]sentMsg(nil), ft.sent...)
		ft.mu.Unlock()
		for _, m := range sent {
			hdr, err := DecodeHeader(m.payload)
			require.NoError(t, err)
			viaSendPath := m.route.Destination.IsValid()
			seen[hdr.MsgType]++
			switch hdr.MsgType {
			case MsgTypePath, MsgTypePathTear, MsgTypeResvConf:
				require.True(t, viaSendPath, "message type %d left through Send, which writes no Router Alert option", hdr.MsgType)
			case MsgTypeResv:
				require.False(t, viaSendPath, "RESV is a hop-by-hop reply and leaves through Send")
			}
		}
	}
	require.Equal(t, 2, seen[MsgTypePath], "the ingress originated one PATH and the transit relayed one")
	require.Equal(t, 1, seen[MsgTypePathTear], "the transit relayed the PathTear")
	require.Equal(t, 1, seen[MsgTypeResv], "the transit relayed the RESV")
}

// RFC requirement: RFC2205-3.1.2-1 negative -- DecodeMessage refuses a received PATH whose first object declares Length 6, at least 4 but not a multiple of 4, with errBadObjLen, although the message itself is 4-aligned and the object fits inside it.
func TestRFC2205ReceivedObjectLengthNotMultipleOfFourRejected(t *testing.T) {
	msg := make([]byte, rsvpHdrLen+8)
	msg[0] = 0x10 // Version 1, no flags
	msg[1] = MsgTypePath
	binary.BigEndian.PutUint16(msg[6:8], uint16(len(msg)))
	binary.BigEndian.PutUint16(msg[rsvpHdrLen:], 6) // object Length 6
	msg[rsvpHdrLen+2] = 1                           // SESSION class
	msg[rsvpHdrLen+3] = 7                           // LSP_TUNNEL_IPv4 C-Type

	_, err := DecodeMessage(msg)
	require.Error(t, err)
	require.True(t, errors.Is(err, errBadObjLen), "unaligned object Length refused as a bad object length, got %v", err)
}

// sentRoutes returns the PathRoute of every message of msgType the fake
// transport carried through SendPath, in send order.
func sentRoutes(t *testing.T, ft *fakeTransport, msgType uint8) []PathRoute {
	t.Helper()
	ft.mu.Lock()
	defer ft.mu.Unlock()
	var routes []PathRoute
	for i := range ft.sent {
		m := &ft.sent[i]
		hdr, err := DecodeHeader(m.payload)
		require.NoError(t, err)
		if hdr.MsgType == msgType {
			routes = append(routes, m.route)
		}
	}
	return routes
}

// ingressPathRoute returns the PATH route of an ingress that signaled the
// rfc2205PSB tunnel, whose ERO first hop is the transit.
func ingressPathRoute(t *testing.T) PathRoute {
	t.Helper()
	e, ft, _ := testEngine(t, rfc2205Ingress.String(), nil)
	psb := rfc2205PSB()
	setupTunnel(e.log, e.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO, Bandwidth: 3.2e9}, e.cfg(), e)
	routes := sentRoutes(t, ft, MsgTypePath)
	require.Len(t, routes, 1, "the ingress signals one PATH")
	return routes[0]
}

// RFC requirement: RFC2205-3-2 positive -- the PATH an ingress originates and the PATH a transit relays are both handed to SendPath with IP source = the sender the SENDER_TEMPLATE describes and IP destination = the session's tunnel endpoint.
func TestRFC2205PathIPAddressesAreSenderAndSession(t *testing.T) {
	origin := ingressPathRoute(t)
	require.Equal(t, rfc2205Ingress, origin.Source, "originated PATH IP source")
	require.Equal(t, rfc2205Egress, origin.Destination, "originated PATH IP destination")

	_, ft, _ := rfc2205TransitWithPath(t)
	relayed := sentRoutes(t, ft, MsgTypePath)
	require.Len(t, relayed, 1)
	require.Equal(t, rfc2205Ingress, relayed[0].Source, "relayed PATH keeps the sender as IP source")
	require.Equal(t, rfc2205Egress, relayed[0].Destination, "relayed PATH keeps the session as IP destination")
}

// RFC requirement: RFC2205-3-2 negative -- a PATH never takes a hop's address in place of the sender or the session: the ingress's PATH toward next hop 10.0.0.5 is not addressed to that hop, and the transit's relay does not carry the transit's own address as IP source.
func TestRFC2205PathIPAddressesNeverHopAddresses(t *testing.T) {
	origin := ingressPathRoute(t)
	require.Equal(t, rfc2205Transit, origin.NextHop, "fixture: the ERO sends the PATH to the transit")
	require.NotEqual(t, origin.NextHop, origin.Destination, "PATH IP destination is not the next hop")

	_, ft, _ := rfc2205TransitWithPath(t)
	relayed := sentRoutes(t, ft, MsgTypePath)
	require.Len(t, relayed, 1)
	require.NotEqual(t, rfc2205Transit, relayed[0].Source, "a relaying transit does not put its own address in the IP source")
}

// RFC requirement: RFC2205-3-12 positive -- the PathTear a transit relays is handed to SendPath with the same IP source (the sender), IP destination (the session) and next hop as the PATH it relayed for that session.
func TestRFC2205PathTearRoutedLikePath(t *testing.T) {
	e, ft, psb := rfc2205TransitWithPath(t)
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPathTear(psb, rfc2205Ingress)})
	paths, tears := sentRoutes(t, ft, MsgTypePath), sentRoutes(t, ft, MsgTypePathTear)
	require.Len(t, paths, 1)
	require.Len(t, tears, 1)
	require.Equal(t, paths[0].Source, tears[0].Source, "PathTear IP source")
	require.Equal(t, paths[0].Destination, tears[0].Destination, "PathTear IP destination")
	require.Equal(t, paths[0].NextHop, tears[0].NextHop, "PathTear next hop")
	require.Equal(t, rfc2205Ingress, tears[0].Source)
	require.Equal(t, rfc2205Egress, tears[0].Destination)
}

// RFC requirement: RFC2205-3-12 negative -- a relayed PathTear never leaves addressed like a hop-by-hop message: its IP source is not the relaying transit's own address and it does not leave through Send, the unicast reply carrier.
func TestRFC2205PathTearNotAddressedHopByHop(t *testing.T) {
	e, ft, psb := rfc2205TransitWithPath(t)
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPathTear(psb, rfc2205Ingress)})
	tears := sentRoutes(t, ft, MsgTypePathTear)
	require.Len(t, tears, 1)
	require.True(t, tears[0].Destination.IsValid(), "PathTear left through SendPath, not Send")
	require.NotEqual(t, rfc2205Transit, tears[0].Source, "PathTear IP source is not the transit's own address")
}
