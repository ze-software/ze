// Design: docs/features/interfaces.md -- Kernel neighbor table (ARP/ND) readback
// Overview: ifacenetlink.go -- package hub
// Related: show_linux.go -- ListInterfaces/GetInterface siblings

//go:build linux

package ifacenetlink

import (
	"fmt"
	"syscall"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// ListNeighbors returns the full kernel neighbor cache (RTM_GETNEIGH).
// family is translated from the iface.NeighborFamily* constants to the
// kernel's AF_* constants. NeighborFamilyAny leaves the dump unfiltered.
//
// Unresolved entries (no IP) are skipped. FAILED and INCOMPLETE entries
// are kept so an operator can diagnose neighbor discovery problems; the
// HardwareAddr field will be empty for those states.
func (b *netlinkBackend) ListNeighbors(family int) ([]iface.NeighborInfo, error) {
	nlFamily := netlink.FAMILY_ALL
	switch family {
	case iface.NeighborFamilyIPv4:
		nlFamily = syscall.AF_INET
	case iface.NeighborFamilyIPv6:
		nlFamily = syscall.AF_INET6
	}

	neighs, err := netlink.NeighList(0, nlFamily)
	if err != nil {
		return nil, fmt.Errorf("iface: neigh list: %w", err)
	}

	// Build an index->name map once so each neighbor's device can be
	// reported by name instead of an opaque ifindex.
	links, lerr := netlink.LinkList()
	if lerr != nil {
		return nil, fmt.Errorf("iface: link list: %w", lerr)
	}
	idxName := make(map[int]string, len(links))
	for _, l := range links {
		idxName[l.Attrs().Index] = l.Attrs().Name
	}

	result := make([]iface.NeighborInfo, 0, len(neighs))
	for i := range neighs {
		n := &neighs[i]
		if n.IP == nil || n.IP.IsUnspecified() {
			continue
		}
		fam := "ipv4"
		if n.IP.To4() == nil {
			fam = "ipv6"
		}
		entry := iface.NeighborInfo{
			Address: n.IP.String(),
			Device:  idxName[n.LinkIndex],
			Family:  fam,
			State:   neighStateString(n.State),
		}
		if len(n.HardwareAddr) > 0 {
			entry.MAC = n.HardwareAddr.String()
		}
		result = append(result, entry)
	}
	return result, nil
}

// neighStateString maps a NUD_* bitfield to a single name. NUD flags are
// in theory a bitmask but the kernel typically sets exactly one state bit
// at a time; higher-priority bits win.
// ResetCounters signals that netlink cannot physically zero counters
// in the kernel. Returning iface.ErrCountersNotResettable tells the
// dispatch layer to fall back to a baseline-delta model: capture the
// current values and subtract them from every subsequent read so the
// operator sees "since last clear" deltas. See iface.ErrCountersNotResettable.
func (b *netlinkBackend) ResetCounters(_ string) error {
	return iface.ErrCountersNotResettable
}

func neighStateString(s int) string {
	switch {
	case s&netlink.NUD_PERMANENT != 0:
		return "permanent"
	case s&netlink.NUD_NOARP != 0:
		return "noarp"
	case s&netlink.NUD_REACHABLE != 0:
		return "reachable"
	case s&netlink.NUD_STALE != 0:
		return "stale"
	case s&netlink.NUD_DELAY != 0:
		return "delay"
	case s&netlink.NUD_PROBE != 0:
		return "probe"
	case s&netlink.NUD_FAILED != 0:
		return "failed"
	case s&netlink.NUD_INCOMPLETE != 0:
		return "incomplete"
	}
	return "none"
}
