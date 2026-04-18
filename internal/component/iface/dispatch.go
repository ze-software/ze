// Design: docs/features/interfaces.md -- Backend dispatch functions
// Overview: iface.go -- shared types and topic constants
// Related: backend.go -- Backend interface and registry

package iface

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// backendOrErr returns the active backend or an error if none is loaded.
func backendOrErr() (Backend, error) {
	b := GetBackend()
	if b == nil {
		return nil, fmt.Errorf("iface: no backend loaded")
	}
	return b, nil
}

// Package-level functions that delegate to the active backend.

func CreateDummy(name string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.CreateDummy(name)
}
func CreateVeth(name, peer string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.CreateVeth(name, peer)
}
func CreateBridge(name string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.CreateBridge(name)
}
func CreateVLAN(parent string, vid int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.CreateVLAN(parent, vid)
}
func DeleteInterface(name string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.DeleteInterface(name)
}
func AddAddress(iface, cidr string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.AddAddress(iface, cidr)
}
func RemoveAddress(iface, cidr string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.RemoveAddress(iface, cidr)
}

func AddRoute(ifaceName, destCIDR, gateway string, metric int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.AddRoute(ifaceName, destCIDR, gateway, metric)
}

func RemoveRoute(ifaceName, destCIDR, gateway string, metric int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.RemoveRoute(ifaceName, destCIDR, gateway, metric)
}

func ListRoutes(ifaceName, destCIDR string) ([]RouteInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return b.ListRoutes(ifaceName, destCIDR)
}

// ListNeighbors returns the kernel neighbor table via the active backend.
// family is one of NeighborFamilyAny / NeighborFamilyIPv4 / NeighborFamilyIPv6.
func ListNeighbors(family int) ([]NeighborInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return b.ListNeighbors(family)
}

// ListKernelRoutes returns up to `limit` entries from the kernel's
// routing table via the active backend. filterPrefix (non-empty)
// narrows the dump to a single CIDR. limit == 0 means unbounded.
func ListKernelRoutes(filterPrefix string, limit int) ([]KernelRoute, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return b.ListKernelRoutes(filterPrefix, limit)
}

// ResetCounters zeros RX/TX counters for the named interface (or every
// managed interface when name == "") via the active backend. Backends
// that cannot physically clear counters in the kernel (Linux netlink)
// trigger a baseline-delta fallback: the current values become a
// per-interface baseline and GetStats/ListInterfaces/GetInterface
// subtract that baseline before returning. Wrap detection (raw < baseline)
// automatically rebases the baseline to zero so a subsequent kernel-level
// reset does not poison the delta view. See counters.go.
func ResetCounters(name string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return resetCountersViaBackend(b, name)
}

func ReplaceAddressWithLifetime(ifaceName, cidr string, validLft, preferredLft int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.ReplaceAddressWithLifetime(ifaceName, cidr, validLft, preferredLft)
}

func SetAdminUp(iface string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetAdminUp(iface)
}
func SetAdminDown(iface string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetAdminDown(iface)
}
func SetMTU(iface string, mtu int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetMTU(iface, mtu)
}
func SetMACAddress(iface, mac string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetMACAddress(iface, mac)
}

func GetMACAddress(iface string) (string, error) {
	b, err := backendOrErr()
	if err != nil {
		return "", err
	}
	return b.GetMACAddress(iface)
}

func GetStats(iface string) (*InterfaceStats, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	s, err := b.GetStats(iface)
	if err != nil {
		return nil, err
	}
	baselines.applyBaseline(iface, s)
	return s, nil
}

func ListInterfaces() ([]InterfaceInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	ifs, err := b.ListInterfaces()
	if err != nil {
		return nil, err
	}
	for i := range ifs {
		baselines.applyBaseline(ifs[i].Name, ifs[i].Stats)
	}
	return ifs, nil
}

func GetInterface(name string) (*InterfaceInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	info, err := b.GetInterface(name)
	if err != nil {
		return nil, err
	}
	if info != nil {
		baselines.applyBaseline(info.Name, info.Stats)
	}
	return info, nil
}

func BridgeAddPort(bridge, port string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.BridgeAddPort(bridge, port)
}
func BridgeDelPort(port string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.BridgeDelPort(port)
}
func BridgeSetSTP(bridge string, on bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.BridgeSetSTP(bridge, on)
}

func SetupMirror(src, dst string, ingress, egress bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetupMirror(src, dst, ingress, egress)
}

func RemoveMirror(src string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.RemoveMirror(src)
}

// Monitor wraps the backend's monitoring capability.
type Monitor struct {
	backend  Backend
	eventBus ze.EventBus
}

// NewMonitor creates a monitor via the active backend.
func NewMonitor(eb ze.EventBus) (*Monitor, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return &Monitor{backend: b, eventBus: eb}, nil
}

// Start begins monitoring via the backend.
func (m *Monitor) Start() error {
	return m.backend.StartMonitor(m.eventBus)
}

// Stop halts monitoring via the backend.
func (m *Monitor) Stop() {
	m.backend.StopMonitor()
}
