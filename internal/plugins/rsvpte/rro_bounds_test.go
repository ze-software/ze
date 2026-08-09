// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RRO bounding (F9) tests
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrependRROCapsDepth(t *testing.T) {
	// VALIDATES: F9 -- prependRRO bounds the recorded route so a long path or a
	// routing loop cannot grow it past what the message buffer can encode, and
	// reports the truncation so callers can surface it (not silent).
	down := make([]rroEntry, maxRecordRouteHops+10)
	for i := range down {
		down[i] = rroEntry{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.2")}
	}
	out, truncated := prependRRO(netip.MustParseAddr("10.0.0.1"), down)
	assert.LessOrEqual(t, len(out), maxRecordRouteHops)
	assert.True(t, truncated, "an over-limit recorded route must report truncation, not drop hops silently")

	// A route within the limit is not flagged as truncated.
	short := []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.2")}}
	_, shortTrunc := prependRRO(netip.MustParseAddr("10.0.0.1"), short)
	assert.False(t, shortTrunc, "a short recorded route is not truncated")
}

func TestBuildResvOverlongRRODoesNotOverflow(t *testing.T) {
	// VALIDATES: F9 -- encoding a RESV whose RRO exceeds the buffer stays within
	// maxRSVPMessage (no out-of-range write) and still decodes.
	rro := make([]rroEntry, 500)
	for i := range rro {
		rro[i] = rroEntry{Type: RROSubIPv6, Address: netip.MustParseAddr("2001:db8::1")}
	}
	rsb := &resvStateBlock{
		Session: sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1},
		Label:   labelObject{Label: 16000},
		RRO:     rro,
	}
	filter := senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1}
	raw := buildResv(rsb, filter, DefaultRefreshPeriod, netip.MustParseAddr("10.0.0.5"))
	assert.LessOrEqual(t, len(raw), maxRSVPMessage, "encoded RESV must fit the fixed message buffer")
	_, err := DecodeMessage(raw)
	require.NoError(t, err, "truncated-RRO RESV must still decode")
}

func TestDecodeRROCapsEntries(t *testing.T) {
	// VALIDATES: F9 -- DecodeRRO bounds the number of entries it returns.
	body := make([]byte, 0, (maxRecordRouteHops+50)*8)
	for range maxRecordRouteHops + 50 {
		// One IPv4 RRO subobject: type, len=8, 4-byte addr, prefix-len, flags.
		body = append(body, RROSubIPv4, 8, 10, 0, 0, 1, 32, 0)
	}
	entries, err := decodeRRO(body)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(entries), maxRecordRouteHops)
}
