// Design: docs/architecture/wire/isis.md -- TLV 135 encoding.
// RFC: rfc/short/rfc5305.md -- Section 4.2 (the sub-TLV presence bit of TLV 135).
package packet

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// extIPControlOctet encodes one TLV 135 entry and returns its control octet and the
// encoded length, so a test can read the S bit and see whether a sub-TLV length octet
// followed the prefix.
func extIPControlOctet(t *testing.T, e ExtIPReachEntry) (byte, int) {
	t.Helper()
	tlv := ExtendedIPReachTLV{Entries: []ExtIPReachEntry{e}}
	buf := make([]byte, 64)
	n := tlv.WriteTo(buf, 0)
	// Header (2) + metric (4) precede the control octet.
	return buf[TLVHeaderLen+4], n
}

// RFC requirement: RFC5305-4.2-1 positive -- a TLV 135 entry with no sub-TLVs is encoded
// with the sub-TLV presence (S) bit 0 and no sub-TLV length octet after the prefix
// (ExtendedIPReachTLV.WriteTo, tlv_ipv4.go, sets extIPCtrlSubTLV only when hasSub).
func TestRFC5305NoSubTLVsClearsPresenceBit(t *testing.T) {
	e := ExtIPReachEntry{Prefix: netip.MustParsePrefix("10.1.0.0/16"), Metric: types.NewPrefixMetric(10)}
	ctrl, n := extIPControlOctet(t, e)
	if ctrl&extIPCtrlSubTLV != 0 {
		t.Fatalf("control octet %#02x has the S bit set with no sub-TLVs", ctrl)
	}
	// header 2 + metric 4 + control 1 + two prefix octets for /16 = 9, no length octet.
	if n != 9 {
		t.Fatalf("encoded length = %d, want 9 (no sub-TLV length octet)", n)
	}
}

// RFC requirement: RFC5305-4.2-1 negative -- the clear S bit is not a blanket: an entry
// that carries a sub-TLV is encoded with the S bit 1 and a sub-TLV length octet, so the
// bit reports the sub-TLV presence rather than staying 0 (ExtendedIPReachTLV.WriteTo,
// tlv_ipv4.go).
func TestRFC5305SubTLVsSetPresenceBit(t *testing.T) {
	e := ExtIPReachEntry{Prefix: netip.MustParsePrefix("10.1.0.0/16"), Metric: types.NewPrefixMetric(10), SubTLVs: []SubTLV{{Type: 1, Value: []byte{0xaa}}}}
	ctrl, n := extIPControlOctet(t, e)
	if ctrl&extIPCtrlSubTLV == 0 {
		t.Fatalf("control octet %#02x has the S bit clear with a sub-TLV present", ctrl)
	}
	// 9 + length octet 1 + sub-TLV (2 + 1) = 13.
	if n != 13 {
		t.Fatalf("encoded length = %d, want 13 (sub-TLV length octet and one sub-TLV)", n)
	}
}
