// Design: docs/architecture/update-building.md -- final originated next-hop admission
// Related: forward_next_hop.go -- nextHopValue.has compares wire and peer addresses
// RFC 4271 Section 5.1.3 -- see rfc/short/rfc4271.md.

package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestScopedNextHopPeerSettingsRetainsZone establishes the real static-peer
// parser input. Transport scope remains part of the configured peer identity.
func TestScopedNextHopPeerSettingsRetainsZone(t *testing.T) {
	settings := scopedNextHopSettings(t)
	address := netip.MustParseAddr("fe80::2%if0")
	assert.Equal(t, address, settings.Address)
	assert.Equal(t, "if0", settings.Address.Zone())
	assert.Equal(t, netip.AddrPortFrom(address, DefaultBGPPort), settings.PeerKey())
	assert.Equal(t, NextHopUnchanged, settings.NextHopMode)

	session := NewSession(settings)
	assert.Same(t, settings, session.settings)
	assert.Equal(t, address, session.settings.Address)
}

// TestScopedNextHopComparisonIgnoresTransportZone compares wire-decoded MP32
// values with a scoped peer, retaining the different-address and invalid controls.
func TestScopedNextHopComparisonIgnoresTransportZone(t *testing.T) {
	_, body, _ := scopedNextHopUpdate(t, netip.MustParseAddr("fe80::2"))
	nextHop := payloadNextHop(body)
	require.Equal(t, netip.MustParseAddr("2001:db8::9"), nextHop.mp)
	require.Equal(t, netip.MustParseAddr("fe80::2"), nextHop.mpLL)
	require.Empty(t, nextHop.mp.Zone())
	require.Empty(t, nextHop.mpLL.Zone())
	assert.True(t, nextHop.has(netip.MustParseAddr("fe80::2")))
	assert.True(t, nextHop.has(netip.MustParseAddr("fe80::2%if0")),
		"an interface zone does not change the recipient address on the wire")
	assert.False(t, nextHop.has(netip.MustParseAddr("fe80::3%if0")))
	assert.False(t, nextHop.has(netip.Addr{}))
}

// TestRFC4271ScopedNextHopWithholdsPeerOwn observes flushed output from three
// originated writers for a configured scoped recipient and a valid MP32 pair.
// RFC requirement: RFC4271-5.1.3-1 positive -- raw body, raw full-packet and typed SendUpdate do not advertise IPv6 NLRI with the scoped recipient's address in the MP32 link-local slot.
// MUTATION: retaining the query zone in nextHopValue.has emits the forbidden announcement on all three writers.
func TestRFC4271ScopedNextHopWithholdsPeerOwn(t *testing.T) {
	for _, rail := range []string{"raw-body", "raw-packet", "typed"} {
		t.Run(rail, func(t *testing.T) {
			scopedNextHopCheck(t, rail, true)
		})
	}
}

// TestRFC4271ScopedNextHopPreservesThirdParty changes only the MP32 link-local
// address and requires the complete next-hop field and exact announced prefix.
// RFC requirement: RFC4271-5.1.3-1 negative -- the same raw body, raw full-packet and typed writers advertise the exact IPv6 prefix and MP32 field when only its link-local slot changes to a different address.
// MUTATION: an unconditional peer-own refusal loses the permitted announcement on all three writers.
func TestRFC4271ScopedNextHopPreservesThirdParty(t *testing.T) {
	for _, rail := range []string{"raw-body", "raw-packet", "typed"} {
		t.Run(rail, func(t *testing.T) {
			scopedNextHopCheck(t, rail, false)
		})
	}
}

func scopedNextHopCheck(t *testing.T, rail string, forbidden bool) {
	t.Helper()
	peer, conn := scopedNextHopPeer(t)
	linkLocal := netip.MustParseAddr("fe80::3")
	if forbidden {
		linkLocal = netip.MustParseAddr("fe80::2")
	}
	update, body, field := scopedNextHopUpdate(t, linkLocal)
	switch rail {
	case "raw-body":
		require.NoError(t, peer.SendRawMessage(uint8(msgtype.TypeUPDATE), body))
	case "raw-packet":
		packet := make([]byte, message.HeaderLen+len(body))
		copy(packet, message.Marker[:])
		binary.BigEndian.PutUint16(packet[16:18], uint16(len(packet)))
		packet[18] = byte(msgtype.TypeUPDATE)
		copy(packet[message.HeaderLen:], body)
		require.NoError(t, peer.SendRawMessage(0, packet))
	case "typed":
		require.NoError(t, peer.SendUpdate(update))
	default:
		t.Fatalf("unknown originated writer %q", rail)
	}
	assert.Equal(t, netip.MustParseAddr("fe80::2%if0"), peer.settings.Address)
	assert.Equal(t, peer.settings.PeerKey(), peer.session.settings.PeerKey())
	assert.Equal(t, "if0", conn.remote.Zone)
	assert.Equal(t, peer.settings.Address, conn.remote.AddrPort().Addr())
	assert.Equal(t, fsm.StateEstablished, peer.session.fsm.State())
	assert.False(t, conn.isClosed(), "recognized raw UPDATEs keep the session usable")

	output := conn.written()
	if forbidden {
		assert.Empty(t, output, "the recipient must not receive its own link-local next hop")
		return
	}
	require.GreaterOrEqual(t, len(output), message.HeaderLen)
	assert.Equal(t, message.Marker[:], output[:16])
	require.Equal(t, len(output), int(binary.BigEndian.Uint16(output[16:18])),
		"the permitted twin must emit one complete frame")
	assert.Equal(t, byte(msgtype.TypeUPDATE), output[18])
	sections, err := wire.ParseUpdateSections(output[message.HeaderLen:])
	require.NoError(t, err)
	actualBody := output[message.HeaderLen:]
	assert.Empty(t, sections.Withdrawn(actualBody))
	assert.Empty(t, sections.NLRI(actualBody))
	attrs := sections.Attrs(actualBody)
	_, _, _, withdrawn := attribute.AttrFind(attrs, attribute.AttrMPUnreachNLRI)
	assert.False(t, withdrawn, "a withdrawal cannot satisfy the permitted announcement")
	_, _, reach, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	require.True(t, found)
	require.Len(t, reach, 42)
	assert.Equal(t, []byte{0, 2, 1, 32}, reach[:4])
	assert.Equal(t, field, reach[4:36], "all 32 next-hop octets must survive")
	assert.Zero(t, reach[36])
	assert.Equal(t, []byte{32, 0x20, 1, 0x0d, 0xb8}, reach[37:])
}

func scopedNextHopSettings(t *testing.T) *PeerSettings {
	t.Helper()
	tree, err := config.ParseTreeForValidation(`bgp {
    router-id 1.2.3.4;
    peer scoped {
        connection {
            local { ip auto; }
            remote { ip fe80::2%if0; }
        }
        session {
            asn { local 65000; remote 65001; }
            next-hop unchanged;
        }
    }
}`)
	require.NoError(t, err)
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers, err := PeersFromTree(bgp.ToMap())
	require.NoError(t, err)
	require.Len(t, peers, 1)
	return peers[0]
}

func scopedNextHopPeer(t *testing.T) (*Peer, *scopedNextHopConn) {
	t.Helper()
	settings := scopedNextHopSettings(t)
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	session := NewSession(settings)
	for _, event := range []fsm.Event{
		fsm.EventManualStart, fsm.EventTCPConnectionConfirmed,
		fsm.EventBGPOpen, fsm.EventKeepaliveMsg,
	} {
		require.NoError(t, session.fsm.Event(event))
	}
	conn := &scopedNextHopConn{
		local:  &net.TCPAddr{IP: net.ParseIP("fe80::1"), Port: 179, Zone: "if0"},
		remote: &net.TCPAddr{IP: settings.Address.AsSlice(), Port: 179, Zone: settings.Address.Zone()},
	}
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.transport.Store(connectedTransport(conn))
	peer.session = session
	local := []capability.Capability{
		&capability.ASN4{ASN: 65000},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}
	remote := []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}
	negotiated := capability.Negotiate(local, remote, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
	session.negotiated = negotiated
	peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
	peer.setEncodingContexts(negotiated)
	session.egressFinalAccepts = func(filterapi.ExportKind) bool { return true }
	require.Equal(t, NextHopUnchanged, settings.NextHopMode)
	require.Equal(t, "if0", connectedLocalAddress(session).Zone())
	return peer, conn
}

func scopedNextHopUpdate(t *testing.T, linkLocal netip.Addr) (*message.Update, []byte, []byte) {
	t.Helper()
	field := make([]byte, 32)
	global := netip.MustParseAddr("2001:db8::9").As16()
	local := linkLocal.As16()
	copy(field, global[:])
	copy(field[16:], local[:])
	reach := make([]byte, 42)
	copy(reach, []byte{0, 2, 1, 32})
	copy(reach[4:], field)
	copy(reach[37:], []byte{32, 0x20, 1, 0x0d, 0xb8})
	attrs := make([]byte, 58)
	copy(attrs, []byte{0x80, 14, 42})
	copy(attrs[3:], reach)
	copy(attrs[45:], []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe8})
	update := &message.Update{PathAttributes: attrs}
	packet := make([]byte, message.HeaderLen+4+len(attrs))
	n := update.WriteTo(packet, 0, nil)
	require.Equal(t, len(packet), n)
	return update, packet[message.HeaderLen:], field
}

type scopedNextHopConn struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	local  *net.TCPAddr
	remote *net.TCPAddr
	closed bool
}

func (c *scopedNextHopConn) Write(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buffer.Write(data)
}

func (c *scopedNextHopConn) written() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.buffer.Bytes()...)
}

func (c *scopedNextHopConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *scopedNextHopConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *scopedNextHopConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c *scopedNextHopConn) LocalAddr() net.Addr              { return c.local }
func (c *scopedNextHopConn) RemoteAddr() net.Addr             { return c.remote }
func (c *scopedNextHopConn) SetDeadline(time.Time) error      { return nil }
func (c *scopedNextHopConn) SetReadDeadline(time.Time) error  { return nil }
func (c *scopedNextHopConn) SetWriteDeadline(time.Time) error { return nil }
