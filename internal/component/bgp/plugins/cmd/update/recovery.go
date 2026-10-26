// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package update

import (
	"fmt"
	"net/netip"
	"strconv"
)

// recoveryFromMeta distinguishes an ordinary withdrawal from the forwarding
// owner's DOWN inventory. The request names the departed source and the
// forward generation of the session whose end it answers (recovery-generation,
// the peer-down event's session-generation); the reactor's writer owes only
// keys that session or an older one of the source owns. A departure without
// its generation is refused, never defaulted: a zero bound owes nothing and a
// missing one would owe a newer session's route graceful restart retains.
func recoveryFromMeta(meta map[string]any) (netip.Addr, uint64, error) {
	value, present := meta["recovery-source"]
	// refuse-by-default: guards recoveryFromMeta -- no recovery-source is an ordinary withdrawal, not a departure; the invalid address is that answer, which the caller branches on
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
	value, present = meta["recovery-generation"]
	if !present {
		return netip.Addr{}, 0, fmt.Errorf("recovery-source %s without recovery-generation: "+
			"the departure must name the session-generation of the peer-down event it answers", address)
	}
	text, ok := value.(string)
	if !ok {
		return netip.Addr{}, 0, fmt.Errorf("recovery-generation must be a decimal string")
	}
	generation, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("recovery-generation: %w", err)
	}
	if generation == 0 {
		return netip.Addr{}, 0, fmt.Errorf("recovery-generation 0 names no session: " +
			"send the nonzero session-generation of the peer-down event")
	}
	return address, generation, nil
}
