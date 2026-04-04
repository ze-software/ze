// Design: docs/features/interfaces.md — Interface name validation
// Overview: iface.go — shared types and topic constants

package iface

import (
	"fmt"
	"strings"
)

// Interface name length limits (Linux kernel IFNAMSIZ = 16, including NUL).
const (
	minIfaceNameLen = 1
	maxIfaceNameLen = 15
)

// VLAN ID range per IEEE 802.1Q.
const (
	minVLANID = 1
	maxVLANID = 4094
)

// MTU limits. 68 is the minimum for IPv4 (RFC 791). 16000 is a practical
// upper bound for common virtual/physical NICs (jumbo frames).
const (
	minMTU = 68
	maxMTU = 16000
)

func validateVLANID(id int) error {
	if id < minVLANID || id > maxVLANID {
		return fmt.Errorf("iface: vlan id %d not in [%d, %d]", id, minVLANID, maxVLANID)
	}
	return nil
}

func validateMTU(mtu int) error {
	if mtu < minMTU || mtu > maxMTU {
		return fmt.Errorf("iface: mtu %d not in [%d, %d]", mtu, minMTU, maxMTU)
	}
	return nil
}

// ValidateIfaceName checks that name is a valid Linux interface name.
// Linux kernel forbids '/' and NUL in interface names (IFNAMSIZ).
// We also reject ".." sequences to prevent path traversal in sysctl writes.
// Exported so backend implementations can use it.
func ValidateIfaceName(name string) error {
	n := len(name)
	if n < minIfaceNameLen || n > maxIfaceNameLen {
		return fmt.Errorf("iface: name %q length %d not in [%d, %d]",
			name, n, minIfaceNameLen, maxIfaceNameLen)
	}
	for i := range n {
		c := name[i]
		if c == '/' || c == 0 || c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			return fmt.Errorf("iface: name %q contains forbidden character", name)
		}
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("iface: name %q contains path traversal sequence", name)
	}
	return nil
}
