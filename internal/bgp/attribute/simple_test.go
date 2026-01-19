package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextHop(t *testing.T) {
	nh := &NextHop{Addr: netip.MustParseAddr("10.0.0.1")}

	assert.Equal(t, AttrNextHop, nh.Code())
	assert.Equal(t, FlagTransitive, nh.Flags())
	assert.Equal(t, 4, nh.Len())
	assert.Equal(t, []byte{10, 0, 0, 1}, nh.Pack())
}

func TestNextHopParse(t *testing.T) {
	nh, err := ParseNextHop([]byte{192, 168, 1, 1})
	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), nh.Addr)
}

func TestMED(t *testing.T) {
	med := MED(100)

	assert.Equal(t, AttrMED, med.Code())
	assert.Equal(t, FlagOptional, med.Flags())
	assert.Equal(t, 4, med.Len())
	assert.Equal(t, []byte{0, 0, 0, 100}, med.Pack())
}

func TestMEDParse(t *testing.T) {
	med, err := ParseMED([]byte{0, 0, 1, 0})
	require.NoError(t, err)
	assert.Equal(t, MED(256), med)
}

func TestLocalPref(t *testing.T) {
	lp := LocalPref(200)

	assert.Equal(t, AttrLocalPref, lp.Code())
	assert.Equal(t, FlagTransitive, lp.Flags())
	assert.Equal(t, 4, lp.Len())
	assert.Equal(t, []byte{0, 0, 0, 200}, lp.Pack())
}

func TestLocalPrefParse(t *testing.T) {
	lp, err := ParseLocalPref([]byte{0, 0, 0, 100})
	require.NoError(t, err)
	assert.Equal(t, LocalPref(100), lp)
}

func TestAtomicAggregate(t *testing.T) {
	aa := AtomicAggregate{}

	assert.Equal(t, AttrAtomicAggregate, aa.Code())
	assert.Equal(t, FlagTransitive, aa.Flags())
	assert.Equal(t, 0, aa.Len())
	assert.Nil(t, aa.Pack())
}

func TestAggregator(t *testing.T) {
	agg := &Aggregator{
		ASN:     65001,
		Address: netip.MustParseAddr("10.0.0.1"),
	}

	assert.Equal(t, AttrAggregator, agg.Code())
	assert.Equal(t, FlagOptional|FlagTransitive, agg.Flags())
	assert.Equal(t, 8, agg.Len())

	packed := agg.Pack()
	assert.Equal(t, []byte{0, 0, 0xFD, 0xE9, 10, 0, 0, 1}, packed)
}

func TestAggregatorParse4Byte(t *testing.T) {
	data := []byte{0, 0, 0xFD, 0xE9, 10, 0, 0, 1}
	agg, err := ParseAggregator(data, true)
	require.NoError(t, err)
	assert.Equal(t, uint32(65001), agg.ASN)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), agg.Address)
}

func TestClusterList(t *testing.T) {
	cl := ClusterList{0x01020304, 0x05060708}

	assert.Equal(t, AttrClusterList, cl.Code())
	assert.Equal(t, FlagOptional, cl.Flags())
	assert.Equal(t, 8, cl.Len())

	packed := cl.Pack()
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6, 7, 8}, packed)
}

func TestClusterListParse(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	cl, err := ParseClusterList(data)
	require.NoError(t, err)
	assert.Equal(t, ClusterList{0x01020304, 0x05060708}, cl)
}

func TestOriginatorID(t *testing.T) {
	oid := OriginatorID(netip.MustParseAddr("10.0.0.1"))

	assert.Equal(t, AttrOriginatorID, oid.Code())
	assert.Equal(t, FlagOptional, oid.Flags())
	assert.Equal(t, 4, oid.Len())
	assert.Equal(t, []byte{10, 0, 0, 1}, oid.Pack())
}

// TestOriginatorIDParse verifies ORIGINATOR_ID parsing (RFC 4456).
//
// VALIDATES: 4-byte router ID is correctly parsed.
// PREVENTS: Route reflection failures due to parse errors.
func TestOriginatorIDParse(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		data := []byte{192, 168, 1, 1}
		oid, err := ParseOriginatorID(data)
		require.NoError(t, err)
		assert.Equal(t, netip.MustParseAddr("192.168.1.1"), netip.Addr(oid))
	})

	t.Run("wrong length short", func(t *testing.T) {
		data := []byte{10, 0, 0}
		_, err := ParseOriginatorID(data)
		assert.ErrorIs(t, err, ErrInvalidLength)
	})

	t.Run("wrong length long", func(t *testing.T) {
		data := []byte{10, 0, 0, 1, 2}
		_, err := ParseOriginatorID(data)
		assert.ErrorIs(t, err, ErrInvalidLength)
	})

	t.Run("empty", func(t *testing.T) {
		_, err := ParseOriginatorID(nil)
		assert.ErrorIs(t, err, ErrInvalidLength)
	})
}
