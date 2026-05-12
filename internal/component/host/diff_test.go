package host

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: DiffInventory detects NIC carrier state change.
// PREVENTS: carrier flip going unreported.
func TestDiffEmitsCarrierChange(t *testing.T) {
	prev := &Inventory{
		NICs: []NICInfo{
			{Name: "eth0", Carrier: true},
			{Name: "eth1", Carrier: false},
		},
	}
	curr := &Inventory{
		NICs: []NICInfo{
			{Name: "eth0", Carrier: false},
			{Name: "eth1", Carrier: true},
		},
	}

	events := DiffInventory(prev, curr)
	assert.Len(t, events, 2)

	bySubject := map[string]DiffEvent{}
	for _, e := range events {
		bySubject[e.Subject] = e
	}

	eth0 := bySubject["eth0"]
	assert.Equal(t, "carrier-change", eth0.Code)
	assert.Equal(t, false, eth0.Detail["carrier"])

	eth1 := bySubject["eth1"]
	assert.Equal(t, "carrier-change", eth1.Code)
	assert.Equal(t, true, eth1.Detail["carrier"])
}

// VALIDATES: DiffInventory detects ECC correctable error increment.
// PREVENTS: growing ECC errors going unreported.
func TestDiffEmitsECCError(t *testing.T) {
	prev := &Inventory{
		Memory: &MemoryInfo{ECCPresent: true, ECCCorrectableErrors: 5},
	}
	curr := &Inventory{
		Memory: &MemoryInfo{ECCPresent: true, ECCCorrectableErrors: 8},
	}

	events := DiffInventory(prev, curr)
	assert.Len(t, events, 1)
	assert.Equal(t, "ecc-error", events[0].Code)
	assert.Equal(t, uint64(8), events[0].Detail["correctable"])
}

// VALIDATES: DiffInventory detects CPU throttle count increase.
// PREVENTS: thermal throttle events going unreported.
func TestDiffEmitsThrottle(t *testing.T) {
	prev := &Inventory{
		CPU: &CPUInfo{
			Cores: []CoreInfo{
				{CPU: 0, CoreThrottleCount: 10, PackageThrottleCount: 2},
			},
		},
	}
	curr := &Inventory{
		CPU: &CPUInfo{
			Cores: []CoreInfo{
				{CPU: 0, CoreThrottleCount: 15, PackageThrottleCount: 2},
			},
		},
	}

	events := DiffInventory(prev, curr)
	assert.Len(t, events, 1)
	assert.Equal(t, "throttle", events[0].Code)
	assert.Equal(t, 0, events[0].Detail["cpu"])
}

// VALIDATES: DiffInventory returns empty when nothing changed.
// PREVENTS: spurious events on identical snapshots.
func TestDiffNoChange(t *testing.T) {
	inv := &Inventory{
		NICs: []NICInfo{{Name: "eth0", Carrier: true}},
		Memory: &MemoryInfo{
			ECCPresent:           true,
			ECCCorrectableErrors: 5,
		},
	}
	events := DiffInventory(inv, inv)
	assert.Empty(t, events)
}

// VALIDATES: DiffInventory handles nil prev gracefully.
// PREVENTS: panic on first-ever diff.
func TestDiffNilPrev(t *testing.T) {
	curr := &Inventory{
		NICs: []NICInfo{{Name: "eth0", Carrier: true}},
	}
	events := DiffInventory(nil, curr)
	assert.Empty(t, events)
}

// VALIDATES: DiffInventory handles nil curr gracefully.
// PREVENTS: panic when detection fails.
func TestDiffNilCurr(t *testing.T) {
	prev := &Inventory{
		NICs: []NICInfo{{Name: "eth0", Carrier: true}},
	}
	events := DiffInventory(prev, nil)
	assert.Empty(t, events)
}
