package mrt_test

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

func buildBGPMessage(msgType byte, body []byte) []byte {
	totalLen := 19 + len(body)
	msg := make([]byte, totalLen)
	for i := range 16 {
		msg[i] = 0xff
	}
	binary.BigEndian.PutUint16(msg[16:], uint16(totalLen))
	msg[18] = msgType
	copy(msg[19:], body)
	return msg
}

func TestParseBGPMessage_Keepalive(t *testing.T) {
	msg := buildBGPMessage(4, nil)
	parsed, err := mrt.ParseBGPMessage(msg)
	require.NoError(t, err)
	assert.Equal(t, uint8(4), parsed.Type)
	assert.Nil(t, parsed.Open)
	assert.Nil(t, parsed.Update)
	assert.Nil(t, parsed.Notification)
}

func TestParseBGPMessage_Open(t *testing.T) {
	body := make([]byte, 10)
	body[0] = 4   // version
	body[1] = 0   // ASN high
	body[2] = 100 // ASN low (AS100)
	body[3] = 0   // hold time high
	body[4] = 90  // hold time low
	body[5] = 1   // router ID
	body[6] = 2
	body[7] = 3
	body[8] = 4
	body[9] = 0 // opt params length

	msg := buildBGPMessage(1, body)
	parsed, err := mrt.ParseBGPMessage(msg)
	require.NoError(t, err)
	require.NotNil(t, parsed.Open)
	assert.Equal(t, uint8(4), parsed.Open.Version)
	assert.Equal(t, uint32(100), parsed.Open.ASN)
	assert.Equal(t, uint16(90), parsed.Open.HoldTime)
	assert.Equal(t, [4]byte{1, 2, 3, 4}, parsed.Open.RouterID)
}

func TestParseBGPMessage_OpenWith4ByteAS(t *testing.T) {
	cap4byte := []byte{65, 4, 0, 1, 0, 0} // capability 65, len 4, AS 65536
	optParam := []byte{2, byte(len(cap4byte))}
	optParam = append(optParam, cap4byte...)

	body := make([]byte, 10+len(optParam))
	body[0] = 4
	binary.BigEndian.PutUint16(body[1:], 23456) // AS_TRANS
	binary.BigEndian.PutUint16(body[3:], 180)
	body[5] = 10
	body[6] = 20
	body[7] = 30
	body[8] = 40
	body[9] = byte(len(optParam))
	copy(body[10:], optParam)

	msg := buildBGPMessage(1, body)
	parsed, err := mrt.ParseBGPMessage(msg)
	require.NoError(t, err)
	require.NotNil(t, parsed.Open)
	assert.Equal(t, uint32(65536), parsed.Open.ASN)
	assert.Equal(t, uint16(180), parsed.Open.HoldTime)
}

func TestParseBGPMessage_Update(t *testing.T) {
	// UPDATE: withdrawn=0, attrs=[ORIGIN IGP], NLRI=10.0.0.0/8
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP

	body := make([]byte, 2+2+len(attrs)+2)
	off := 0
	binary.BigEndian.PutUint16(body[off:], 0) // withdrawn length
	off += 2
	binary.BigEndian.PutUint16(body[off:], uint16(len(attrs)))
	off += 2
	copy(body[off:], attrs)
	off += len(attrs)
	body[off] = 8    // /8
	body[off+1] = 10 // 10.x.x.x

	msg := buildBGPMessage(2, body)
	parsed, err := mrt.ParseBGPMessage(msg)
	require.NoError(t, err)
	require.NotNil(t, parsed.Update)
	assert.Empty(t, parsed.Update.WithdrawnPrefixes)
	assert.Len(t, parsed.Update.Attributes, 1)
	assert.Equal(t, uint8(1), parsed.Update.Attributes[0].Code)
	assert.Equal(t, []byte{0x00}, parsed.Update.Attributes[0].Value)
	require.Len(t, parsed.Update.AnnouncedPrefixes, 1)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), parsed.Update.AnnouncedPrefixes[0])
}

func TestParseBGPMessage_UpdateWithWithdrawn(t *testing.T) {
	// Withdraw 192.168.1.0/24
	withdrawn := []byte{24, 192, 168, 1}

	body := make([]byte, 2+len(withdrawn)+2)
	binary.BigEndian.PutUint16(body[0:], uint16(len(withdrawn)))
	copy(body[2:], withdrawn)
	binary.BigEndian.PutUint16(body[2+len(withdrawn):], 0) // attr length

	msg := buildBGPMessage(2, body)
	parsed, err := mrt.ParseBGPMessage(msg)
	require.NoError(t, err)
	require.NotNil(t, parsed.Update)
	require.Len(t, parsed.Update.WithdrawnPrefixes, 1)
	assert.Equal(t, netip.MustParsePrefix("192.168.1.0/24"), parsed.Update.WithdrawnPrefixes[0])
}

func TestParseBGPMessage_Notification(t *testing.T) {
	body := []byte{6, 2, 0xDE, 0xAD} // Cease, Administrative Shutdown, data
	msg := buildBGPMessage(3, body)
	parsed, err := mrt.ParseBGPMessage(msg)
	require.NoError(t, err)
	require.NotNil(t, parsed.Notification)
	assert.Equal(t, uint8(6), parsed.Notification.Code)
	assert.Equal(t, uint8(2), parsed.Notification.Subcode)
	assert.Equal(t, []byte{0xDE, 0xAD}, parsed.Notification.Data)
}

func TestParseBGPMessage_BadMarker(t *testing.T) {
	msg := make([]byte, 19)
	msg[0] = 0x00 // bad marker
	binary.BigEndian.PutUint16(msg[16:], 19)
	msg[18] = 4
	_, err := mrt.ParseBGPMessage(msg)
	assert.Error(t, err)
}

func TestParseBGPMessage_TooShort(t *testing.T) {
	_, err := mrt.ParseBGPMessage([]byte{0xff, 0xff})
	assert.Error(t, err)
}

func TestParseASPath_4Byte(t *testing.T) {
	// AS_SEQUENCE with 3 ASNs: 65000, 65001, 65002
	data := []byte{
		2, 3, // type=sequence, count=3
		0, 0, 0xFD, 0xE8, // 65000
		0, 0, 0xFD, 0xE9, // 65001
		0, 0, 0xFD, 0xEA, // 65002
	}
	segs, err := mrt.ParseASPath(data, true)
	require.NoError(t, err)
	require.Len(t, segs, 1)
	assert.Equal(t, uint8(2), segs[0].Type)
	assert.Equal(t, []uint32{65000, 65001, 65002}, segs[0].ASNs)
}

func TestParseASPath_2Byte(t *testing.T) {
	data := []byte{
		2, 2, // sequence, 2 ASNs
		0xFD, 0xE8, // 65000
		0xFD, 0xE9, // 65001
	}
	segs, err := mrt.ParseASPath(data, false)
	require.NoError(t, err)
	require.Len(t, segs, 1)
	assert.Equal(t, []uint32{65000, 65001}, segs[0].ASNs)
}

func TestParseASPath_MultipleSegments(t *testing.T) {
	data := []byte{
		2, 2, 0, 0, 0, 1, 0, 0, 0, 2, // sequence: 1, 2
		1, 2, 0, 0, 0, 3, 0, 0, 0, 4, // set: 3, 4
	}
	segs, err := mrt.ParseASPath(data, true)
	require.NoError(t, err)
	require.Len(t, segs, 2)
	assert.Equal(t, uint8(2), segs[0].Type)
	assert.Equal(t, uint8(1), segs[1].Type)
}

func TestParseAttributes(t *testing.T) {
	// ORIGIN(1)=IGP + LOCAL_PREF(5)=100
	data := []byte{
		0x40, 1, 1, 0x00, // ORIGIN IGP
		0x40, 5, 4, 0, 0, 0, 100, // LOCAL_PREF 100
	}
	attrs := mrt.ParseAttributes(data)
	require.Len(t, attrs, 2)
	assert.Equal(t, uint8(1), attrs[0].Code)
	assert.Equal(t, []byte{0x00}, attrs[0].Value)
	assert.Equal(t, uint8(5), attrs[1].Code)
	assert.Equal(t, []byte{0, 0, 0, 100}, attrs[1].Value)
}

func TestExtractNextHop(t *testing.T) {
	attrs := []mrt.PathAttribute{
		{Code: 3, Value: []byte{10, 0, 0, 1}},
	}
	nh := mrt.ExtractNextHop(attrs)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), nh)
}

func TestExtractNextHop_MPReach(t *testing.T) {
	// AFI=2, SAFI=1, NH_len=16, NH=2001:db8::1
	mpReach := make([]byte, 4+16+1)
	binary.BigEndian.PutUint16(mpReach[0:], 2) // AFI IPv6
	mpReach[2] = 1                             // SAFI unicast
	mpReach[3] = 16                            // NH length
	nh := netip.MustParseAddr("2001:db8::1").As16()
	copy(mpReach[4:20], nh[:])

	attrs := []mrt.PathAttribute{
		{Code: 14, Value: mpReach},
	}
	got := mrt.ExtractNextHop(attrs)
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), got)
}

func TestExtractOrigin(t *testing.T) {
	attrs := []mrt.PathAttribute{{Code: 1, Value: []byte{2}}}
	v, ok := mrt.ExtractOrigin(attrs)
	assert.True(t, ok)
	assert.Equal(t, uint8(2), v)
}

func TestExtractCommunities(t *testing.T) {
	// Two communities: 65000:100, 65000:200
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data[0:], 65000<<16|100)
	binary.BigEndian.PutUint32(data[4:], 65000<<16|200)

	attrs := []mrt.PathAttribute{{Code: 8, Value: data}}
	comms := mrt.ExtractCommunities(attrs)
	require.Len(t, comms, 2)
	assert.Equal(t, uint32(65000<<16|100), comms[0])
	assert.Equal(t, uint32(65000<<16|200), comms[1])
}

func TestParsePrefixes(t *testing.T) {
	// 10.0.0.0/8, 192.168.0.0/16
	data := []byte{8, 10, 16, 192, 168}
	pfxs := mrt.ParsePrefixes(data, false)
	require.Len(t, pfxs, 2)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), pfxs[0])
	assert.Equal(t, netip.MustParsePrefix("192.168.0.0/16"), pfxs[1])
}

func TestFindAttribute(t *testing.T) {
	attrs := []mrt.PathAttribute{
		{Code: 1, Value: []byte{0}},
		{Code: 2, Value: []byte{1, 2, 3}},
	}
	assert.NotNil(t, mrt.FindAttribute(attrs, 1))
	assert.NotNil(t, mrt.FindAttribute(attrs, 2))
	assert.Nil(t, mrt.FindAttribute(attrs, 99))
}
