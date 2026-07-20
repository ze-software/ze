// RFC: rfc/short/rfc6793.md — four-octet AS capability advertisement and AS_TRANS in OPEN
//
// Drives sendOpen (session_negotiate.go) through a real Accept over net.Pipe and
// inspects the OPEN this speaker puts on the wire.

package reactor

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/capability"
)

// rfc6793SentOpen starts a passive session with the given local AS, accepts a
// net.Pipe connection, and returns the OPEN message sendOpen wrote.
func rfc6793SentOpen(t *testing.T, localAS uint32, disableASN4 bool) *message.Open {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), localAS, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.DisableASN4 = disableASN4

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	raw := acceptWithReader(t, session, server, client)
	require.Greater(t, len(raw), message.HeaderLen, "OPEN must have been sent")

	open, err := message.UnpackOpen(raw[message.HeaderLen:])
	require.NoError(t, err)
	return open
}

// rfc6793ASN4Cap returns the ASN4 capability from an OPEN, or nil.
func rfc6793ASN4Cap(t *testing.T, open *message.Open) *capability.ASN4 {
	t.Helper()
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams)
	require.NoError(t, err)
	for _, c := range caps {
		if a, ok := c.(*capability.ASN4); ok {
			return a
		}
	}
	return nil
}

// TestRFC6793OpenAdvertisesFourOctetASCapability proves this speaker advertises
// its support for four-octet AS numbers through a BGP Capabilities Advertisement
// (sendOpen, session_negotiate.go).
//
// RFC requirement: RFC6793-4.1-1 positive -- the OPEN this speaker sends carries the
// four-octet AS number capability (code 65) as a Capabilities Optional Parameter, and its
// value is the speaker's own AS.
func TestRFC6793OpenAdvertisesFourOctetASCapability(t *testing.T) {
	open := rfc6793SentOpen(t, 4200000001, false)
	c := rfc6793ASN4Cap(t, open)
	require.NotNil(t, c, "OPEN must advertise the four-octet AS number capability")
	assert.Equal(t, capability.Code(65), c.Code())
	assert.Equal(t, uint32(4200000001), c.ASN)
}

// TestRFC6793OpenOmitsCapabilityWhenDisabled proves the advertisement is produced
// by the ASN4 branch of sendOpen and not by fixed OPEN bytes: with the
// capability turned off in configuration the OPEN carries no code 65.
//
// RFC requirement: RFC6793-4.1-1 negative -- an OPEN built with the four-octet AS support
// disabled carries no code-65 capability, so the advertisement in the positive case is the
// speaker declaring support rather than an unconditional constant.
func TestRFC6793OpenOmitsCapabilityWhenDisabled(t *testing.T) {
	open := rfc6793SentOpen(t, 65001, true)
	assert.Nil(t, rfc6793ASN4Cap(t, open),
		"no four-octet AS capability when support is not advertised")
}

// TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS proves the OPEN header carries
// AS_TRANS when the speaker's AS cannot be represented in the two-octet
// "My Autonomous System" field (sendOpen, session_negotiate.go).
//
// RFC requirement: RFC6793-4.2.1-1 positive -- a speaker whose AS is non-mappable has no
// two-octet AS number, so the OPEN "My Autonomous System" field carries AS_TRANS (23456)
// while the real AS travels in the capability value.
func TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS(t *testing.T) {
	open := rfc6793SentOpen(t, 4200000001, false)
	assert.Equal(t, uint16(message.AS_TRANS), open.MyAS)
	assert.Equal(t, uint16(23456), open.MyAS)

	c := rfc6793ASN4Cap(t, open)
	require.NotNil(t, c)
	assert.Equal(t, uint32(4200000001), c.ASN, "the real AS is only in the capability")
}

// TestRFC6793OpenMyASIsRealASWhenMappable proves the AS_TRANS substitution is
// scoped to speakers without a two-octet AS number.
//
// RFC requirement: RFC6793-4.2.1-1 negative -- a speaker whose AS does fit in two octets puts
// that AS in the OPEN "My Autonomous System" field, never AS_TRANS, so the placeholder is not
// written unconditionally.
func TestRFC6793OpenMyASIsRealASWhenMappable(t *testing.T) {
	open := rfc6793SentOpen(t, 65001, false)
	assert.Equal(t, uint16(65001), open.MyAS)
	assert.NotEqual(t, uint16(message.AS_TRANS), open.MyAS)
}
