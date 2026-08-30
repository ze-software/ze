// Design: docs/features/interfaces.md — OS interface discovery
// Overview: iface.go — shared types and topic constants
// Related: validators.go -- MAC address CompleteFn for config validator registry

package iface

import (
	"sort"
)

// Ze interface type names matching the YANG schema list/container names.
const (
	zeTypeEthernet  = "ethernet"
	zeTypeBridge    = "bridge"
	zeTypeVeth      = "veth"
	zeTypeDummy     = "dummy"
	zeTypeLoopback  = "loopback"
	zeTypeTunnel    = "tunnel"
	zeTypeWireguard = "wireguard"
	zeTypeXFRM      = "xfrm"
	// zeTypeMacvlan is the kernel link type for macvlan devices. It is
	// deliberately NOT a YANG-configurable interface type and NOT in
	// zeManageable (config_apply.go): macvlans are plugin-owned devices
	// (device_owner.go), created/deleted via the owned-device registry and
	// guarded by the "ze:owned:" alias marker, never by the Phase 4 prune.
	zeTypeMacvlan = "macvlan"
)

// SupportedTypes returns the canonical list of Ze interface type names
// matching the YANG schema list/container names. Used by UI components
// that need to enumerate interface types without hardcoding the list.
func SupportedTypes() []string {
	return []string{
		zeTypeEthernet,
		zeTypeBridge,
		zeTypeVeth,
		zeTypeDummy,
		zeTypeTunnel,
		zeTypeWireguard,
		zeTypeXFRM,
		zeTypeLoopback,
	}
}

// kernelTunnelKinds is the set of Linux netlink link types reported by the
// kernel for the tunnel encapsulation kinds Ze supports. Used to map kernel
// types into the single "tunnel" zeType (the YANG list is one tunnel list
// regardless of encapsulation), and by zeManageable (config_apply.go) to say
// which link types Ze creates and deletes.
//
// It must hold every value of kernelLinkTypes (tunnel.go), which is the same
// mapping the other way. TestKernelTunnelKindsCoversEveryModeledKind pins the
// two together: "vxlan" was missing here from the day the kind was modeled, so
// Ze created a vxlan and then refused to delete it, and a vxlan on the host was
// classified as ethernet by the MAC fallback at the end of infoToZeType.
var kernelTunnelKinds = map[string]bool{
	kernelLinkGRE:       true,
	kernelLinkGRETap:    true,
	kernelLinkIP6GRE:    true,
	kernelLinkIP6GRETap: true,
	kernelLinkIPIP:      true,
	kernelLinkSIT:       true,
	kernelLinkIP6Tnl:    true,
	kernelLinkVxlan:     true,
}

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
		return nil, errIfaceNoBackendLoaded
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
		di := DiscoveredInterface{
			Name: infos[i].Name,
			Type: zeType,
			MAC:  infos[i].MAC,
		}
		if zeType == zeTypeWireguard {
			// Read per-device state via wgctrl so ze init can emit a
			// complete wireguard config block for a manually-created
			// netdev. Errors (missing wireguard module, insufficient
			// perms) are non-fatal: the entry is still discovered, the
			// emitter simply falls back to a skeleton with just the
			// interface name.
			if spec, specErr := b.GetWireguardDevice(infos[i].Name); specErr == nil {
				s := spec
				di.Wireguard = &s
			}
		}
		if zeType == zeTypeXFRM {
			if info, infoErr := b.GetXFRMInfo(infos[i].Name); infoErr == nil {
				di.XFRM = &info
			}
		}
		result = append(result, di)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type < result[j].Type
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// CanonicalInterfaceType maps an InterfaceInfo to a Ze/YANG interface type.
// It returns "" for link types not represented by Ze's interface schema.
func CanonicalInterfaceType(info *InterfaceInfo) string {
	return infoToZeType(info)
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
	if kernelTunnelKinds[info.Type] {
		return zeTypeTunnel
	}
	if info.Type == zeTypeWireguard {
		return zeTypeWireguard
	}
	if info.Type == zeTypeXFRM {
		return zeTypeXFRM
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
