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
	// routing loop cannot grow it past what the message buffer can encode: an
	// over-limit route is dropped whole (RFC 3209 Section 4.4.3) and the drop is
	// reported so callers can surface it (not silent).
	down := make([]rroEntry, maxRecordRouteHops+10)
	for i := range down {
		down[i] = rroEntry{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.2")}
	}
	out, dropped := prependRRO(netip.MustParseAddr("10.0.0.1"), down, 0)
	assert.Empty(t, out, "an over-limit recorded route is dropped whole, never truncated")
	assert.True(t, dropped, "an over-limit recorded route must report the drop, not lose the RRO silently")

	// A route within the limit is kept and not flagged as dropped.
	short := []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.2")}}
	kept, shortDropped := prependRRO(netip.MustParseAddr("10.0.0.1"), short, 0)
	assert.Len(t, kept, 2)
	assert.False(t, shortDropped, "a short recorded route is not dropped")
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
	msg, err := DecodeMessage(raw)
	require.NoError(t, err, "a RESV without the overlong RRO must still decode")
	require.Len(t, msg.FlowDescriptors, 1)
	require.Len(t, msg.FlowDescriptors[0].Filters, 1)
	assert.False(t, msg.FlowDescriptors[0].Filters[0].HasRRO, "the encoder must drop the entire oversized RRO")
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
