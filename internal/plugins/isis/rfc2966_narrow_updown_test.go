// Design: docs/architecture/isis/isis-9-spf-rib.md -- L1/L2 route leaking with the up/down bit.
//
// VALIDATES: RFC 2966 section 2 in the narrow encoding the bit is defined in:
// bit 8 (0x80) of the default metric octet of a TLV 128/130 entry. The engine
// originates a leaked narrow prefix in its native TLV (lsdb/origination.go), so
// an L2-derived narrow prefix leaked into L1 carries the bit on the wire, and an
// L1-derived one leaked into L2 does not.
// PREVENTS: the up/down bit being carried only by TLV 135 while a leaked narrow
// entry goes out with bit 8 clear, which lets an L1L2 router in the area leak
// the prefix back into L2 (the loop RFC 2966 closes).

package isis

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
)

// narrowEntryMetricOctet returns the raw default-metric octet of prefix in a
// TLV 128 (external false) or TLV 130 (external true) of lsp.
func narrowEntryMetricOctet(t *testing.T, lsp *packet.LSP, prefix netip.Prefix, external bool) (octet byte, present bool) {
	t.Helper()
	typ := uint8(packet.TLVIPInternalReachability)
	if external {
		typ = packet.TLVIPExternalReachability
	}
	const entryLen = 12 // RFC 1195 sec 5.2: metric octets 4, IP address 4, mask 4
	for _, tl := range lsp.TLVs {
		if tl.Type != typ {
			continue
		}
		decoded, err := packet.DecodeNarrowIPReachTLV(tl.Value, external)
		if err != nil {
			t.Fatalf("decode TLV %d: %v", typ, err)
		}
		for i, ent := range decoded.Entries {
			if ent.Prefix == prefix {
				return tl.Value[i*entryLen], true
			}
		}
	}
	return 0, false
}

// RFC requirement: RFC2966-2-1 positive -- the engine's originated L1 LSP, decoded
// from the stored bytes, carries the L2-derived narrow prefixes leaked into L1
// (one internal in TLV 128, one external in TLV 130) with bit 8 of the default
// metric octet set to one and the six-bit metric intact.
// RFC requirement: RFC2966-2-1 negative -- the bit is set only for L2-derived
// prefixes advertised into L1: the L1-derived narrow prefix leaked into L2 goes
// out in the L2 LSP's TLV 128 with bit 8 clear.
func TestRFC2966NarrowLeakUpDownBit(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"metric":"10"}}}}}`)
	defer eng.shutdown()
	node := eng.cfg.SystemID

	downInternal := netip.MustParsePrefix("10.2.0.0/24")
	downExternal := netip.MustParsePrefix("10.3.0.0/24")
	up := netip.MustParsePrefix("10.1.0.0/24")
	eng.applyLeak(spf.LeakResult{
		IntoL1: []spf.LeakedPrefix{
			{Prefix: downInternal, Metric: 27, UpDown: true, Narrow: true},
			{Prefix: downExternal, Metric: 33, UpDown: true, Narrow: true, External: true},
		},
		IntoL2: []spf.LeakedPrefix{{Prefix: up, Metric: 15, UpDown: false, Narrow: true}},
	})

	l1 := mustFrag0(t, eng, node, lsdb.Level1)
	for _, c := range []struct {
		prefix   netip.Prefix
		external bool
		metric   byte
	}{{downInternal, false, 27}, {downExternal, true, 33}} {
		octet, present := narrowEntryMetricOctet(t, l1, c.prefix, c.external)
		if !present {
			t.Fatalf("L1 LSP lacks the narrow leak %s (external %v)", c.prefix, c.external)
		}
		if octet&0x80 == 0 {
			t.Errorf("L1 LSP narrow leak %s: default metric octet %#02x, want bit 8 (0x80) set", c.prefix, octet)
		}
		if octet&0x3f != c.metric {
			t.Errorf("L1 LSP narrow leak %s: metric %d, want %d", c.prefix, octet&0x3f, c.metric)
		}
	}

	l2 := mustFrag0(t, eng, node, lsdb.Level2)
	octet, present := narrowEntryMetricOctet(t, l2, up, false)
	if !present {
		t.Fatalf("L2 LSP lacks the narrow leak %s", up)
	}
	if octet&0x80 != 0 {
		t.Errorf("L2 LSP narrow leak %s: default metric octet %#02x, want bit 8 clear", up, octet)
	}
}
