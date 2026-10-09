// Design: docs/features/interfaces.md -- Non-Linux interface listing via the standard library
// Overview: ifacenetlink.go -- package hub
// Related: backend_other.go -- the stub backend these methods hang off

//go:build !linux

package ifacenetlink

import (
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/component/iface"
)

// ListInterfaces lists the OS interfaces through the Go standard library. It
// is the one read the stub answers, because `ze init` discovers interfaces
// with it on a developer machine. The standard library reports no link kind,
// so Type is set only for a loopback and iface classifies the rest by MAC.
func (s *stubBackend) ListInterfaces() ([]iface.InterfaceInfo, error) {
	ifcs, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("iface: list interfaces: %w", err)
	}
	result := make([]iface.InterfaceInfo, 0, len(ifcs))
	for i := range ifcs {
		info, infoErr := stdlibInterfaceInfo(&ifcs[i])
		if infoErr != nil {
			return nil, infoErr
		}
		result = append(result, info)
	}
	return result, nil
}

// GetInterface returns one OS interface through the Go standard library.
func (s *stubBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	if err := iface.ValidateIfaceName(name); err != nil {
		return nil, err
	}
	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("iface: get %q: %w", name, err)
	}
	info, err := stdlibInterfaceInfo(ifc)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// stdlibInterfaceInfo converts one standard-library interface, addresses
// included. A failed address read is returned rather than dropped, so an
// interface is never reported as holding no address when it was not read.
func stdlibInterfaceInfo(ifc *net.Interface) (iface.InterfaceInfo, error) {
	state := "down"
	if ifc.Flags&net.FlagUp != 0 {
		state = "up"
	}
	info := iface.InterfaceInfo{
		Name:   ifc.Name,
		OsName: ifc.Name,
		Index:  ifc.Index,
		State:  state,
		MTU:    ifc.MTU,
	}
	if len(ifc.HardwareAddr) > 0 {
		info.MAC = ifc.HardwareAddr.String()
	}
	if ifc.Flags&net.FlagLoopback != 0 {
		info.Type = "loopback"
	}
	addrs, err := ifc.Addrs()
	if err != nil {
		return iface.InterfaceInfo{}, fmt.Errorf("iface: addresses of %q: %w", ifc.Name, err)
	}
	info.Addresses = stdlibAddrInfo(addrs)
	return info, nil
}

// stdlibAddrInfo converts the standard library's interface addresses. Only
// *net.IPNet carries a prefix; any other net.Addr names no interface address.
func stdlibAddrInfo(addrs []net.Addr) []iface.AddrInfo {
	result := make([]iface.AddrInfo, 0, len(addrs))
	for _, addr := range addrs {
		prefix, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		family := "ipv4"
		if prefix.IP.To4() == nil {
			family = "ipv6"
		}
		ones, _ := prefix.Mask.Size()
		result = append(result, iface.AddrInfo{
			Address:      prefix.IP.String(),
			PrefixLength: ones,
			Family:       family,
			LinkLocal:    family == "ipv6" && prefix.IP.IsLinkLocalUnicast(),
		})
	}
	return result
}
