// Design: docs/features/interfaces.md -- Linux-specific bounds (VLAN / MTU)

//go:build linux

package iface

import "fmt"

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
