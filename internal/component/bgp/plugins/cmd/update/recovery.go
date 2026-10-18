// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package update

import (
	"fmt"
	"net/netip"
)

// recoveryFromMeta distinguishes an ordinary withdrawal from the forwarding
// owner's DOWN inventory. The reactor's writer decides which keys the departed
// source's ended session still owns, so the request carries only the source.
func recoveryFromMeta(meta map[string]any) (netip.Addr, error) {
	value, present := meta["recovery-source"]
	if !present {
		return netip.Addr{}, nil
	}
	source, ok := value.(string)
	if !ok {
		return netip.Addr{}, fmt.Errorf("recovery-source must be an address string")
	}
	address, err := netip.ParseAddr(source)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("recovery-source: %w", err)
	}
	return address, nil
}
