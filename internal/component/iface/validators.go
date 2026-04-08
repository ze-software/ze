// Design: docs/features/interfaces.md -- MAC address completion for config validators
// Related: discover.go -- DiscoverInterfaces used by the CompleteFn

package iface

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterCompleteFn("mac-address", macAddressCompleteFn)
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
