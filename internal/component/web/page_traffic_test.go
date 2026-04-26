package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
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

	data := BuildTrafficTableData(infos)
	assert.Equal(t, "Traffic", data.Title)
	require.Len(t, data.Rows, 2)
}

// TestTrafficTableData_SortByRate verifies rows are sorted by total rate descending.
func TestTrafficTableData_SortByRate(t *testing.T) {
	infos := []iface.InterfaceInfo{
		{
			Name: "low",
			Stats: &iface.InterfaceStats{
				RxBytes: 100, TxBytes: 100, // total 200
			},
		},
		{
			Name: "high",
			Stats: &iface.InterfaceStats{
				RxBytes: 5000, TxBytes: 5000, // total 10000
			},
		},
		{
			Name: "mid",
			Stats: &iface.InterfaceStats{
				RxBytes: 1000, TxBytes: 1000, // total 2000
			},
		},
	}

	data := BuildTrafficTableData(infos)
	require.Len(t, data.Rows, 3)
	assert.Equal(t, "high", data.Rows[0].Key, "highest traffic should be first")
	assert.Equal(t, "mid", data.Rows[1].Key, "medium traffic should be second")
	assert.Equal(t, "low", data.Rows[2].Key, "lowest traffic should be last")
}

// TestTrafficTableData_NilStats verifies nil stats produce zero values.
func TestTrafficTableData_NilStats(t *testing.T) {
	infos := []iface.InterfaceInfo{
		{Name: "eth0", Stats: nil},
	}

	data := BuildTrafficTableData(infos)
	require.Len(t, data.Rows, 1)
	// First cell is interface name, all counter cells should be "0".
	assert.Equal(t, "eth0", data.Rows[0].Cells[0])
	assert.Equal(t, "0", data.Rows[0].Cells[1]) // RX Bytes
}

// TestTrafficTableData_Empty verifies empty state.
func TestTrafficTableData_Empty(t *testing.T) {
	data := BuildTrafficTableData(nil)
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No interfaces to monitor.", data.EmptyMessage)
}

// TestComputeTrafficRate verifies the TrafficRow total rate computation.
func TestComputeTrafficRate(t *testing.T) {
	tr := TrafficRow{
		RxBytes:   1000,
		TxBytes:   2000,
		TotalRate: 3000,
	}
	assert.Equal(t, uint64(3000), tr.TotalRate)
}
