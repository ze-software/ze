// VALIDATES: host hardware inventory projects into a bounded fleet.InventoryReport
// with accurate per-section availability, ECC presence semantics, sorted deduped
// NIC drivers, control-character sanitization, and no raw SMART or drive serials.
// PREVENTS: unavailable hardware readings being misreported as zero values,
// quirky DMI/sysfs bytes breaking wire encoding, or drive serials/SMART data
// leaking to the management hub.

package host

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/smart"
	"github.com/ze-software/ze/pkg/fleet"
)

// TestProjectFleetInventory verifies hardware inventory projection across full,
// partial, ECC-present, ECC-absent, zero-NIC vs failed-NIC, and control-char
// inputs, plus init() registration in the fleet report producer registry.
//
// VALIDATES: AC-1, AC-16, AC-17 (host inventory summary projection and exclusion
// of raw SMART and block-device serials).
// PREVENTS: missing sections appearing available, ECC absence emitting zero
// counters, or SMART/serial data appearing in wire JSON.
func TestProjectFleetInventory(t *testing.T) {
	ts := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

	t.Run("producer registered at init", func(t *testing.T) {
		producers := fleet.ReportProducers()
		var found bool
		for _, p := range producers {
			if p.Method == fleet.VerbInventoryReport {
				found = true
				assert.Equal(t, fleetInventoryProducerName, p.Name)
				assert.Equal(t, FleetInventoryInterval, p.Interval)
				assert.NotNil(t, p.Collect)
			}
		}
		require.True(t, found, "inventory producer must be registered via init()")
	})

	t.Run("full inventory with ECC zero counters excludes SMART and drive serials", func(t *testing.T) {
		inv := &Inventory{
			CPU: &CPUInfo{
				LogicalCPUs: 16,
				ModelName:   "AMD EPYC 7313P 16-Core Processor",
			},
			NICs: []NICInfo{
				{Name: "eth0", Driver: "mlx5_core"},
				{Name: "eth1", Driver: "igb"},
				{Name: "eth2", Driver: "mlx5_core"},
				{Name: "eth3", Driver: ""},
			},
			Memory: &MemoryInfo{
				TotalBytes:             64 << 30,
				ECCPresent:             true,
				ECCCorrectableErrors:   0,
				ECCUncorrectableErrors: 0,
			},
			Storage: &StorageInfo{
				Devices: []StorageDevice{
					{
						Name:      "nvme0n1",
						SizeBytes: 512 << 30,
						Serial:    "SECRET-DRIVE-SERIAL-001",
						Smart: &smart.Info{
							Healthy:      true,
							TempCelsius:  42,
							PowerOnHours: 1200,
							PercentUsed:  3,
						},
					},
					{
						Name:      "nvme1n1",
						SizeBytes: 512 << 30,
						Serial:    "SECRET-DRIVE-SERIAL-002",
					},
				},
			},
			DMI: &DMIInfo{
				SystemProduct: "PowerEdge R6515",
				SystemSerial:  "DMI-SYS-9988",
			},
		}

		rep := ProjectFleetInventory(inv, "26.11.01", ts)
		assert.Equal(t, "2026-11-26T12:00:00Z", rep.CollectedAt)
		assert.Equal(t, "26.11.01", rep.SoftwareVersion)
		assert.Equal(t, fleet.InventoryCPU{
			Available:   true,
			LogicalCPUs: 16,
			Model:       "AMD EPYC 7313P 16-Core Processor",
		}, rep.CPU)
		assert.Equal(t, fleet.InventoryNIC{
			Available: true,
			Count:     4,
			Drivers:   []string{"igb", "mlx5_core"},
		}, rep.NIC)
		require.NotNil(t, rep.Memory.ECC, "ECCPresent=true must emit ECC struct even when error counts are 0/0")
		assert.True(t, rep.Memory.Available)
		assert.Equal(t, uint64(64<<30), rep.Memory.TotalBytes)
		assert.Equal(t, fleet.InventoryECC{CorrectableErrors: 0, UncorrectableErrors: 0}, *rep.Memory.ECC)
		assert.Equal(t, fleet.InventoryStorage{
			Available:   true,
			DeviceCount: 2,
			TotalBytes:  1024 << 30,
		}, rep.Storage)
		assert.Equal(t, fleet.InventoryDMI{
			Available: true,
			Model:     "PowerEdge R6515",
			Serial:    "DMI-SYS-9988",
		}, rep.DMI)

		wire, err := fleet.EncodeInventoryReport(rep)
		require.NoError(t, err)
		wireText := string(wire)
		assert.NotContains(t, wireText, "SECRET-DRIVE-SERIAL-001")
		assert.NotContains(t, wireText, "SECRET-DRIVE-SERIAL-002")
		assert.NotContains(t, wireText, "smart")
		assert.NotContains(t, wireText, "temp-celsius")
		assert.NotContains(t, wireText, "power-on-hours")
		assert.NotContains(t, wireText, "percent-used")

		decoded, err := fleet.DecodeInventoryReport(wire)
		require.NoError(t, err)
		assert.Equal(t, rep, decoded)
	})

	t.Run("memory without ECC omits ECC pointer and nonzero ECC counters preserved", func(t *testing.T) {
		noECC := ProjectFleetInventory(&Inventory{
			Memory: &MemoryInfo{
				TotalBytes:             16 << 30,
				ECCPresent:             false,
				ECCCorrectableErrors:   5,
				ECCUncorrectableErrors: 1,
			},
		}, "26.11.01", ts)
		assert.True(t, noECC.Memory.Available)
		assert.Nil(t, noECC.Memory.ECC)

		withECC := ProjectFleetInventory(&Inventory{
			Memory: &MemoryInfo{
				TotalBytes:             32 << 30,
				ECCPresent:             true,
				ECCCorrectableErrors:   7,
				ECCUncorrectableErrors: 2,
			},
		}, "26.11.01", ts)
		require.NotNil(t, withECC.Memory.ECC)
		assert.Equal(t, uint64(7), withECC.Memory.ECC.CorrectableErrors)
		assert.Equal(t, uint64(2), withECC.Memory.ECC.UncorrectableErrors)
	})

	t.Run("partial inventory and NIC availability distinction", func(t *testing.T) {
		empty := ProjectFleetInventory(nil, "", ts)
		assert.Equal(t, strUnknown, empty.SoftwareVersion)
		assert.Equal(t, fleet.InventoryCPU{}, empty.CPU)
		assert.Equal(t, fleet.InventoryNIC{}, empty.NIC)
		assert.Equal(t, fleet.InventoryMemory{}, empty.Memory)
		assert.Equal(t, fleet.InventoryStorage{}, empty.Storage)
		assert.Equal(t, fleet.InventoryDMI{}, empty.DMI)
		_, err := fleet.EncodeInventoryReport(empty)
		require.NoError(t, err)

		// Non-nil empty NIC slice with no scan error means 0 physical NICs were observed.
		zeroNICs := ProjectFleetInventory(&Inventory{
			CPU:  &CPUInfo{LogicalCPUs: 4, ModelName: "ARM Neoverse"},
			NICs: []NICInfo{},
		}, "dev", ts)
		assert.True(t, zeroNICs.CPU.Available)
		assert.True(t, zeroNICs.NIC.Available)
		assert.Zero(t, zeroNICs.NIC.Count)
		assert.Nil(t, zeroNICs.NIC.Drivers)
		assert.False(t, zeroNICs.Memory.Available)
		assert.False(t, zeroNICs.Storage.Available)
		assert.False(t, zeroNICs.DMI.Available)
		_, err = fleet.EncodeInventoryReport(zeroNICs)
		require.NoError(t, err)

		// A recorded "nics" error marks the NIC section unavailable even if slice is non-nil.
		failedNICs := ProjectFleetInventory(&Inventory{
			NICs:   []NICInfo{},
			Errors: []DetectError{{Path: "nics", Err: "permission denied"}},
		}, "dev", ts)
		assert.False(t, failedNICs.NIC.Available)
		assert.Zero(t, failedNICs.NIC.Count)
		assert.Nil(t, failedNICs.NIC.Drivers)
		_, err = fleet.EncodeInventoryReport(failedNICs)
		require.NoError(t, err)
	})

	t.Run("control characters invalid UTF-8 and bounds sanitized", func(t *testing.T) {
		overText := strings.Repeat("é", fleet.ReportTextMax)
		nics := make([]NICInfo, 0, fleet.InventoryDriversMax+10)
		for i := range fleet.InventoryDriversMax + 10 {
			nics = append(nics, NICInfo{
				Name:   fmt.Sprintf("eth%d", i),
				Driver: fmt.Sprintf("drv-%03d\x00\n", i),
			})
		}

		inv := &Inventory{
			CPU: &CPUInfo{
				LogicalCPUs: -1,
				ModelName:   "Xeon\x00\r\n\tGold \xff",
			},
			NICs: nics,
			DMI: &DMIInfo{
				SystemProduct: "\x07" + overText,
				SystemSerial:  "SER\x1b[31m123\x00",
			},
		}

		rep := ProjectFleetInventory(inv, "26.11\x00\n", ts)
		assert.Equal(t, "26.11", rep.SoftwareVersion)
		assert.Equal(t, 0, rep.CPU.LogicalCPUs)
		assert.Equal(t, "XeonGold", rep.CPU.Model)
		assert.Len(t, rep.NIC.Drivers, fleet.InventoryDriversMax)
		assert.Equal(t, "drv-000", rep.NIC.Drivers[0])
		assert.LessOrEqual(t, len(rep.DMI.Model), fleet.ReportTextMax)
		assert.True(t, utf8.ValidString(rep.DMI.Model))
		assert.Equal(t, "SER[31m123", rep.DMI.Serial)

		_, err := fleet.EncodeInventoryReport(rep)
		require.NoError(t, err)
	})
}
