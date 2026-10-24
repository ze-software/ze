// Design: docs/architecture/api/update-syntax.md -- End-of-RIB on request
// Related: reactor_api_forward.go -- AnnounceEOR, announceEOR

package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnnounceEORSendsEachTimeItIsAsked pins that an explicit End-of-RIB
// request reaches the wire every time, as ExaBGP's announce_eor does.
//
// VALIDATES: two AnnounceEOR calls put two End-of-RIB markers on the wire.
// PREVENTS: the initial-sync claim swallowing a script's or an operator's
// second marker, which left an ExaBGP script's `announce eor` silent.
func TestAnnounceEORSendsEachTimeItIsAsked(t *testing.T) {
	adapter, peer, conn := newEORGuardPeer(t)

	for range 2 {
		require.NoError(t, adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender()))
	}
	eor := eorWire(family.IPv4Unicast)
	assert.Equal(t, bytes.Repeat(eor, 2), conn.written(), "each request puts its own marker on the wire")
	assert.Equal(t, uint32(2), peer.Stats().EORSent)
}

// TestAnnounceEORRefusesAFamilyNoPeerNegotiated pins the answer when no
// selected peer negotiated the family.
//
// RFC 4760 Section 8: "A BGP speaker that uses Multiprotocol Extensions SHOULD
// use the Capability Advertisement procedures [BGP-CAP] to determine whether
// the speaker could use Multiprotocol Extensions with a particular peer."
//
// VALIDATES: an IPv6 unicast End-of-RIB to a peer that negotiated IPv4 unicast
// only writes nothing and answers an error naming the family.
// PREVENTS: a marker for a family the session never carried, and a `done`
// answer for a request that reached no peer.
func TestAnnounceEORRefusesAFamilyNoPeerNegotiated(t *testing.T) {
	adapter, _, conn := newEORGuardPeer(t)

	err := adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv6), uint8(family.SAFIUnicast), plugin.OperatorSender())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ipv6/unicast")
	assert.Empty(t, conn.written(), "no marker for a family the session did not negotiate")
}

// TestAnnounceEORSentOnlyToThePeerCarryingTheFamily pins the other half of
// ExaBGP's rule: with several peers selected, a family one of them negotiated
// is sent to that peer alone and answered done.
//
// ExaBGP's announce_eor (src/exabgp/reactor/api/command/announce.py, pin
// f9bfb853b) keeps the established peers that carry the family, then those
// that may send its End-of-RIB, and answers done once any remain.
//
// RFC 4760 Section 8: "A BGP speaker that uses Multiprotocol Extensions SHOULD
// use the Capability Advertisement procedures [BGP-CAP] to determine whether
// the speaker could use Multiprotocol Extensions with a particular peer."
//
// VALIDATES: `peer *` over a peer with IPv4 unicast alone and a peer with IPv6
// unicast puts the IPv6 marker on the second peer's wire only, with no error.
// PREVENTS: a marker for a family a selected session never carried, and an
// error for a request that did reach a carrying peer.
func TestAnnounceEORSentOnlyToThePeerCarryingTheFamily(t *testing.T) {
	ipv4Only, ipv4Conn := newEstablishedRecordingPeer(t, "10.0.0.2", family.IPv4Unicast)
	carrying, carryingConn := newEstablishedRecordingPeer(t, "10.0.0.3", family.IPv4Unicast, family.IPv6Unicast)
	adapter := &reactorAPIAdapter{r: &Reactor{
		config: &Config{LocalAS: 65000},
		peers: map[netip.AddrPort]*Peer{
			ipv4Only.settings.PeerKey(): ipv4Only,
			carrying.settings.PeerKey(): carrying,
		},
	}}

	err := adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv6), uint8(family.SAFIUnicast), plugin.OperatorSender())
	require.NoError(t, err, "one selected peer carries the family, so the request is done")
	assert.Equal(t, eorWire(family.IPv6Unicast), carryingConn.written(), "the carrying peer gets the marker")
	assert.Empty(t, ipv4Conn.written(), "no marker for a session that did not negotiate the family")
	assert.Equal(t, uint32(1), carrying.Stats().EORSent)
	assert.Equal(t, uint32(0), ipv4Only.Stats().EORSent)
}
