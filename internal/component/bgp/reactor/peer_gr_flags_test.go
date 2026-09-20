package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: restartFlagsFor, the decision that puts the RFC 4724 Restart
// State and Forwarding State bits in the OPEN Ze sends.
// PREVENTS: the two failures that face opposite ways. A cold start claiming a
// restart preserved its forwarding state, which RFC 4724 Section 4.1 forbids.
// And a real restart that DID preserve it claiming otherwise, which RFC 4724
// Section 4.2 answers by having the peer "immediately remove all the stale
// routes" for that family, at the moment Ze has re-advertised nothing.

// grCapWithIPv4Unicast is the payload the gr plugin builds for a peer
// carrying one family with the default restart time: 0078 then the
// <AFI 1, SAFI 1, Flags 0> tuple.
func grCapWithIPv4Unicast() []plugin.InjectedCapability {
	return []plugin.InjectedCapability{
		{Code: 64, Value: []byte{0x00, 0x78, 0x00, 0x01, 0x01, 0x00}, Plugin: "gr"},
	}
}

func TestRestartFlagsFor(t *testing.T) {
	tests := []struct {
		name                string
		inRestartWindow     bool
		forwardingPreserved bool
		want                []byte
		why                 string
	}{
		{
			name:            "cold start",
			want:            []byte{0x00, 0x78, 0x00, 0x01, 0x01, 0x00},
			why:             "no restart happened, so neither bit has anything to report",
			inRestartWindow: false,
		},
		{
			// The forwarding plane kept nothing, so the peer is told to drop
			// the stale routes at once rather than hold them over an outage.
			name:            "restart, forwarding flushed",
			inRestartWindow: true,
			want:            []byte{0x80, 0x78, 0x00, 0x01, 0x01, 0x00},
			why:             "R set, F clear",
		},
		{
			name:                "restart, forwarding preserved",
			inRestartWindow:     true,
			forwardingPreserved: true,
			want:                []byte{0x80, 0x78, 0x00, 0x01, 0x01, 0x80},
			why:                 "R set, and F set on the family whose routes survived",
		},
		{
			// The window is what makes the claim about a restart. Outside it
			// there is no restart to have preserved anything during.
			name:                "cold start, forwarding plane would preserve",
			inRestartWindow:     false,
			forwardingPreserved: true,
			want:                []byte{0x00, 0x78, 0x00, 0x01, 0x01, 0x00},
			why:                 "a preserving forwarding plane is not a restart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := restartFlagsFor(grCapWithIPv4Unicast(), tt.inRestartWindow, tt.forwardingPreserved)
			assert.Equal(t, tt.want, got[0].Value, tt.why)
		})
	}
}
