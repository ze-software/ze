// Design: docs/features/interfaces.md -- MAC address completion for config validators
// Related: discover.go -- DiscoverInterfaces used by the CompleteFn

package iface

import (
	"github.com/ze-software/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterCompleteFn("mac-address", macAddressCompleteFn)
	yang.RegisterCompleteFn("os-device-name", osDeviceNameCompleteFn)
}

// osDeviceNameCompleteFn returns the kernel device names present on this box, so
// the `os-name` selector completes to a device that exists instead of being
// typed blind. A mistyped alias is not refused by validation -- the YANG lets a
// binding defer until its device appears -- so completion is what separates an
// operator's typo from an operator's intent. Called lazily at completion time,
// not at init.
func osDeviceNameCompleteFn() []string {
	discovered, err := DiscoverInterfaces()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(discovered))
	for _, di := range discovered {
		if di.Name != "" {
			names = append(names, di.Name)
		}
	}
	return names
}

// macAddressCompleteFn returns MAC addresses from discovered OS interfaces
// for CLI tab completion. Called lazily at completion time, not at init.
func macAddressCompleteFn() []string {
	discovered, err := DiscoverInterfaces()
	if err != nil {
		return nil
	}
	var macs []string
	for _, di := range discovered {
		if di.MAC != "" && di.MAC != "00:00:00:00:00:00" {
			macs = append(macs, di.MAC)
		}
	}
	return macs
}
