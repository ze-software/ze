package analyze

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

func TestBuildBMPRouteMonitoring_IPv4(t *testing.T) {
	bgpMsg := make([]byte, 19)
	for i := range 16 {
		bgpMsg[i] = 0xff
	}
	binary.BigEndian.PutUint16(bgpMsg[16:], 19)
	bgpMsg[18] = 4

	m := &mrt.MessageRecord{
		BGP4MPHeader: mrt.BGP4MPHeader{
			PeerAS: 65001,
			AFI:    mrt.AFIIPv4,
			PeerIP: []byte{10, 0, 0, 1},
		},
		BGPMessage: bgpMsg,
	}

	buf := buildBMPRouteMonitoring(1700000000, 500, m)

	require.True(t, len(buf) >= BMPCommonHdrLen+BMPPeerHdrLen+19)
	assert.Equal(t, byte(3), buf[0])
	totalLen := binary.BigEndian.Uint32(buf[1:5])
	assert.Equal(t, uint32(len(buf)), totalLen)
	assert.Equal(t, byte(0), buf[5]) // Route Monitoring

	off := BMPCommonHdrLen
	assert.Equal(t, byte(0), buf[off])   // peer type global
	assert.Equal(t, byte(0), buf[off+1]) // flags: IPv4
	// IPv4-mapped address
	assert.Equal(t, byte(0xff), buf[off+20])
	assert.Equal(t, byte(0xff), buf[off+21])
	assert.Equal(t, byte(10), buf[off+22])

	peerAS := binary.BigEndian.Uint32(buf[off+26 : off+30])
	assert.Equal(t, uint32(65001), peerAS)

	ts := binary.BigEndian.Uint32(buf[off+34 : off+38])
	assert.Equal(t, uint32(1700000000), ts)
	usec := binary.BigEndian.Uint32(buf[off+38 : off+42])
	assert.Equal(t, uint32(500), usec)
}

func TestBuildBMPRouteMonitoring_IPv6(t *testing.T) {
	bgpMsg := make([]byte, 19)
	for i := range 16 {
		bgpMsg[i] = 0xff
	}
	binary.BigEndian.PutUint16(bgpMsg[16:], 19)
	bgpMsg[18] = 4

	peerIP := make([]byte, 16)
	peerIP[0] = 0x20
	peerIP[1] = 0x01

	m := &mrt.MessageRecord{
		BGP4MPHeader: mrt.BGP4MPHeader{
			PeerAS: 65002,
			AFI:    mrt.AFIIPv6,
			PeerIP: peerIP,
		},
		BGPMessage: bgpMsg,
	}

	buf := buildBMPRouteMonitoring(1700000001, 0, m)

	off := BMPCommonHdrLen
	assert.Equal(t, byte(0x80), buf[off+1]) // flags: IPv6
	assert.Equal(t, byte(0x20), buf[off+10])
	assert.Equal(t, byte(0x01), buf[off+11])
}
