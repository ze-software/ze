// VALIDATES: spec-ldp-keepalive-time-proposal -- the ldp/keepalive-time leaf an
// operator writes is the KeepAlive Time ze proposes in its Initialization message,
// and the hold time it derives. The chain under test is the delivered config JSON
// -> parseLDPConfig -> sessionConfigForAdj (the builder startSessionForAdj calls)
// -> NewSession -> SendInit -> the Common Session Parameters TLV on the wire.
// PREVENTS: regression to a session built with DefaultKeepaliveTime, which
// proposed 60 seconds whatever the operator configured and held every session at
// 180 seconds, so a peer that proposed more could not be declared dead sooner.
package ldp

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// testAdjacency is a discovered neighbor on 10.0.0.2, as a Hello would leave it.
func testAdjacency() *Adjacency {
	return &Adjacency{
		PeerLSRID:     [4]byte{10, 0, 0, 2},
		TransportAddr: netip.MustParseAddr("10.0.0.2"),
		Interface:     "eth0",
	}
}

// proposedKeepalive returns the KeepAlive Time of the Initialization message sess
// sends, read back off the wire.
func proposedKeepalive(t *testing.T, sess *Session, peer net.Conn) uint16 {
	t.Helper()
	sent := make(chan error, 1)
	go func() { sent <- sess.SendInit() }()

	_, msgHdr, msgBody := readLDPPDU(t, peer)
	require.Equal(t, MsgTypeInitialize, msgHdr.Type, "first message must be an Initialization")
	if err := <-sent; err != nil {
		t.Fatalf("SendInit: %v", err)
	}
	init, err := DecodeInit(msgHdr.MessageID, msgBody)
	require.NoError(t, err)
	return init.KeepaliveTime
}

// TestConfiguredKeepaliveReachesInitializationMessage drives the operator's leaf
// from the JSON the plugin is handed at boot to the bytes of the Common Session
// Parameters TLV. RFC 5036 Section 3.5.3: the KeepAlive Time "indicates the number
// of seconds that the sending LSR proposes for the value of the KeepAlive Time".
func TestConfiguredKeepaliveReachesInitializationMessage(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "ldp",
		Data: `{"ldp":{"keepalive-time":"15","interfaces":"eth0","lsr-id":"10.0.0.1","transport-address":"10.0.0.1"}}`,
	}}
	cfg, err := parseLDPConfig(sections)
	require.NoError(t, err)
	require.Equal(t, 15*time.Second, cfg.KeepaliveTime)

	local, peer := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = peer.Close() }()

	sessCfg := sessionConfigForAdj(cfg.LSRID.As4(), cfg.KeepaliveTime, testAdjacency())
	sess := NewSession(local, sessCfg, newLIB(), slogutil.DiscardLogger())

	assert.Equal(t, uint16(15), proposedKeepalive(t, sess, peer), "ze must propose the configured KeepAlive Time")
	assert.Equal(t, 15*time.Second, sess.currentKeepalive())
	assert.Equal(t, 45*time.Second, sess.currentHoldTime(), "hold time is three times the KeepAlive Time")
}

// TestSessionConfigForAdjCarriesPeerIdentity pins the rest of the session
// parameters the builder fills from the adjacency, so a future leaf added beside
// keepalive cannot displace one of them.
func TestSessionConfigForAdjCarriesPeerIdentity(t *testing.T) {
	adj := testAdjacency()
	got := sessionConfigForAdj([4]byte{10, 0, 0, 1}, 15*time.Second, adj)

	assert.Equal(t, [4]byte{10, 0, 0, 1}, got.LocalLSRID)
	assert.Equal(t, uint16(0), got.LocalLabelSpace, "ze uses the platform-wide label space")
	assert.Equal(t, adj.PeerLSRID, got.PeerLSRID)
	assert.Equal(t, adj.PeerLabelSpace, got.PeerLabelSpace)
	assert.Equal(t, adj.TransportAddr, got.PeerAddr)
	assert.Equal(t, 15*time.Second, got.KeepaliveTime)
}

// TestSessionKeepaliveBoundaries walks the range RFC 5036 Section 3.5.3 can carry:
// "Two octet unsigned non zero integer". One second and 65535 seconds are proposed
// as written; a value under or over that cannot be encoded, so the session
// proposes the default instead of a truncated number the peer would read as
// another interval.
func TestSessionKeepaliveBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		configured time.Duration
		want       uint16
	}{
		{"lowest encodable", time.Second, 1},
		{"highest encodable", keepaliveTimeMax, 65535},
		{"below the range", 0, uint16(DefaultKeepaliveTime.Seconds())},
		{"above the range", keepaliveTimeMax + time.Second, uint16(DefaultKeepaliveTime.Seconds())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local, peer := net.Pipe()
			defer func() { _ = local.Close() }()
			defer func() { _ = peer.Close() }()

			sessCfg := sessionConfigForAdj([4]byte{10, 0, 0, 1}, tt.configured, testAdjacency())
			sess := NewSession(local, sessCfg, newLIB(), slogutil.DiscardLogger())

			assert.Equal(t, tt.want, proposedKeepalive(t, sess, peer))
		})
	}
}
