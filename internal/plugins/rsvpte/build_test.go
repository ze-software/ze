// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- full-message encoder round-trip tests
package rsvpte

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func samplePSB() *pathStateBlock {
	return &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 42, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 7},
		ERO: []eroHop{
			{Loose: false, Address: netip.MustParsePrefix("10.0.0.5/32")},
			{Loose: true, Address: netip.MustParsePrefix("10.0.0.9/32")},
		},
		SenderTSpec:   FlowSpec{TokenRate: 1e9, TokenBucket: 1e9, PeakRate: 1e9, MinPolicedUnit: 20, MaxPacketSize: 1500},
		LabelRequest:  labelRequest{L3PID: 0x0800},
		RefreshPeriod: 30 * time.Second,
	}
}

func TestBuildPathRoundTrip(t *testing.T) {
	psb := samplePSB()
	hop := netip.MustParseAddr("10.0.0.1")

	raw := buildPath(psb, hop, 64)
	msg, err := DecodeMessage(raw)
	require.NoError(t, err)

	assert.Equal(t, MsgTypePath, msg.Header.MsgType)
	assert.Equal(t, uint8(64), msg.Header.TTL)
	assert.Equal(t, int(msg.Header.Length), len(raw), "header length matches buffer")

	require.True(t, msg.HasSession)
	assert.Equal(t, psb.Session, msg.Session)
	require.True(t, msg.HasSenderTemplate)
	assert.Equal(t, psb.SenderTemplate, msg.SenderTemplate)
	require.True(t, msg.HasHop)
	assert.Equal(t, hop, msg.Hop.NextHop)
	// RFC requirement: RFC3209-4.2-1 positive -- buildPath appends a LABEL_REQUEST (build.go:92) so a PATH ze builds carries the object that requests label allocation.
	require.True(t, msg.HasLabelRequest)
	assert.Equal(t, psb.LabelRequest, msg.LabelRequest)
	require.True(t, msg.HasERO)
	require.Len(t, msg.ERO, 2)
	assert.Equal(t, psb.ERO[0].Address, msg.ERO[0].Address)
	assert.Equal(t, psb.ERO[1].Loose, msg.ERO[1].Loose)
	require.True(t, msg.HasSenderTSpec)
	assert.InDelta(t, psb.SenderTSpec.TokenRate, msg.SenderTSpec.TokenRate, 1)
	require.True(t, msg.HasTimeValues)
	assert.Equal(t, uint32(30000), msg.TimeValues.RefreshPeriod)
}

func TestBuildResvRoundTrip(t *testing.T) {
	rsb := &resvStateBlock{
		Session:  sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 42, ExtTunnelID: 0x0a000001},
		FlowSpec: FlowSpec{TokenRate: 5e8, TokenBucket: 5e8, PeakRate: 5e8, MinPolicedUnit: 20, MaxPacketSize: 1500},
		Label:    labelObject{Label: 16001},
		Style:    StyleSharedExplicit,
	}
	filter := senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 7}
	hop := netip.MustParseAddr("10.0.0.5")

	raw := buildResv(rsb, filter, 30*time.Second, hop)
	msg, err := DecodeMessage(raw)
	require.NoError(t, err)

	assert.Equal(t, MsgTypeResv, msg.Header.MsgType)
	require.True(t, msg.HasSession)
	assert.Equal(t, rsb.Session, msg.Session)
	// RFC requirement: RFC3209-4.1-1 positive -- buildResv emits a LABEL object (build.go:129) so a RESV ze builds carries the label the node reports upstream.
	require.True(t, msg.HasLabel)
	assert.Equal(t, uint32(16001), msg.Label.Label)
	// RFC requirement: RFC3209-6-2 positive -- a RESV built with the SE style carries STYLE = Shared Explicit (18) on the wire (build.go:126, wire.go:680), the style make-before-break requires.
	require.True(t, msg.HasStyle)
	assert.Equal(t, StyleSharedExplicit, msg.Style)
	require.True(t, msg.HasSenderTemplate, "filter spec decodes as sender template")
	assert.Equal(t, filter, msg.SenderTemplate)
	require.True(t, msg.HasFlowSpec)
	assert.InDelta(t, rsb.FlowSpec.TokenRate, msg.FlowSpec.TokenRate, 1)
}

func TestBuildPathErrRoundTrip(t *testing.T) {
	session := sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 42}
	sender := senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 7}
	es := errorSpec{
		ErrorNode:  netip.MustParseAddr("10.0.0.5"),
		ErrorCode:  ErrCodeAdmissionControlFailure,
		ErrorValue: ErrValueRequestedBandwidth,
	}
	raw := buildPathErr(session, sender, FlowSpec{}, es, netip.MustParseAddr("10.0.0.5"))
	msg, err := DecodeMessage(raw)
	require.NoError(t, err)

	assert.Equal(t, MsgTypePathErr, msg.Header.MsgType)
	require.True(t, msg.HasErrorSpec)
	assert.Equal(t, ErrCodeAdmissionControlFailure, msg.ErrorSpec.ErrorCode)
	assert.Equal(t, ErrValueRequestedBandwidth, msg.ErrorSpec.ErrorValue)
	assert.Equal(t, es.ErrorNode, msg.ErrorSpec.ErrorNode)
}

func TestBuildPathTearRoundTrip(t *testing.T) {
	psb := samplePSB()
	raw := buildPathTear(psb, netip.MustParseAddr("10.0.0.1"))
	msg, err := DecodeMessage(raw)
	require.NoError(t, err)
	assert.Equal(t, MsgTypePathTear, msg.Header.MsgType)
	require.True(t, msg.HasSession)
	assert.Equal(t, psb.Session, msg.Session)
	require.True(t, msg.HasSenderTemplate)
}

func TestInternetChecksumValid(t *testing.T) {
	// RFC 1071: summing the message including its checksum field yields all-ones
	// (which the verifier folds to zero after complement).
	raw := buildPath(samplePSB(), netip.MustParseAddr("10.0.0.1"), 64)
	assert.NotZero(t, raw[2:4], "checksum field is filled")

	// Recomputing over the message with a correct checksum present must fold to 0.
	got := internetChecksum(raw)
	assert.Equal(t, uint16(0), got, "checksum over message including checksum field is zero")
}
