package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
