package web

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestTrafficTableData_Build verifies traffic table construction.
func TestTrafficTableData_Build(t *testing.T) {
	infos := []iface.InterfaceInfo{
		{
			Name: "eth0",
			Stats: &iface.InterfaceStats{
				RxBytes: 1000, RxPackets: 10, TxBytes: 2000, TxPackets: 20,
			},
		},
		{
			Name: "eth1",
			Stats: &iface.InterfaceStats{
				RxBytes: 5000, RxPackets: 50, TxBytes: 3000, TxPackets: 30,
			},
		},
	}

	data := buildTrafficTableData(infos)
	assert.Equal(t, "Traffic", data.Title)
	require.Len(t, data.Rows, 2)
}

// TestTrafficTableData_SortByRate proves the page entry point hands the
// template the rows in the order buildTrafficRows produced.
//
// VALIDATES: buildTrafficTableData keeps the descending RX + TX order.
// PREVENTS: a one-direction rate, and a re-order between the row builder and
// the table.
//
// It reads the same fixture as TestTrafficRowsSortBySumOfBothDirections, where
// each of RX alone, TX alone and the sum gives a different order.
func TestTrafficTableData_SortByRate(t *testing.T) {
	data := buildTrafficTableData(trafficSortFixture())

	require.Len(t, data.Rows, 3)
	assert.Equal(t, "tx-led", data.Rows[0].Key, "highest total rate is first")
	assert.Equal(t, "rx-led", data.Rows[1].Key, "second total rate is second")
	assert.Equal(t, "middling", data.Rows[2].Key, "lowest total rate is last")
}

// TestTrafficTableData_NilStats verifies nil stats produce zero values.
func TestTrafficTableData_NilStats(t *testing.T) {
	infos := []iface.InterfaceInfo{
		{Name: "eth0", Stats: nil},
	}

	data := buildTrafficTableData(infos)
	require.Len(t, data.Rows, 1)
	// First cell is interface name, all counter cells should be "0".
	assert.Equal(t, "eth0", data.Rows[0].Cells[0])
	assert.Equal(t, "0", data.Rows[0].Cells[1]) // RX Bytes
}

// TestTrafficTableData_Empty verifies empty state.
func TestTrafficTableData_Empty(t *testing.T) {
	data := buildTrafficTableData(nil)
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No interfaces to monitor.", data.EmptyMessage)
}

// TestTrafficRowsSortBySumOfBothDirections proves the row order comes from
// RX + TX, and not from one direction alone.
//
// VALIDATES: buildTrafficRows sets TotalRate to RxBytes + TxBytes and sorts on
// it, descending.
// PREVENTS: a one-direction rate that orders the table by RX alone or by TX
// alone.
//
// The three rows give each formula a different order, which is what makes the
// assertion discriminate in both directions. Sum: tx-led (110), rx-led (95),
// middling (80). RX alone: rx-led (90), middling (50), tx-led (10). TX alone:
// tx-led (100), middling (30), rx-led (5). Asserting all three positions
// therefore fails for a formula that drops either direction.
func TestTrafficRowsSortBySumOfBothDirections(t *testing.T) {
	rows := buildTrafficRows(trafficSortFixture())

	require.Len(t, rows, 3)
	assert.Equal(t, "tx-led", rows[0].Key)
	assert.Equal(t, "rx-led", rows[1].Key)
	assert.Equal(t, "middling", rows[2].Key)
}

// TestTrafficSortFixtureDiscriminates proves the fixture can tell the sum from
// either term on its own.
//
// VALIDATES: trafficSortFixture orders differently under RX alone, TX alone and
// the sum.
// PREVENTS: a later edit to the fixture that equalizes the totals, which would
// weaken TestTrafficRowsSortBySumOfBothDirections and
// TestTrafficTableData_SortByRate at the same time and in silence. Both read
// this one fixture, so the property they depend on is asserted here rather than
// left as a comment.
func TestTrafficSortFixtureDiscriminates(t *testing.T) {
	order := func(rate func(*iface.InterfaceStats) uint64) []string {
		infos := trafficSortFixture()
		sort.SliceStable(infos, func(i, j int) bool {
			return rate(infos[i].Stats) > rate(infos[j].Stats)
		})
		names := make([]string, 0, len(infos))
		for i := range infos {
			names = append(names, infos[i].Name)
		}
		return names
	}

	sum := order(func(s *iface.InterfaceStats) uint64 { return s.RxBytes + s.TxBytes })
	rxOnly := order(func(s *iface.InterfaceStats) uint64 { return s.RxBytes })
	txOnly := order(func(s *iface.InterfaceStats) uint64 { return s.TxBytes })

	assert.NotEqual(t, sum, rxOnly, "fixture cannot detect a rate that drops TX")
	assert.NotEqual(t, sum, txOnly, "fixture cannot detect a rate that drops RX")

	// Distinct totals, asserted separately. The orderings above do not imply
	// them. Equal totals leave a stable sort in input order. That order can
	// still differ from both single-direction orders, so the pair above holds
	// while the sort tests assert nothing about the rate.
	totals := make(map[uint64]string, len(sum))
	for _, info := range trafficSortFixture() {
		total := info.Stats.RxBytes + info.Stats.TxBytes
		other, seen := totals[total]
		require.False(t, seen, "%s and %s share total %d, so their order is arbitrary",
			other, info.Name, total)
		totals[total] = info.Name
	}
}

// trafficSortFixture returns three interfaces whose sum order differs from
// their RX-only order and from their TX-only order. Both sort tests read it, so
// the one property they rely on is stated and computed in one place, and
// TestTrafficSortFixtureDiscriminates asserts it.
func trafficSortFixture() []iface.InterfaceInfo {
	return []iface.InterfaceInfo{
		{
			Name:  "tx-led",
			Stats: &iface.InterfaceStats{RxBytes: 10, TxBytes: 100}, // total 110
		},
		{
			Name:  "rx-led",
			Stats: &iface.InterfaceStats{RxBytes: 90, TxBytes: 5}, // total 95
		},
		{
			Name:  "middling",
			Stats: &iface.InterfaceStats{RxBytes: 50, TxBytes: 30}, // total 80
		},
	}
}
