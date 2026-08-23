package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

func TestNextHop(t *testing.T) {
	t.Parallel()
	nh := &NextHop{Addr: netip.MustParseAddr("10.0.0.1")}

	assert.Equal(t, AttrNextHop, nh.Code())
	assert.Equal(t, FlagTransitive, nh.Flags())
	assert.Equal(t, 4, nh.Len())

	buf := make([]byte, 64)
	n := nh.WriteTo(buf, 0)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte{10, 0, 0, 1}, buf[:n])
}

func TestNextHopParse(t *testing.T) {
	t.Parallel()
	nh, err := ParseNextHop([]byte{192, 168, 1, 1})
	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), nh.Addr)
}

func TestMED(t *testing.T) {
	t.Parallel()
	med := MED(100)

	assert.Equal(t, AttrMED, med.Code())
	assert.Equal(t, FlagOptional, med.Flags())
	assert.Equal(t, 4, med.Len())

	buf := make([]byte, 64)
	n := med.WriteTo(buf, 0)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte{0, 0, 0, 100}, buf[:n])
}

func TestMEDParse(t *testing.T) {
	t.Parallel()
	med, err := ParseMED([]byte{0, 0, 1, 0})
	require.NoError(t, err)
	assert.Equal(t, MED(256), med)
}

func TestLocalPref(t *testing.T) {
	t.Parallel()
	lp := LocalPref(200)

	assert.Equal(t, AttrLocalPref, lp.Code())
	assert.Equal(t, FlagTransitive, lp.Flags())
	assert.Equal(t, 4, lp.Len())

	buf := make([]byte, 64)
	n := lp.WriteTo(buf, 0)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte{0, 0, 0, 200}, buf[:n])
}

func TestLocalPrefParse(t *testing.T) {
	t.Parallel()
	lp, err := ParseLocalPref([]byte{0, 0, 0, 100})
	require.NoError(t, err)
	assert.Equal(t, LocalPref(100), lp)
}

func TestAtomicAggregate(t *testing.T) {
	t.Parallel()
	aa := AtomicAggregate{}

	assert.Equal(t, AttrAtomicAggregate, aa.Code())
	assert.Equal(t, FlagTransitive, aa.Flags())
	assert.Equal(t, 0, aa.Len())

	buf := make([]byte, 64)
	n := aa.WriteTo(buf, 0)
	assert.Equal(t, 0, n)
}

func TestAggregator(t *testing.T) {
	t.Parallel()
	agg := &Aggregator{
		ASN:     65001,
		Address: netip.MustParseAddr("10.0.0.1"),
	}

	assert.Equal(t, AttrAggregator, agg.Code())
	assert.Equal(t, FlagOptional|FlagTransitive, agg.Flags())
	assert.Equal(t, 8, agg.Len())

	buf := make([]byte, 64)
	n := agg.WriteTo(buf, 0)
	assert.Equal(t, 8, n)
	assert.Equal(t, []byte{0, 0, 0xFD, 0xE9, 10, 0, 0, 1}, buf[:n])
}

func TestAggregatorParse4Byte(t *testing.T) {
	t.Parallel()
	data := []byte{0, 0, 0xFD, 0xE9, 10, 0, 0, 1}
	agg, err := ParseAggregator(data, true)
	require.NoError(t, err)
	assert.Equal(t, uint32(65001), agg.ASN)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), agg.Address)
}

func TestClusterList(t *testing.T) {
	t.Parallel()
	cl := ClusterList{0x01020304, 0x05060708}

	assert.Equal(t, AttrClusterList, cl.Code())
	assert.Equal(t, FlagOptional, cl.Flags())
	assert.Equal(t, 8, cl.Len())

	buf := make([]byte, 64)
	n := cl.WriteTo(buf, 0)
	assert.Equal(t, 8, n)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6, 7, 8}, buf[:n])
}

func TestClusterListParse(t *testing.T) {
	t.Parallel()
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	cl, err := ParseClusterList(data)
	require.NoError(t, err)
	assert.Equal(t, ClusterList{0x01020304, 0x05060708}, cl)
}

func TestOriginatorID(t *testing.T) {
	t.Parallel()
	oid := OriginatorID(netip.MustParseAddr("10.0.0.1"))

	assert.Equal(t, AttrOriginatorID, oid.Code())
	assert.Equal(t, FlagOptional, oid.Flags())
	assert.Equal(t, 4, oid.Len())

	buf := make([]byte, 64)
	n := oid.WriteTo(buf, 0)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte{10, 0, 0, 1}, buf[:n])
}

// TestOriginatorIDParse verifies ORIGINATOR_ID parsing (RFC 4456).
//
// VALIDATES: 4-byte router ID is correctly parsed.
// PREVENTS: Route reflection failures due to parse errors.
func TestOriginatorIDParse(t *testing.T) {
	t.Parallel()
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		data := []byte{192, 168, 1, 1}
		oid, err := ParseOriginatorID(data)
		require.NoError(t, err)
		assert.Equal(t, netip.MustParseAddr("192.168.1.1"), netip.Addr(oid))
	})

	t.Run("wrong length short", func(t *testing.T) {
		t.Parallel()
		data := []byte{10, 0, 0}
		_, err := ParseOriginatorID(data)
		assert.ErrorIs(t, err, ErrInvalidLength)
	})

	t.Run("wrong length long", func(t *testing.T) {
		t.Parallel()
		data := []byte{10, 0, 0, 1, 2}
		_, err := ParseOriginatorID(data)
		assert.ErrorIs(t, err, ErrInvalidLength)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		_, err := ParseOriginatorID(nil)
		assert.ErrorIs(t, err, ErrInvalidLength)
	})
}

// attrCanary is the pattern an untouched destination octet holds.
//
// A test that compares only the returned COUNT cannot see a write that returns
// the promised number and writes more, which is how an unbounded address copy
// survived TestLenMatchesWriteTo. So every test below writes at a NON-ZERO offset
// into a buffer filled with this pattern, and asserts the pattern survives on both
// sides of the declared value region (ai/rules/interop-and-goal-validation.md,
// "Prove the test discriminates").
const attrCanary = 0x5A

// canaryBuf returns a destination buffer of size octets, all attrCanary.
func canaryBuf(size int) []byte {
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = attrCanary
	}
	return buf
}

// assertOnlyRegionWritten asserts buf holds want at off and the canary elsewhere.
func assertOnlyRegionWritten(t *testing.T, buf []byte, off int, want []byte) {
	t.Helper()
	assert.Equal(t, want, buf[off:off+len(want)], "the attribute value")
	for i := range buf {
		if i >= off && i < off+len(want) {
			continue
		}
		require.EqualValues(t, attrCanary, buf[i],
			"octet %d lies outside the declared value region and was written", i)
	}
}

// fixedWidthAddressForms are the four netip.Addr forms an IP address field the RFC
// fixes at four octets must answer for, with the four octets each one owes.
//
// The boundary here is the address FORM, because the form is what used to change
// the octet count: netip.Addr.AsSlice answers 0, 4 or 16 by the value, while
// AGGREGATOR, AS4_AGGREGATOR and ORIGINATOR_ID each declare 4.
var fixedWidthAddressForms = []struct {
	name  string
	addr  netip.Addr
	field [4]byte
}{
	{"IPv4", netip.MustParseAddr("192.0.2.1"), [4]byte{192, 0, 2, 1}},
	{"IPv4-in-IPv6", netip.MustParseAddr("::ffff:192.0.2.1"), [4]byte{192, 0, 2, 1}},
	{"IPv6", netip.MustParseAddr("2001:db8::1"), [4]byte{0, 0, 0, 0}},
	{"zero Addr", netip.Addr{}, [4]byte{0, 0, 0, 0}},
}

// TestAggregatorWriteToStaysWithinLen covers AC-1.
//
// RFC 4271 Section 5.1.7 gives AGGREGATOR an AS number and an IP address, and
// RFC 6793 Section 4 makes the pair four octets each, which is the 8 Len returns.
//
// VALIDATES: WriteTo returns 8 and touches exactly those eight octets for every
// address form, writing the four IPv4 octets when the address has an IPv4 form and
// four zeros when it does not.
//
// PREVENTS: an IPv6 Address writing twelve octets past the region the size query
// reserved while still returning 8, which no caller can detect.
func TestAggregatorWriteToStaysWithinLen(t *testing.T) {
	t.Parallel()
	for _, form := range fixedWidthAddressForms {
		t.Run(form.name, func(t *testing.T) {
			t.Parallel()
			agg := &Aggregator{ASN: 65001, Address: form.addr}
			buf := canaryBuf(64)

			n := agg.WriteTo(buf, 7)

			assert.Equal(t, 8, n)
			assert.Equal(t, agg.Len(), n, "Len must promise exactly what WriteTo writes")
			assertOnlyRegionWritten(t, buf, 7, append([]byte{0x00, 0x00, 0xFD, 0xE9}, form.field[:]...))
		})
	}
}

// TestAggregatorWriteToWithContextStaysWithinLen covers AC-2.
//
// RFC 6793 Section 4.1: a speaker sends AGGREGATOR with a two-octet AS number to a
// peer that did not negotiate four-octet support, and substitutes AS_TRANS (23456,
// RFC 6793 Section 9) for an AS number that does not fit. Both branches carry the
// same four-octet address field.
//
// VALIDATES: every context and address form returns LenWithContext for that context
// and touches exactly that many octets, with AS_TRANS still substituted.
//
// PREVENTS: bounding the eight-octet branch alone and leaving the six-octet one
// writing an address past the value it declared.
func TestAggregatorWriteToWithContextStaysWithinLen(t *testing.T) {
	t.Parallel()
	contexts := []struct {
		name string
		ctx  *bgpctx.EncodingContext
	}{
		{"nil context", nil},
		{"four-octet ASN peer", bgpctx.EncodingContextForASN4(true)},
		{"two-octet ASN peer", bgpctx.EncodingContextForASN4(false)},
	}
	asns := []struct {
		name   string
		asn    uint32
		wide   []byte // the four-octet encoding
		narrow []byte // the two-octet encoding, AS_TRANS above 65535
	}{
		{"ASN 65001", 65001, []byte{0x00, 0x00, 0xFD, 0xE9}, []byte{0xFD, 0xE9}},
		{"ASN 65536 (above two octets)", 65536, []byte{0x00, 0x01, 0x00, 0x00}, []byte{0x5B, 0xA0}},
	}

	for _, form := range fixedWidthAddressForms {
		for _, tc := range contexts {
			for _, as := range asns {
				t.Run(form.name+"/"+tc.name+"/"+as.name, func(t *testing.T) {
					t.Parallel()
					agg := &Aggregator{ASN: as.asn, Address: form.addr}
					buf := canaryBuf(64)

					n := agg.WriteToWithContext(buf, 7, nil, tc.ctx)

					want := append(append([]byte{}, as.wide...), form.field[:]...)
					if tc.ctx != nil && !tc.ctx.ASN4() {
						want = append(append([]byte{}, as.narrow...), form.field[:]...)
					}
					assert.Equal(t, len(want), n)
					assert.Equal(t, agg.LenWithContext(nil, tc.ctx), n,
						"LenWithContext must promise exactly what WriteToWithContext writes")
					assertOnlyRegionWritten(t, buf, 7, want)
				})
			}
		}
	}
}

// TestOriginatorIDWriteToStaysWithinLen covers AC-4.
//
// RFC 4456 Section 8: ORIGINATOR_ID "is 4 bytes long".
//
// VALIDATES: WriteTo returns 4 and touches exactly four octets for every address
// form.
//
// PREVENTS: an IPv6 value returning 16 and overwriting twelve octets the attribute
// never declared, which is the same defect with the count visible rather than
// hidden.
func TestOriginatorIDWriteToStaysWithinLen(t *testing.T) {
	t.Parallel()
	for _, form := range fixedWidthAddressForms {
		t.Run(form.name, func(t *testing.T) {
			t.Parallel()
			orig := OriginatorID(form.addr)
			buf := canaryBuf(64)

			n := orig.WriteTo(buf, 7)

			assert.Equal(t, 4, n)
			assert.Equal(t, orig.Len(), n, "Len must promise exactly what WriteTo writes")
			assertOnlyRegionWritten(t, buf, 7, form.field[:])
		})
	}
}

// TestNextHopLenMatchesWriteToForEveryAddressForm covers AC-5.
//
// NEXT_HOP is the attribute whose count the RFC does NOT fix: RFC 4271
// Section 5.1.3 defines the four-octet IPv4 form, and an IPv6 address reaches this
// attribute at sixteen octets through RFC 4760 compatibility. So the direction of
// the rule inverts here -- the value decides the count, and Len must follow the
// write rather than the write following Len.
//
// VALIDATES: Len equals what WriteTo returns and equals len(Addr.AsSlice()) for
// every address form, and the write stays inside that many octets.
//
// PREVENTS: the zero Addr counting four while the write emits none, which
// desynchronizes every attribute after it in the block.
func TestNextHopLenMatchesWriteToForEveryAddressForm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		addr netip.Addr
		want []byte
	}{
		{"IPv4", netip.MustParseAddr("192.0.2.1"), []byte{192, 0, 2, 1}},
		{"IPv4-in-IPv6", netip.MustParseAddr("::ffff:192.0.2.1"), []byte{
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF, 192, 0, 2, 1,
		}},
		{"IPv6", netip.MustParseAddr("2001:db8::1"), []byte{
			0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
		}},
		{"zero Addr", netip.Addr{}, []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			nh := &NextHop{Addr: tt.addr}
			buf := canaryBuf(64)

			n := nh.WriteTo(buf, 7)

			assert.Equal(t, len(tt.want), nh.Len())
			assert.Equal(t, nh.Len(), n, "Len must promise exactly what WriteTo writes")
			assert.Equal(t, len(tt.addr.AsSlice()), nh.Len(),
				"Len must be measured with the same source the write reads")
			assertOnlyRegionWritten(t, buf, 7, tt.want)
		})
	}
}

// TestNextHopValidateNextHops covers the refusal half of the NEXT_HOP rule.
//
// Len and WriteTo now agree at ZERO for an address with no wire form, so the
// announce plan's count check passes it. RFC 4271 Section 5.1.3 admits no
// zero-length NEXT_HOP, which is why the refusal is stated separately.
//
// VALIDATES: the zero Addr is refused with ErrUnencodableNextHop, and every valid
// form is accepted.
//
// PREVENTS: a zero-length NEXT_HOP value reaching the wire once the counts agree.
func TestNextHopValidateNextHops(t *testing.T) {
	t.Parallel()
	for _, form := range fixedWidthAddressForms {
		t.Run(form.name, func(t *testing.T) {
			t.Parallel()
			err := (&NextHop{Addr: form.addr}).ValidateNextHops()
			if form.addr.IsValid() {
				assert.NoError(t, err, "a valid address must encode")
				return
			}
			require.ErrorIs(t, err, ErrUnencodableNextHop)
			assert.Contains(t, err.Error(), "NEXT_HOP",
				"the wrap must name the attribute the operator has to fix")
		})
	}
}

// TestAttributeWriteToAllocatesNothing covers AC-7.
//
// These methods sit on the announce path, which runs for every route toward every
// peer. ai/rules/performance.md fixes the buffer-first shape for that reason, and
// the bound added here writes through netip.Addr.As4, an array value, rather than
// through the AsSlice it replaced.
//
// VALIDATES: every changed size query and write costs zero heap allocations.
//
// PREVENTS: the fix paying for correctness with an allocation on the hottest
// encode path in the daemon.
func TestAttributeWriteToAllocatesNothing(t *testing.T) {
	// No t.Parallel: AllocsPerRun sets GOMAXPROCS to 1 for its measurement, and a
	// test running beside it allocates into the same count.
	buf := make([]byte, 64)
	asn4 := bgpctx.EncodingContextForASN4(true)
	old := bgpctx.EncodingContextForASN4(false)

	// An IPv6 address is the form that used to reach netip.Addr.AsSlice, which
	// allocates a sixteen-octet slice. Measuring the IPv4 form alone would miss it.
	addr := netip.MustParseAddr("2001:db8::1")
	agg := &Aggregator{ASN: 65001, Address: addr}
	as4agg := &AS4Aggregator{ASN: 65001, Address: addr}
	orig := OriginatorID(addr)
	nh := &NextHop{Addr: addr}

	calls := []struct {
		name string
		fn   func()
	}{
		{"Aggregator.WriteTo", func() { agg.WriteTo(buf, 0) }},
		{"Aggregator.WriteToWithContext ASN4", func() { agg.WriteToWithContext(buf, 0, nil, asn4) }},
		{"Aggregator.WriteToWithContext two-octet", func() { agg.WriteToWithContext(buf, 0, nil, old) }},
		{"AS4Aggregator.WriteTo", func() { as4agg.WriteTo(buf, 0) }},
		{"AS4Aggregator.WriteToWithContext", func() { as4agg.WriteToWithContext(buf, 0, nil, asn4) }},
		{"OriginatorID.WriteTo", func() { orig.WriteTo(buf, 0) }},
		{"NextHop.Len", func() { _ = nh.Len() }},
		{"NextHop.ValidateNextHops", func() { _ = nh.ValidateNextHops() }},
	}

	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			assert.Zero(t, testing.AllocsPerRun(100, c.fn),
				"%s must allocate nothing on the announce path", c.name)
		})
	}
}
