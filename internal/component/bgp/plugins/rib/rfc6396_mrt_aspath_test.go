// Design: docs/architecture/mrt.md -- RIB snapshot bridge for MRT TABLE_DUMP_V2
// RFC: rfc/short/rfc6396.md -- Section 4.3.4, RIB entries carry 4-byte AS numbers
// Related: rib_mrt.go -- dumpRIBForMRT, the bridge this drives

// VALIDATES: a route received on a session with no 4-byte AS support reaches
// a written TABLE_DUMP_V2 RIB entry with its AS_PATH in 4-byte encoding, and a
// route from a 4-byte session reaches it unchanged.
// PREVENTS: an MRT RIB entry carrying the 2-byte AS_PATH the peer sent, which
// every reader parses as 4-byte and so decodes into wrong AS numbers.

package rib

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// rfc6396DumpedASPath receives body on a session whose 4-byte AS support is
// asn4, through the ingest reconciliation the session read path applies
// (wireu.CollapseAS4Family) and the RIB's received-UPDATE entry point, then
// takes the RIB's MRT snapshot, writes the one route as a TABLE_DUMP_V2
// RIB_IPV4_UNICAST record with the mrt library writers the MRT plugin uses,
// decodes that record, and returns the AS_PATH value of its RIB entry.
func rfc6396DumpedASPath(t *testing.T, body []byte, asn4 bool) []byte {
	t.Helper()
	collapsed := make([]byte, wireu.CollapseAS4FamilySize(body))
	n, _, err := wireu.CollapseAS4Family(collapsed, body, asn4)
	require.NoError(t, err)
	if n == 0 {
		// Already canonical: the session read path keeps the payload it has.
		n = copy(collapsed, body)
	}

	r := newTestRIBManager(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(asn4))
	feedReceivedWithIdentifier(t, r, netip.MustParseAddr("192.0.2.1"), identifierFor(t, "192.0.2.1"), ctxID, collapsed[:n])

	record := make([]byte, 0, 512)
	routes := 0
	r.dumpRIBForMRT(registry.RIBDumpVisitor{
		OnPeer: func(string, uint32, [4]byte, bool) uint16 { return 0 },
		OnRoute: func(peerIndex, _, _ uint16, prefixLen uint8, prefix, attrs []byte) {
			routes++
			buf := make([]byte, 512)
			off := mrtfmt.WriteRIBHeader(buf, 0, 1, prefixLen, prefix)
			off += mrtfmt.WriteRIBEntries(buf, off, []mrtfmt.RIBEntry{{PeerIndex: peerIndex, Attributes: attrs}}, false)
			record = append(record, buf[:off]...)
		},
	})
	require.Equal(t, 1, routes, "the RIB must hold the received route")

	rib, err := mrtfmt.DecodeRIBRecord(mrtfmt.TDV2RIBIPv4Unicast, record)
	require.NoError(t, err)
	require.Len(t, rib.Entries, 1)
	attrs := rib.Entries[0].Attributes
	for pos := 0; pos+3 <= len(attrs); {
		flags, code := attrs[pos], attrs[pos+1]
		hdr, length := 3, int(attrs[pos+2])
		if flags&0x10 != 0 {
			hdr, length = 4, int(binary.BigEndian.Uint16(attrs[pos+2:]))
		}
		if code == 2 {
			return attrs[pos+hdr : pos+hdr+length]
		}
		pos += hdr + length
	}
	t.Fatal("the written RIB entry carries no AS_PATH")
	return nil
}

// rfc6396Update builds an IPv4 unicast UPDATE for 10.40.0.0/24 whose AS_PATH
// value is asPath.
func rfc6396Update(asPath []byte) []byte {
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	attrs = append(attrs, 0x40, 0x02, byte(len(asPath)))
	attrs = append(attrs, asPath...)
	attrs = append(attrs, 0x40, 0x03, 0x04, 192, 0, 2, 1) // NEXT_HOP
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	return append(body, 24, 10, 40, 0)
}

// TestRFC6396TwoByteSessionRouteDumpsFourByteASPath receives a route whose
// AS_PATH is in 2-byte encoding on a session without 4-byte AS support, and
// reads the AS_PATH of the TABLE_DUMP_V2 RIB entry written for it.
// Method: the whole receive-to-file path except the socket and the file:
// ingest reconciliation, the RIB, its MRT snapshot bridge, and the mrt record
// writers. The session's encoding context stays 2-byte, so a stage that read
// the stored bytes by the session's width would mis-read them.
//
// RFC requirement: RFC6396-4.3.4-1 positive -- a route received from a 2-byte AS session with AS_PATH AS_SEQUENCE 65001 65002 in 2-byte encoding is written to a TABLE_DUMP_V2 RIB entry whose AS_PATH holds the two AS numbers in 4-byte encoding.
func TestRFC6396TwoByteSessionRouteDumpsFourByteASPath(t *testing.T) {
	twoByte := []byte{0x02, 0x02, 0xFD, 0xE9, 0xFD, 0xEA}
	got := rfc6396DumpedASPath(t, rfc6396Update(twoByte), false)
	require.Equal(t,
		[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA},
		got, "the RIB entry's AS_PATH must hold every AS number in 4 octets")
}

// TestRFC6396FourByteSessionRouteDumpsASPathUnchanged is the counterpart: the
// route arrives on a 4-byte session already in the required encoding, with an
// AS number above 65535 that no 2-byte field can hold.
// Method: the same path as the positive test; the AS_PATH in the written RIB
// entry must equal the received one, so the encoding is neither widened a
// second time nor narrowed.
//
// RFC requirement: RFC6396-4.3.4-1 negative -- a route received from a 4-byte AS session with AS_PATH AS_SEQUENCE 65001 200000 already in 4-byte encoding is written to the TABLE_DUMP_V2 RIB entry octet-equal, never widened again or narrowed.
func TestRFC6396FourByteSessionRouteDumpsASPathUnchanged(t *testing.T) {
	fourByte := []byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x03, 0x0D, 0x40}
	got := rfc6396DumpedASPath(t, rfc6396Update(fourByte), true)
	require.Equal(t, fourByte, got, "a 4-byte AS_PATH must reach the RIB entry as received")
}
