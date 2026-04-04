// Design: docs/features/interfaces.md — OS interface discovery
// Overview: iface.go — shared types and topic constants

package iface

import (
	"fmt"
	"sort"
)

// Ze interface type names matching the YANG schema list/container names.
const (
	zeTypeEthernet = "ethernet"
	zeTypeBridge   = "bridge"
	zeTypeVeth     = "veth"
	zeTypeDummy    = "dummy"
	zeTypeLoopback = "loopback"
)

// DiscoverInterfaces enumerates OS network interfaces and classifies them
// by Ze interface type. Returns only types supported by Ze's YANG schema
// (ethernet, bridge, veth, dummy, loopback), sorted by type then name.
//
// On Linux, interface types are determined from netlink link types.
// On other platforms, loopback is detected by flags; all other interfaces
// with a MAC address are classified as ethernet.
func DiscoverInterfaces() ([]DiscoveredInterface, error) {
	b := GetBackend()
	if b == nil {
		return nil, fmt.Errorf("iface: no backend loaded")
	}
	infos, err := b.ListInterfaces()
	if err != nil {
		return nil, err
	}

	var result []DiscoveredInterface
	for i := range infos {
		zeType := infoToZeType(&infos[i])
		if zeType == "" {
			continue
		}
		result = append(result, DiscoveredInterface{
			Name: infos[i].Name,
			Type: zeType,
			MAC:  infos[i].MAC,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type < result[j].Type
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// infoToZeType maps an InterfaceInfo to a Ze interface type string.
// Returns "" for link types not supported by Ze's YANG schema.
func infoToZeType(info *InterfaceInfo) string {
	// Non-Linux: show_other.go sets Type="loopback" explicitly.
	if info.Type == zeTypeLoopback {
		return zeTypeLoopback
	}
	// Linux: loopback is netlink type "device" with name "lo".
	if info.Type == "device" && info.Name == "lo" {
		return zeTypeLoopback
	}
	switch info.Type {
	case "device":
		return zeTypeEthernet
	case zeTypeBridge:
		return zeTypeBridge
	case zeTypeVeth:
		return zeTypeVeth
	case zeTypeDummy:
		return zeTypeDummy
	}
	// Non-Linux fallback: interface with a real MAC is likely ethernet.
	if info.MAC != "" && info.MAC != "00:00:00:00:00:00" {
		return zeTypeEthernet
	}
	return "" // unsupported or indeterminate type
}
