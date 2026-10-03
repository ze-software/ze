// Design: docs/architecture/l2tp/bng-5-pppoe.md -- RFC 1332 conformance coverage
// Related: rfc1332_ipcp_test.go -- the receive side of the same row
//
// Drives the send side of RFC 1332 Section 3: the IPCP options ze writes use
// the LCP Configuration Option format, with IPCP's own option Types.

package ppp

import (
	"bytes"
	"net/netip"
	"testing"
)

// TestIPCPOptionsWrittenInLCPFormat writes all three IPCP options ze sends
// and reads the octets, then walks them with the LCP option parser.
//
// VALIDATES: RFC 1332 Section 3, send side: Type, Length, Data per option,
// the LCP format, with IPCP option Types.
// PREVENTS: an IPCP writer that drops the Length octet, writes a Length that
// disagrees with the Data, or borrows LCP option Types.
//
// RFC requirement: RFC1332-3-1 positive -- WriteIPCPOptions writes each option as Type, Length 6, four address octets, with the IPCP Types 3, 129 and 131, and ParseLCPOptions, the LCP option parser, reads the octets back as those three options.
func TestIPCPOptionsWrittenInLCPFormat(t *testing.T) {
	t.Parallel()

	opts := iPCPOptions{
		IPAddress: netip.MustParseAddr("10.0.0.5"), HasIPAddress: true,
		PrimaryDNS: netip.MustParseAddr("9.9.9.9"), HasPrimary: true,
		SecondaryDNS: netip.MustParseAddr("149.112.112.112"), HasSecondary: true,
	}
	buf := make([]byte, 64)
	n := WriteIPCPOptions(buf, 0, opts)
	want := []byte{
		3, 6, 10, 0, 0, 5,
		129, 6, 9, 9, 9, 9,
		131, 6, 149, 112, 112, 112,
	}
	if !bytes.Equal(buf[:n], want) {
		t.Fatalf("IPCP options = % x, want % x", buf[:n], want)
	}

	parsed, err := ParseLCPOptions(buf[:n])
	if err != nil {
		t.Fatalf("the LCP option parser refused the IPCP options: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("the LCP option parser read %d options, want 3", len(parsed))
	}
	for i, typ := range []uint8{IPCPOptIPAddress, IPCPOptPrimaryDNS, IPCPOptSecondaryDNS} {
		if parsed[i].Type != typ || len(parsed[i].Data) != 4 {
			t.Fatalf("option %d = type %d with %d octets, want type %d with 4", i, parsed[i].Type, len(parsed[i].Data), typ)
		}
	}
}
