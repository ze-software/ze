// Design: docs/architecture/api/update-syntax.md -- End-of-RIB on request
// Related: reactor_api_forward.go -- AnnounceEOR, announceEOR

package reactor

import (
	"bytes"
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
