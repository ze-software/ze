// Design: docs/architecture/api/text-format.md -- the OPEN event line
// Related: text.go -- AppendOpen
package format

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: the text OPEN line's `remote as` header names the peer's AS, as
// every other text event does, and the OPEN's own My Autonomous System goes in
// its own `asn` field. Method: a SENT OPEN carries Ze's own AS 65000 to a
// peer of AS 65001; a RECEIVED OPEN from a dynamic peer whose AS is not yet
// negotiated carries the peer's AS 65010 in the message alone.
// PREVENTS: a sent OPEN reading `remote as 65000`, Ze's own AS under the
// header that names the peer, and a received OPEN's AS being lost when the
// header has no negotiated AS to write.
func TestTextOpenRemoteASNamesThePeer(t *testing.T) {
	tests := []struct {
		name      string
		peerAS    rpc.RemoteAS
		openAS    uint32
		direction rpc.MessageDirection
		want      string
	}{
		{
			name:      "sent OPEN carries the local AS",
			peerAS:    rpc.RemoteASOf(65001),
			openAS:    65000,
			direction: rpc.DirectionSent,
			want:      "peer 10.0.0.1 remote as 65001 sent open 7 asn 65000 router-id 1.1.1.1 hold-time 90\n",
		},
		{
			name:      "received OPEN before the peer AS is negotiated",
			peerAS:    rpc.RemoteAS{},
			openAS:    65010,
			direction: rpc.DirectionReceived,
			want:      "peer 10.0.0.1 remote as unknown received open 7 asn 65010 router-id 1.1.1.1 hold-time 90\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peer := plugin.PeerInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: tt.peerAS}
			open := DecodedOpen{ASN: tt.openAS, RouterID: "1.1.1.1", HoldTime: 90}
			got := string(AppendOpen(nil, &peer, open, tt.direction, 7))
			if got != tt.want {
				t.Errorf("AppendOpen:\n got:  %q\n want: %q", got, tt.want)
			}
		})
	}
}
