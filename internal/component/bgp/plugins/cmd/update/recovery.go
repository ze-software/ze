// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package update

import (
	"fmt"
	"net/netip"
	"strconv"
)

// recoveryFromMeta distinguishes an ordinary withdrawal from the forwarding
// owner's DOWN inventory. The decimal cut avoids float64 loss across JSON IPC.
func recoveryFromMeta(meta map[string]any) (netip.Addr, uint64, error) {
	value, present := meta["recovery-source"]
	if !present {
		return netip.Addr{}, 0, nil
	}
	source, ok := value.(string)
	if !ok {
		return netip.Addr{}, 0, fmt.Errorf("recovery-source must be an address string")
	}
	address, err := netip.ParseAddr(source)
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("recovery-source: %w", err)
	}
	cut, ok := meta["recovery-cut"].(string)
	if !ok {
		return netip.Addr{}, 0, fmt.Errorf("recovery-cut must be a decimal string")
	}
	messageID, err := strconv.ParseUint(cut, 10, 64)
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("recovery-cut: %w", err)
	}
	return address, messageID, nil
}
