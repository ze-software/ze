package peer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendRouteASPathIsWhatARealSpeakerSends checks the AS_PATH attribute
// send-route builds, for each session kind and each way a .ci states the path.
//
// VALIDATES: RFC 4271 Section 5.1.2, an eBGP speaker prepends its own AS, so
// on eBGP the path opens with ze-peer's AS; iBGP keeps origin-as alone; an
// explicit as-path goes out as written.
// PREVENTS: an empty or origin-only path from an eBGP ze-peer, which ze
// rightly drops under the RFC 4271 Section 6.3 leftmost-AS check.
func TestSendRouteASPathIsWhatARealSpeakerSends(t *testing.T) {
	tests := []struct {
		name  string
		route RouteToSend
		want  []byte
	}{
		{
			name:  "ibgp without origin-as sends an empty path",
			route: RouteToSend{},
			want:  []byte{0x40, 0x02, 0x00},
		},
		{
			name:  "ibgp with origin-as sends the origin alone",
			route: RouteToSend{OriginAS: 65001},
			want:  []byte{0x40, 0x02, 0x06, asSequence, 1, 0, 0, 0xfd, 0xe9},
		},
		{
			name:  "ebgp without origin-as sends the sender alone",
			route: RouteToSend{SenderAS: 65002},
			want:  []byte{0x40, 0x02, 0x06, asSequence, 1, 0, 0, 0xfd, 0xea},
		},
		{
			name:  "ebgp with a different origin sends sender then origin",
			route: RouteToSend{SenderAS: 65002, OriginAS: 65001},
			want:  []byte{0x40, 0x02, 0x0a, asSequence, 2, 0, 0, 0xfd, 0xea, 0, 0, 0xfd, 0xe9},
		},
		{
			name:  "ebgp whose origin is the sender sends it once",
			route: RouteToSend{SenderAS: 65001, OriginAS: 65001},
			want:  []byte{0x40, 0x02, 0x06, asSequence, 1, 0, 0, 0xfd, 0xe9},
		},
		{
			name:  "ebgp as-set puts the origin in a set after the sender",
			route: RouteToSend{SenderAS: 65002, OriginAS: 65001, ASSet: true},
			want:  []byte{0x40, 0x02, 0x0c, asSequence, 1, 0, 0, 0xfd, 0xea, asSet, 1, 0, 0, 0xfd, 0xe9},
		},
		{
			name:  "an explicit as-path goes out as written",
			route: RouteToSend{SenderAS: 65002, OriginAS: 65003, ASPath: []uint32{65001, 65003}},
			want:  []byte{0x40, 0x02, 0x0a, asSequence, 2, 0, 0, 0xfd, 0xe9, 0, 0, 0xfd, 0xeb},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, asPathAttr(tt.route))
		})
	}
}

// TestSendRouteSenderASFollowsTheSessionKind checks which AS ze-peer prepends.
//
// VALIDATES: ze-peer prepends its OPEN AS when that differs from ze's, and
// nothing when the two match.
// PREVENTS: a prepend on iBGP, where RFC 4271 Section 5.1.2 adds no AS.
func TestSendRouteSenderASFollowsTheSessionKind(t *testing.T) {
	ze := zeOpenBody(65000, 0x01020304, asn4TLV(65000))

	mirrored := &Peer{config: &Config{}}
	assert.Equal(t, uint32(0), mirrored.ebgpSenderAS(ze, nil), "a mirrored AS is iBGP")

	external := &Peer{config: &Config{OpenAS: []OpenASBinding{{AS: 65001}}}}
	require.Equal(t, uint32(65001), external.ebgpSenderAS(ze, nil), "a declared AS apart from ze's is eBGP")
}

// TestSendDefaultRouteKeepsItsIBGPBytes checks send-default-route on both
// session kinds.
//
// VALIDATES: on iBGP the UPDATE is byte-identical to the frame ze-peer always
// sent; on eBGP the only change is the sender's AS in the AS_PATH.
// PREVENTS: an empty AS_PATH from an eBGP ze-peer, which ze drops under the
// RFC 4271 Section 6.3 leftmost-AS check.
func TestSendDefaultRouteKeepsItsIBGPBytes(t *testing.T) {
	ibgp := []byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0x31, 0x02,
		0x00, 0x00, 0x00, 0x15,
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x00, // AS_PATH empty
		0x40, 0x03, 0x04, 0x7F, 0x00, 0x00, 0x01, // NEXT_HOP 127.0.0.1
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64, // LOCAL_PREF 100
		0x20, 0x00, 0x00, 0x00, 0x00, // NLRI 0.0.0.0/32
	}
	msg, err := BuildRouteMsg(defaultRoute(0))
	require.NoError(t, err)
	assert.Equal(t, ibgp, msg)

	ebgp := []byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0x37, 0x02,
		0x00, 0x00, 0x00, 0x1B,
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x06, asSequence, 1, 0, 0, 0xfd, 0xe9, // AS_PATH [65001]
		0x40, 0x03, 0x04, 0x7F, 0x00, 0x00, 0x01,
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
		0x20, 0x00, 0x00, 0x00, 0x00,
	}
	msg, err = BuildRouteMsg(defaultRoute(65001))
	require.NoError(t, err)
	assert.Equal(t, ebgp, msg)
}
