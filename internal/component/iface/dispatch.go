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

func AddRoute(ifaceName, destCIDR, gateway string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.AddRoute(ifaceName, destCIDR, gateway)
}

func RemoveRoute(ifaceName, destCIDR, gateway string) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.RemoveRoute(ifaceName, destCIDR, gateway)
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
	return b.GetStats(iface)
}

func ListInterfaces() ([]InterfaceInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return b.ListInterfaces()
}

func GetInterface(name string) (*InterfaceInfo, error) {
	b, err := backendOrErr()
	if err != nil {
		return nil, err
	}
	return b.GetInterface(name)
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

func SetIPv4Forwarding(iface string, on bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv4Forwarding(iface, on)
}
func SetIPv4ArpFilter(iface string, on bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv4ArpFilter(iface, on)
}
func SetIPv4ArpAccept(iface string, on bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv4ArpAccept(iface, on)
}
func SetIPv4ProxyARP(iface string, on bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv4ProxyARP(iface, on)
}
func SetIPv4ArpAnnounce(iface string, lvl int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv4ArpAnnounce(iface, lvl)
}
func SetIPv4ArpIgnore(iface string, lvl int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv4ArpIgnore(iface, lvl)
}
func SetIPv4RPFilter(iface string, lvl int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv4RPFilter(iface, lvl)
}
func SetIPv6Autoconf(iface string, on bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv6Autoconf(iface, on)
}
func SetIPv6AcceptRA(iface string, lvl int) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv6AcceptRA(iface, lvl)
}
func SetIPv6Forwarding(iface string, on bool) error {
	b, err := backendOrErr()
	if err != nil {
		return err
	}
	return b.SetIPv6Forwarding(iface, on)
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

// EnableSLAAC enables IPv6 SLAAC on an interface.
func EnableSLAAC(iface string) error { return SetIPv6Autoconf(iface, true) }

// DisableSLAAC disables IPv6 SLAAC on an interface.
func DisableSLAAC(iface string) error { return SetIPv6Autoconf(iface, false) }

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
