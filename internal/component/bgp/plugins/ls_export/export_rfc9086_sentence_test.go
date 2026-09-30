// Design: docs/architecture/wire/nlri-bgpls.md -- native origination contract
// RFC: rfc/short/rfc9086.md
// Related: export_rfc9552_polarity_test.go -- TestRFC9086NativePeerSIDReservedFlagsZero
//
// VALIDATES: the originate clause of RFC 9086 Section 5, "Rsvd bits: Reserved
// for future use and MUST be zero when originated and ignored when received",
// on the PeerNode SID TLV 1101 the exporter emits for an EPE peering. The
// receipt half is proven on the consumer (nlri/ls
// TestRFC9086PeerSIDIgnoresReservedFields).
// PREVENTS: a source's reserved flag bits reaching a collector.
package ls_export

import (
	"context"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/epe"
	"github.com/ze-software/ze/internal/core/linkstateevents"
)

// originatedPeerNodeSIDFlags originates one EPE peering whose source PeerNode
// SID carries the given Flags octet and returns the Flags octet of the TLV 1101
// on the emitted Link NLRI UPDATE.
func originatedPeerNodeSIDFlags(t *testing.T, flags byte) byte {
	t.Helper()
	exporter, capture := exportFixture(t)
	local := linkstateevents.NodeID{ASN: 65000, BGPRouterID: netip.MustParseAddr("192.0.2.1")}
	remote := linkstateevents.NodeID{ASN: 65001, BGPRouterID: netip.MustParseAddr("192.0.2.2")}
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.BGP},
		Nodes: []linkstateevents.Node{{ID: local}}, Links: []linkstateevents.Link{{Local: local, Remote: remote,
			Attributes: []linkstateevents.TLV{{Type: 1101, Value: []byte{flags, 5, 0, 0, 0x00, 0x3e, 0x80}}}}}}
	require.NoError(t, exporter.replace(epe.Name, snapshot))
	require.NoError(t, exporter.reconcile(context.Background()))
	for _, command := range capture.commands {
		if !strings.Contains(command, " attr ") {
			continue
		}
		if binary.BigEndian.Uint16(exportCommandBytes(t, command, "nlri")) != 2 {
			continue
		}
		values := exportTLVValues(t, exportCommandBytes(t, command, "attr")[11:], 1101)
		require.Len(t, values, 1)
		return values[0][0]
	}
	t.Fatal("no Link NLRI UPDATE carried a PeerNode SID")
	return 0
}

// TestRFC9086OriginatedPeerSIDRsvdBitsZero is the ordinary case: a source
// PeerNode SID with V and L set and no reserved bit.
//
// RFC requirement: RFC9086-5-5 positive -- a PeerNode SID originated from a source with Flags 0xc0 carries V and L and zero Rsvd bits (the low four bits) on the wire (S5).
func TestRFC9086OriginatedPeerSIDRsvdBitsZero(t *testing.T) {
	require.Equal(t, byte(0xc0), originatedPeerNodeSIDFlags(t, 0xc0))
}

// TestRFC9086SourceRsvdBitsNeverOriginated sets every reserved flag bit in the
// source, the input that reaches the wire if the producer copies the octet.
//
// RFC requirement: RFC9086-5-5 negative -- a source PeerNode SID with all four Rsvd bits set (Flags 0xcf) is originated with those bits zero and V and L intact (S5).
func TestRFC9086SourceRsvdBitsNeverOriginated(t *testing.T) {
	require.Equal(t, byte(0xc0), originatedPeerNodeSIDFlags(t, 0xcf))
}
