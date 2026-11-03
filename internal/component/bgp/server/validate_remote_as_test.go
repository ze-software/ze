// Design: docs/architecture/behavior/fsm-open-sent.md — OPEN acceptance
// Overview: validate.go — remoteOpenAS, the peer AS an OPEN validator plugin is told
// RFC: rfc/short/rfc6793.md — the Four-octet AS capability replaces My AS only between NEW speakers

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestRemoteOpenASFollowsTheSessionRule drives remoteOpenAS over the OPENs a four-octet
// peer and a two-octet peer send, with ze's own OPEN carrying the Four-octet AS
// capability and not carrying it.
//
// VALIDATES: RFC 6793 Section 4.1 reads the capability value "in lieu of" My AS only
// between NEW speakers, so the plugin is told 4200000000 when both OPENs carry the
// capability, AS_TRANS when ze's does not, and My AS for a peer that sends none.
// PREVENTS: the never-set Open.ASN4 read, which told every OPEN validator a received
// four-octet peer was AS 23456 (UnpackOpen leaves the field zero, as it is here).
func TestRemoteOpenASFollowsTheSessionRule(t *testing.T) {
	// Optional parameter type 2 (Capabilities), length 6, capability 65, length 4.
	asn4Cap := func(asn uint32) []byte {
		return []byte{2, 6, 65, 4, byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn)}
	}
	localWithASN4 := &message.Open{Version: 4, MyAS: 65000, OptionalParams: asn4Cap(65000)}
	localWithout := &message.Open{Version: 4, MyAS: 65000}
	remoteFourOctet := &message.Open{Version: 4, MyAS: message.AS_TRANS, OptionalParams: asn4Cap(4200000000)}
	remoteTwoOctet := &message.Open{Version: 4, MyAS: 65001}

	tests := []struct {
		name   string
		local  *message.Open
		remote *message.Open
		want   uint32
	}{
		{"both NEW: the capability value", localWithASN4, remoteFourOctet, 4200000000},
		{"ze is OLD: My AS", localWithout, remoteFourOctet, uint32(message.AS_TRANS)},
		{"the peer is OLD: My AS", localWithASN4, remoteTwoOctet, 65001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := remoteOpenAS(tt.local, tt.remote)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	malformed := &message.Open{Version: 4, MyAS: 65001, OptionalParams: []byte{0xff, 0xff, 0xff}}
	_, err := remoteOpenAS(localWithASN4, malformed)
	require.Error(t, err, "a capability list that does not parse is refused, never answered as My AS")
}
