// Design: docs/features/interfaces.md -- XFRM interface types
// Related: wireguard.go -- WireguardSpec follows the same standalone-struct pattern
// Related: backend.go -- Backend.CreateXFRM and Backend.GetXFRMInfo

package iface

// XFRMSpec carries the parsed configuration for a single XFRM interface.
// XFRM interfaces (Linux 4.19+) bind to the kernel XFRM subsystem via if_id
// for route-based IPsec. The PhysicalDev field is optional; when empty, the
// interface is unbound and uses the routing table to select the underlay.
type XFRMSpec struct {
	Name        string
	IfID        uint32
	PhysicalDev string
}

// XFRMPolicyInfo describes a single XFRM policy bound to an interface's if_id.
type XFRMPolicyInfo struct {
	Src   string
	Dst   string
	Dir   string
	Proto string
	Mode  string
}

// XFRMInfo carries runtime information about an XFRM interface queried from
// netlink. Used by show commands to display if_id and bound policies even
// for interfaces not managed by Ze's config. ParentDev is the resolved name
// of the parent device (empty when unbound). Addresses are the IP addresses
// assigned to the interface, used by ze init to emit a complete config block
// when onboarding an externally-created XFRM interface.
type XFRMInfo struct {
	IfID      uint32
	ParentDev string
	Addresses []string
	Policies  []XFRMPolicyInfo
}
