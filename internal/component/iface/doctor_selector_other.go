//go:build !linux

// Design: docs/architecture/iface/logical-name-resolution.md -- the config-only binding a doctor check reads
// Related: doctor_selector.go -- SelectedNetDevice, the caller

package iface

// macSelectorsReadable is false off Linux: no device addresses were read, so
// SelectedNetDevice says the MAC was not checked rather than that no device
// carries it.
const macSelectorsReadable = false

// netDevicesWithAddress has no sysfs to read off Linux, so no device is found
// to carry mac, and SelectedNetDevice reports the mac match selector as
// unmatched, at warning severity, naming the address it could not find. The
// daemon binds selectors on Linux only.
func netDevicesWithAddress(string) []string {
	return []string{}
}
