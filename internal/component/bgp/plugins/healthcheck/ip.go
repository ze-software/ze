// Design: plan/spec-healthcheck-0-umbrella.md -- IP management via iface
// Overview: healthcheck.go -- plugin lifecycle and probe management
package healthcheck

import "codeberg.org/thomas-mangin/ze/internal/component/iface"

// ipManager defines the interface for IP address management on network interfaces.
// Production uses iface.AddAddress/RemoveAddress; tests inject a mock.
type ipManager interface {
	AddAddress(ifaceName, cidr string) error
	RemoveAddress(ifaceName, cidr string) error
}

// realIPManager wraps the iface package standalone functions.
type realIPManager struct{}

func (realIPManager) AddAddress(ifaceName, cidr string) error {
	return iface.AddAddress(ifaceName, cidr)
}

func (realIPManager) RemoveAddress(ifaceName, cidr string) error {
	return iface.RemoveAddress(ifaceName, cidr)
}

// ipTracker manages VIPs for a single probe.
type ipTracker struct {
	mgr       ipManager
	ifaceName string
	ips       []string        // CIDRs configured for this probe
	added     map[string]bool // CIDRs this tracker has added
}

func newIPTracker(mgr ipManager, ifaceName string, ips []string) *ipTracker {
	return &ipTracker{
		mgr:       mgr,
		ifaceName: ifaceName,
		ips:       ips,
		added:     make(map[string]bool),
	}
}

// addAll adds all configured IPs to the interface.
func (t *ipTracker) addAll() {
	for _, cidr := range t.ips {
		if t.added[cidr] {
			continue
		}
		if err := t.mgr.AddAddress(t.ifaceName, cidr); err != nil {
			logger().Warn("ip add failed", "iface", t.ifaceName, "cidr", cidr, "error", err)
			continue
		}
		t.added[cidr] = true
	}
}

// removeAll removes all IPs that this tracker added.
func (t *ipTracker) removeAll() {
	for cidr := range t.added {
		if err := t.mgr.RemoveAddress(t.ifaceName, cidr); err != nil {
			logger().Warn("ip remove failed", "iface", t.ifaceName, "cidr", cidr, "error", err)
			continue
		}
		delete(t.added, cidr)
	}
}
