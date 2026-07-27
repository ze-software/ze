package analyze

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

// mpReachAttr builds a full RFC 4760 MP_REACH_NLRI attribute.
func mpReachAttr(afi uint16, safi uint8, nextHop, nlri []byte) mrt.PathAttribute {
	v := make([]byte, 0, 5+len(nextHop)+len(nlri))
	v = binary.BigEndian.AppendUint16(v, afi)
	v = append(v, safi, byte(len(nextHop)))
	v = append(v, nextHop...)
	v = append(v, 0) // Reserved
	v = append(v, nlri...)
	return mrt.PathAttribute{Code: mrt.AttrMPReachNLRI, Value: v}
}

// mpUnreachAttr builds an RFC 4760 MP_UNREACH_NLRI attribute.
func mpUnreachAttr(afi uint16, safi uint8, nlri []byte) mrt.PathAttribute {
	v := make([]byte, 0, 3+len(nlri))
	v = binary.BigEndian.AppendUint16(v, afi)
	v = append(v, safi)
	v = append(v, nlri...)
	return mrt.PathAttribute{Code: mrt.AttrMPUnreachNLRI, Value: v}
}

func TestMPReachCount_IPv6Announcements(t *testing.T) {
	// VALIDATES: `ze-analyze show` counts prefixes announced via MP_REACH_NLRI.
	// PREVENTS: an IPv6 UPDATE rendering with no announce count at all, because
	// the UPDATE's own NLRI field is IPv4-only (RFC 4271 Section 4.3).
	nh := netip.MustParseAddr("2001:db8::1").As16()
	nlri := []byte{
		32, 0x20, 0x01, 0x0d, 0xb8, // 2001:db8::/32
		48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, // 2001:db8:1::/48
	}
	attrs := []mrt.PathAttribute{mpReachAttr(2, 1, nh[:], nlri)}
	assert.Equal(t, 2, mpReachCount(attrs))
}

func TestMPUnreachCount_IPv6Withdrawals(t *testing.T) {
	// VALIDATES: `ze-analyze show` counts prefixes withdrawn via MP_UNREACH_NLRI.
	// PREVENTS: an IPv6 withdrawal rendering as W=0, indistinguishable from a
	// record that withdraws nothing.
	nlri := []byte{32, 0x20, 0x01, 0x0d, 0xb8}
	attrs := []mrt.PathAttribute{mpUnreachAttr(2, 1, nlri)}
	assert.Equal(t, 1, mpUnreachCount(attrs))
}

func TestMPCounts_AbsentAttributes(t *testing.T) {
	// VALIDATES: an IPv4-only UPDATE contributes no MP counts.
	// PREVENTS: double-counting IPv4 prefixes that already appear in the
	// UPDATE's own NLRI and withdrawn fields.
	attrs := []mrt.PathAttribute{{Code: mrt.AttrOrigin, Value: []byte{0}}}
	assert.Equal(t, 0, mpReachCount(attrs))
	assert.Equal(t, 0, mpUnreachCount(attrs))
}

func TestMPCounts_MalformedAttributeContributesNothing(t *testing.T) {
	// VALIDATES: a damaged MP attribute contributes zero rather than aborting
	// the record, so show still prints the rest of the line.
	// PREVENTS: one corrupt attribute hiding an otherwise readable record.
	attrs := []mrt.PathAttribute{{Code: mrt.AttrMPReachNLRI, Value: []byte{0, 2}}}
	assert.Equal(t, 0, mpReachCount(attrs))
}

func TestForEachRIBEntry_ReportsMalformedRecord(t *testing.T) {
	// VALIDATES: a malformed RIB record returns an error rather than silently
	// yielding no entries.
	// PREVENTS: silent truncation -- "fewer routes than the file contains"
	// being indistinguishable from "the file has fewer routes".
	called := 0
	err := forEachRIBEntry([]byte{0, 1}, mrt.TDV2RIBIPv4Unicast, func(uint16, []byte) { called++ })
	require.Error(t, err)
	assert.Zero(t, called)
}

func TestForEachRIBEntry_WellFormedRecordYieldsEntries(t *testing.T) {
	// VALIDATES: a well-formed RIB record yields its entries and no error.
	// PREVENTS: the stricter decoder rejecting valid records.
	rec := []byte{
		0, 0, 0, 1, // sequence number
		8, 10, // 10.0.0.0/8
		0, 1, // entry count
		0, 0, // peer index
		0, 0, 0, 0, // originated time
		0, 4, // attribute length
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
	}
	var got [][]byte
	err := forEachRIBEntry(rec, mrt.TDV2RIBIPv4Unicast, func(_ uint16, attrs []byte) {
		got = append(got, attrs)
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, []byte{0x40, 0x01, 0x01, 0x00}, got[0])
}

func TestMalformedCounter_ReportsOnlyWhenDamaged(t *testing.T) {
	// VALIDATES: the operator is told how many records were skipped, and gets
	// no noise when the file is clean.
	// PREVENTS: an incomplete analysis being presented as a complete one.
	var clean malformedCounter
	var buf bytes.Buffer
	clean.note(nil)
	clean.report(&buf)
	assert.Empty(t, buf.String(), "a clean run must print no warning")

	var damaged malformedCounter
	damaged.note(assert.AnError)
	damaged.note(assert.AnError)
	damaged.note(nil)
	buf.Reset()
	damaged.report(&buf)
	assert.Contains(t, buf.String(), "2 malformed RIB record(s) skipped")
	assert.Contains(t, buf.String(), "incomplete")
}
