// Design: plan/spec-iface-0-umbrella.md — Non-Linux interface listing via stdlib
// Overview: iface.go — shared types and topic constants

//go:build !linux

package iface

import (
	"fmt"
	"net"
)

// ListInterfaces returns all OS network interfaces using the Go standard library.
// On non-Linux platforms, type and stats are not available.
func ListInterfaces() ([]InterfaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("iface: list interfaces: %w", err)
	}

	result := make([]InterfaceInfo, 0, len(ifaces))
	for _, ifc := range ifaces {
		info := stdlibToInfo(ifc)

		addrs, addrErr := ifc.Addrs()
		if addrErr == nil {
			info.Addresses = stdlibAddrs(addrs)
		}

		result = append(result, info)
	}
	return result, nil
}

// GetInterface returns info for a single interface by name.
func GetInterface(name string) (*InterfaceInfo, error) {
	if err := validateIfaceName(name); err != nil {
		return nil, err
	}

	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("iface: get %q: %w", name, err)
	}

	info := stdlibToInfo(*ifc)

	addrs, addrErr := ifc.Addrs()
	if addrErr == nil {
		info.Addresses = stdlibAddrs(addrs)
	}

	return &info, nil
}

// stdlibToInfo converts a net.Interface to InterfaceInfo.
func stdlibToInfo(ifc net.Interface) InterfaceInfo {
	state := "down"
	if ifc.Flags&net.FlagUp != 0 {
		state = "up"
	}

	info := InterfaceInfo{
		Name:  ifc.Name,
		Index: ifc.Index,
		State: state,
		MTU:   ifc.MTU,
	}

	if len(ifc.HardwareAddr) > 0 {
		info.MAC = ifc.HardwareAddr.String()
	}

	if ifc.Flags&net.FlagLoopback != 0 {
		info.Type = "loopback"
	}

	return info
}

// stdlibAddrs converts net.Addr slice to AddrInfo slice.
func stdlibAddrs(addrs []net.Addr) []AddrInfo {
	result := make([]AddrInfo, 0, len(addrs))
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		family := "ipv4"
		if ipNet.IP.To4() == nil {
			family = "ipv6"
		}
		ones, _ := ipNet.Mask.Size()
		result = append(result, AddrInfo{
			Address:      ipNet.IP.String(),
			PrefixLength: ones,
			Family:       family,
		})
	}
	return result
}
