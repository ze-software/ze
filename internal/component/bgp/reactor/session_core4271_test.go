// Design: docs/architecture/wire/messages.md -- RFC 4271 receive error handling
package reactor

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
)

func core4271ReadMessage(conn net.Conn) ([]byte, error) {
	var header [message.HeaderLen]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return nil, err
	}
	hdr, err := message.ParseHeader(header[:])
	if err != nil {
		return nil, err
	}
	packet := make([]byte, int(hdr.Length))
	copy(packet, header[:])
	_, err = io.ReadFull(conn, packet[message.HeaderLen:])
	return packet, err
}

// RFC requirement: RFC4271-6.2-9 positive -- an unknown Optional Parameter type produces 2/4 on the socket and closes the session.
// RFC requirement: RFC4271-6.2-9 negative -- an unknown capability inside recognized Type 2 still permits the OPEN handshake.
// RFC requirement: RFC4271-6.2-10 positive -- malformed Type 2 framing, capability framing and recognized capability values produce 2/0 with empty Data.
// RFC requirement: RFC4271-6.2-10 negative -- a correctly encoded Route Refresh capability reaches OpenConfirm and sends KEEPALIVE.
func TestSessionRFC4271OptionalParameterErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		params  []byte
		subcode int
	}{
		{"unknown parameter", []byte{99, 0}, 4},
		{"unknown capability", []byte{2, 2, 200, 0}, -1},
		{"valid route refresh", []byte{2, 2, 2, 0}, -1},
		{"known capability wrong length", []byte{2, 3, 2, 1, 0}, 0},
		{"capability value truncated", []byte{2, 2, 2, 1}, 0},
		{"empty capability parameter", []byte{2, 0}, 0},
		{"parameter value truncated", []byte{2, 3, 2, 0}, 0},
		{"parameter header truncated", []byte{2}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, client := newOpenSentSessionWithClient(t)
			t.Cleanup(s.timers.StopAll)
			require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
			body := validOpenBody()
			body[9] = byte(len(tc.params))
			body = append(body, tc.params...)
			packet := make([]byte, message.HeaderLen+len(body))
			for i := range message.HeaderLen - 3 {
				packet[i] = 0xff
			}
			binary.BigEndian.PutUint16(packet[16:18], uint16(len(packet)))
			packet[18] = byte(msgtype.TypeOPEN)
			copy(packet[message.HeaderLen:], body)
			type response struct {
				packet []byte
				err    error
			}
			done := make(chan response, 1)
			go func() {
				_, err := client.Write(packet)
				var reply []byte
				if err == nil {
					reply, err = core4271ReadMessage(client)
				}
				done <- response{reply, err}
			}()
			err := s.ReadAndProcess()
			reply := <-done
			require.NoError(t, reply.err)
			if tc.subcode < 0 {
				require.NoError(t, err)
				require.Equal(t, byte(msgtype.TypeKEEPALIVE), reply.packet[18])
				require.Equal(t, fsm.StateOpenConfirm, s.State())
			} else {
				require.Error(t, err)
				require.Equal(t, byte(msgtype.TypeNOTIFICATION), reply.packet[18])
				require.Equal(t, []byte{2, byte(tc.subcode)}, reply.packet[message.HeaderLen:])
				require.Equal(t, fsm.StateIdle, s.State())
			}
		})
	}
}

// RFC 7606 Sections 3(c-e), 7.1-7.4 supersede the historical reset/subcode
// requirements below. The observable result must be a withdrawal, not a silent
// drop that leaves an earlier route installed, nor a session reset.
// RFC requirement: RFC4271-6.3-5 positive -- incorrect ORIGIN flags withdraw the announced prefix under RFC 7606 Section 3(c).
// RFC requirement: RFC4271-6.3-5 negative -- the same prefix with correct flags remains announced.
// RFC requirement: RFC4271-6.3-6 positive -- a wrong NEXT_HOP length withdraws the route under RFC 7606 Sections 3(e) and 7.3.
// RFC requirement: RFC4271-6.3-6 negative -- a four-octet NEXT_HOP is accepted.
// RFC requirement: RFC4271-6.3-7 positive -- missing ORIGIN, AS_PATH or NEXT_HOP withdraws the route under RFC 7606 Section 3(d).
// RFC requirement: RFC4271-6.3-7 negative -- all mandatory attributes present permit announcement.
// RFC requirement: RFC4271-6.3-9 positive -- ORIGIN value 3 withdraws the route under RFC 7606 Section 7.1.
// RFC requirement: RFC4271-6.3-9 negative -- ORIGIN value IGP permits announcement.
// RFC requirement: RFC4271-6.3-10 positive -- multicast NEXT_HOP is syntactically invalid and withdraws the route, per RFC 7606 Section 3(e).
// RFC requirement: RFC4271-6.3-10 negative -- unicast NEXT_HOP permits announcement.
// RFC requirement: RFC4271-6.3-12 positive -- unknown segment type, zero segment count, truncated segment and a trailing AS_PATH octet each withdraw the prefix under RFC 7606 Section 7.2.
// RFC requirement: RFC4271-6.3-12 negative -- a correctly framed AS_SEQUENCE permits announcement.
// RFC requirement: RFC4271-6.3-14 positive -- malformed recognized optional MED withdraws the route under RFC 7606 Section 3(e).
// RFC requirement: RFC4271-6.3-14 negative -- a correctly encoded optional MED preserves the announcement.
func TestSessionRFC4271RevisedAttributeErrors(t *testing.T) {
	origin := collapseAttr(0x40, 1, []byte{0})
	path := collapseAttr(0x40, 2, []byte{2, 1, 0xfd, 0xea})
	nextHop := collapseAttr(0x40, 3, []byte{192, 0, 2, 254})
	for _, tc := range []struct {
		name                            string
		origin, path, nextHop, optional []byte
	}{
		{"flags", collapseAttr(0x80, 1, []byte{0}), path, nextHop, nil},
		{"length", origin, path, collapseAttr(0x40, 3, []byte{192, 0, 2}), nil},
		{"missing origin", nil, path, nextHop, nil},
		{"missing path", origin, nil, nextHop, nil},
		{"missing next hop", origin, path, nil, nil},
		{"origin value", collapseAttr(0x40, 1, []byte{3}), path, nextHop, nil},
		{"multicast next hop", origin, path, collapseAttr(0x40, 3, []byte{224, 0, 0, 1}), nil},
		{"unknown segment", origin, collapseAttr(0x40, 2, []byte{99, 1, 0xfd, 0xea}), nextHop, nil},
		{"empty segment", origin, collapseAttr(0x40, 2, []byte{2, 0}), nextHop, nil},
		{"segment overrun", origin, collapseAttr(0x40, 2, []byte{2, 2, 0xfd, 0xea}), nextHop, nil},
		{"trailing path octet", origin, collapseAttr(0x40, 2, []byte{2, 1, 0xfd, 0xea, 2}), nextHop, nil},
		{"optional MED", origin, path, nextHop, collapseAttr(0x80, 4, []byte{0, 0, 0})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, client := firstASSession(t, NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301), nil, nil)
			prefix := []byte{24, 203, 0, 113}
			good := append(firstASAttrs(2, 65002), collapseAttr(0x80, 4, []byte{0, 0, 0, 10})...)
			bad := append(append(append(append([]byte{}, tc.origin...), tc.path...), tc.nextHop...), tc.optional...)
			for _, attrs := range [][]byte{good, bad, good} {
				wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, prefix))
				if bytes.Equal(attrs, bad) {
					require.Equal(t, makeUpdateBody(prefix, nil, nil), wu.Payload())
				} else {
					nlri, err := wu.NLRI()
					require.NoError(t, err)
					require.Equal(t, prefix, nlri)
				}
			}
		})
	}
}

// RFC requirement: RFC4271-6.3-8 positive -- an unknown well-known attribute resets with 3/2 and the entire offending attribute, including its extended-length header.
// RFC requirement: RFC4271-6.3-8 negative -- the same unrecognized code marked optional is accepted.
// RFC requirement: RFC4271-6.3-16 positive -- impossible prefix length and truncated IPv4 NLRI reset with 3/10 and empty Data.
// RFC requirement: RFC4271-6.3-16 negative -- a valid prefix remains announced over the same receive path.
func TestSessionRFC4271RetainedUpdateNotifications(t *testing.T) {
	unknown := []byte{0x50, 250, 0, 2, 0xab, 0xcd}
	for _, tc := range []struct {
		name                   string
		extra, withdrawn, nlri []byte
		subcode                byte
		data                   []byte
	}{
		{"unknown well-known", unknown, nil, []byte{24, 203, 0, 113}, 2, unknown},
		{"NLRI impossible length", nil, nil, []byte{33, 203, 0, 113, 1, 1}, 10, nil},
		{"NLRI truncated", nil, nil, []byte{24, 203, 0}, 10, nil},
		{"withdrawn truncated", nil, []byte{24, 203, 0}, nil, 10, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, client := firstASSession(t, NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301), nil, nil)
			good := append(firstASAttrs(2, 65002), []byte{0xd0, 250, 0, 2, 0xab, 0xcd}...)
			prefix := []byte{24, 203, 0, 113}
			accepted := firstASReceive(t, s, client, makeUpdateBody(nil, good, prefix))
			nlri, err := accepted.NLRI()
			require.NoError(t, err)
			require.Equal(t, prefix, nlri)
			body := makeUpdateBody(tc.withdrawn, append(firstASAttrs(2, 65002), tc.extra...), tc.nlri)
			type response struct {
				packet []byte
				err    error
			}
			done := make(chan response, 1)
			go func() {
				_, err := client.Write(buildUpdateMsg(body))
				var reply []byte
				if err == nil {
					reply, err = core4271ReadMessage(client)
				}
				done <- response{reply, err}
			}()
			require.Error(t, s.ReadAndProcess())
			reply := <-done
			require.NoError(t, reply.err)
			require.Equal(t, byte(msgtype.TypeNOTIFICATION), reply.packet[18])
			require.Equal(t, append([]byte{3, tc.subcode}, tc.data...), reply.packet[message.HeaderLen:])
			require.Equal(t, fsm.StateIdle, s.State())
		})
	}
}

// RFC requirement: RFC4271-6.3-11 positive -- local next hops and off-link third-party next hops on a one-hop eBGP session withdraw only the legacy announcement without resetting.
// RFC requirement: RFC4271-6.3-11 negative -- the sender's address and a third-party host on a connected subnet are accepted; a multihop session accepts an off-link next hop.
func TestSessionRFC4271NextHopSemantics(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.LocalAddress = netip.MustParseAddr("192.0.2.2")
	s, client := firstASSession(t, settings, nil, nil)
	s.nextHopScope.Store(newReceiveNextHopScope([]netip.Prefix{
		netip.MustParsePrefix("192.0.2.2/24"), netip.MustParsePrefix("198.51.100.1/24"),
	}, client, settings))
	prefix := []byte{24, 203, 0, 113}
	for _, tc := range []struct {
		nextHop string
		invalid bool
	}{
		{"192.0.2.1", false}, {"192.0.2.254", false}, {"198.51.100.254", false},
		{"192.0.2.2", true}, {"198.51.100.1", true}, {"203.0.113.1", true},
	} {
		t.Run(tc.nextHop, func(t *testing.T) {
			attrs := append([]byte{}, firstASAttrs(2, 65002)...)
			addr := netip.MustParseAddr(tc.nextHop).As4()
			copy(attrs[len(attrs)-4:], addr[:])
			wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, prefix))
			nlri, err := wu.NLRI()
			require.NoError(t, err)
			if tc.invalid {
				require.Empty(t, nlri)
				require.Equal(t, prefix, wu.Payload()[2:6])
			} else {
				require.Equal(t, prefix, nlri)
			}
		})
	}
	s.nextHopScope.Store(newReceiveNextHopScope([]netip.Prefix{netip.MustParsePrefix("198.51.100.1/24")}, client, settings))
	attrs := firstASAttrs(2, 65002)
	copy(attrs[len(attrs)-4:], []byte{203, 0, 113, 1})
	wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, prefix))
	nlri, err := wu.NLRI()
	require.NoError(t, err)
	require.Equal(t, prefix, nlri)
}

// RFC requirement: RFC4271-8.2.2-19 positive -- a second connection in Established remains readable through a partial OPEN, while the existing session still receives routes.
// RFC requirement: RFC4271-8.2.2-19 negative -- only the completed OPEN triggers collision rejection of the new connection; the established session survives.
func TestSessionRFC4271EstablishedCollisionWaitsForOpen(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s, existing := firstASSession(t, settings, nil, nil)
	peer := NewPeer(settings)
	peer.session = s
	peer.setState(PeerStateEstablished)
	r := &Reactor{clock: clock.RealClock{}}
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
	accepted := make(chan struct{})
	go func() { r.acceptOrReject(server, peer, nil); close(accepted) }()
	packet := message.PackTo(&message.Open{Version: 4, MyAS: 65002, BGPIdentifier: 0xffffffff}, nil)
	_, err := client.Write(packet[:len(packet)-1])
	require.NoError(t, err, "the pending connection must read until OPEN is complete")
	<-accepted
	require.True(t, peer.hasPendingConnection())
	prefix := []byte{24, 203, 0, 113}
	wu := firstASReceive(t, s, existing, makeUpdateBody(nil, firstASAttrs(2, 65002), prefix))
	nlri, err := wu.NLRI()
	require.NoError(t, err)
	require.Equal(t, prefix, nlri)
	_, err = client.Write(packet[len(packet)-1:])
	require.NoError(t, err)
	reply, err := core4271ReadMessage(client)
	require.NoError(t, err)
	require.Equal(t, byte(msgtype.TypeNOTIFICATION), reply[18])
	require.Equal(t, []byte{6, 7}, reply[message.HeaderLen:])
	require.False(t, peer.hasPendingConnection())
	require.Equal(t, fsm.StateEstablished, s.State())
}
