package analyze

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

func TestBuildUpdateBody(t *testing.T) {
	prefix := []byte{10, 0, 0}
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP

	entry := &mrt.RIBEntry{Attributes: attrs}
	body := buildUpdateBody(24, prefix, entry, true)
	require.True(t, len(body) > 4)

	// Withdrawn length = 0
	wdLen := binary.BigEndian.Uint16(body[0:2])
	assert.Equal(t, uint16(0), wdLen)

	// Attribute length
	attrLen := binary.BigEndian.Uint16(body[2:4])
	assert.Equal(t, uint16(4), attrLen)

	// Attributes
	assert.Equal(t, attrs, body[4:8])

	// NLRI: prefix-len + prefix bytes
	assert.Equal(t, byte(24), body[8])
	assert.Equal(t, prefix, body[9:12])
}

func TestServeBGPOpen_Success(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Simulate peer sending OPEN then KEEPALIVE
	go func() {
		openBody := bgpBuildOpen(65001, 90, net.IP{1, 0, 0, 1})
		if err := bgpWrite(client, 1, openBody); err != nil {
			return
		}
		// Read server's OPEN
		_, _, _ = bgpReadMsg(client)
		// Read server's KEEPALIVE
		_, _, _ = bgpReadMsg(client)
		// Send our KEEPALIVE
		_ = bgpWrite(client, 4, nil)
	}()

	peerAS, err := serveBGPOpen(server, 65000, net.IP{2, 0, 0, 1}, 90)
	require.NoError(t, err)
	assert.Equal(t, uint32(65001), peerAS)
}

func TestServeBGPOpen_NotOpen(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Simulate peer sending NOTIFICATION instead of OPEN
	go func() {
		_ = bgpWrite(client, 3, []byte{6, 1}) // NOTIFICATION
	}()

	_, err := serveBGPOpen(server, 65000, net.IP{2, 0, 0, 1}, 90)
	assert.Error(t, err)
}

func TestServeBGPOpen_NotKeepalive(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	go func() {
		openBody := bgpBuildOpen(65002, 90, net.IP{3, 0, 0, 1})
		_ = bgpWrite(client, 1, openBody)
		_, _, _ = bgpReadMsg(client) // read OPEN
		_, _, _ = bgpReadMsg(client) // read KEEPALIVE
		// Send NOTIFICATION instead of KEEPALIVE
		_ = bgpWrite(client, 3, []byte{6, 2})
	}()

	_, err := serveBGPOpen(server, 65000, net.IP{2, 0, 0, 1}, 90)
	assert.Error(t, err)
}
