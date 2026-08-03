// Design: docs/features/interfaces.md -- plugin-owned devices (macvlan)
// Related: backend.go -- Backend.CreateMacvlanDevice consumes MacvlanSpec
// Related: address_owner.go -- sibling plugin-owned address registry (the model)

package iface

import (
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// MacvlanMode selects the macvlan delivery mode. The zero value is bridge, so
// existing callers that leave it unset keep the historical behavior.
type MacvlanMode uint8

const (
	// MacvlanModeBridge lets siblings on one parent reach each other and floods
	// broadcast/multicast between them. The default (zero value).
	MacvlanModeBridge MacvlanMode = iota
	// MacvlanModePrivate isolates siblings from each other. A consumer that wants
	// the kernel to answer ARP/ND for an address from the macvlan's own MAC rather
	// than the parent needs this mode: in bridge mode the parent can win the
	// ARP-flux race and reply with its real MAC.
	MacvlanModePrivate
)

// Wire names for the macvlan delivery modes, as the kernel and `ip link` spell
// them. Deliberately NOT reusing discover.go's zeTypeBridge, which happens to
// share the spelling: that constant names an interface DEVICE TYPE
// (ethernet/veth/bridge/...), while these name a macvlan MODE. Coupling them
// would mean a future rename of the device type silently changed a macvlan
// mode string that the kernel defines.
const (
	macvlanModeNameBridge  = "bridge"
	macvlanModeNamePrivate = "private"
)

// String is the canonical mode name, matching what the netlink backend reads
// back from the kernel (show_linux.go) so the reconcile drift check can compare
// a live device's mode against the desired one.
func (m MacvlanMode) String() string {
	if m == MacvlanModePrivate {
		return macvlanModeNamePrivate
	}
	return macvlanModeNameBridge
}

// MacvlanSpec carries the parameters for a macvlan device that a plugin owns
// through the owned-device registry (device_owner.go). Like WireguardSpec
// (wireguard.go) it is plain data passed by value; the backend consumes it via
// Backend.CreateMacvlanDevice.
//
// Mode selects bridge (zero value) or private delivery. iface treats MAC as an
// opaque validated unicast address: the virtual-MAC values are chosen by the
// caller, never spelled here.
//
// Alias is the kernel IFLA_IFALIAS ownership marker written at create time
// ("ze:owned:<owner>"). It is set by the owned-device reconcile pass, NOT by
// callers of RegisterOwnedMacvlan (they leave it empty); the pass derives it
// from the registering owner so orphan detection can read ownership back from
// the kernel without in-memory history.
type MacvlanSpec struct {
	Name   string      // device name; compose via ComposeOwnedDeviceName (<=15 chars)
	Parent string      // OS device name of the parent interface
	MAC    string      // unicast, non-zero hardware address the device carries
	Mode   MacvlanMode // bridge (default) or private delivery
	Alias  string      // ownership marker set by the reconcile pass, not the caller
}

// validate checks a caller-supplied MacvlanSpec: the name passes
// ValidateIfaceName (length/charset/reserved), the parent is non-empty, and
// the MAC parses to a non-zero unicast address. Alias is ignored here (the
// reconcile pass sets it after registration).
func (s MacvlanSpec) validate() error {
	if err := ValidateIfaceName(s.Name); err != nil {
		return fmt.Errorf("iface: macvlan name: %w", err)
	}
	if s.Parent == "" {
		return fmt.Errorf("iface: macvlan %q: parent is empty", s.Name)
	}
	if _, err := parseUnicastMAC(s.MAC); err != nil {
		return fmt.Errorf("iface: macvlan %q: %w", s.Name, err)
	}
	return nil
}

// parseUnicastMAC parses mac and rejects it unless it is a non-zero unicast
// address. The multicast/group bit is the least-significant bit of the first
// octet (IEEE 802); an all-zero MAC is not a valid device address.
func parseUnicastMAC(mac string) (net.HardwareAddr, error) {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return nil, fmt.Errorf("mac %q: %w", mac, err)
	}
	if len(hw) == 0 {
		return nil, fmt.Errorf("mac %q is empty", mac)
	}
	if hw[0]&0x01 != 0 {
		return nil, fmt.Errorf("mac %q is multicast, must be unicast", mac)
	}
	allZero := true
	for _, b := range hw {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil, fmt.Errorf("mac %q is all-zero", mac)
	}
	return hw, nil
}

// macEqual reports whether two MAC strings denote the same hardware address,
// tolerating case/format differences by parsing both. Falls back to a string
// compare only when either side does not parse (which validation prevents for
// registered specs). Used by the reconcile drift check.
func macEqual(a, b string) bool {
	ha, ea := net.ParseMAC(a)
	hb, eb := net.ParseMAC(b)
	if ea != nil || eb != nil {
		return a == b
	}
	if len(ha) != len(hb) {
		return false
	}
	for i := range ha {
		if ha[i] != hb[i] {
			return false
		}
	}
	return true
}

// ComposeOwnedDeviceName builds a deterministic, collision-free owned-device
// name of the form "<prefix>-<parentIfindex>-<id>" that fits the 15-char
// IFNAMSIZ budget enforced by ValidateIfaceName (validate.go). It is the ONE
// place the budget math and the reject-not-truncate rule live, so callers
// (which pass a short prefix such as "zv4"/"zv6") keep zero name-length logic
// and iface keeps zero consumer-specific knowledge.
//
// The ifindex makes the name unique per parent at any instant; ifindex churn
// across reboots is harmless because owned devices are runtime state recreated
// each boot (see the spec's Key Design Decisions). A candidate that would
// exceed the 15-char limit is REJECTED naming the limit and the candidate --
// never truncated (ai/rules/protocol.md). parentIfindex must be
// positive (0 = "no parent" in netlink) and id non-negative.
func ComposeOwnedDeviceName(prefix string, parentIfindex, id int) (string, error) {
	if prefix == "" {
		return "", fmt.Errorf("iface: owned device name prefix is empty")
	}
	if parentIfindex <= 0 {
		return "", fmt.Errorf("iface: owned device parent ifindex %d must be positive", parentIfindex)
	}
	if id < 0 {
		return "", fmt.Errorf("iface: owned device id %d must be non-negative", id)
	}
	var b textbuf.Buffer
	name := b.Str(prefix).Byte('-').Int(int64(parentIfindex)).Byte('-').Int(int64(id)).String()
	if len(name) > maxIfaceNameLen {
		return "", fmt.Errorf("iface: composed owned device name %q exceeds %d-char limit (no truncation)", name, maxIfaceNameLen)
	}
	// Belt-and-suspenders: reuse the full interface-name validation so a bad
	// prefix (forbidden char, reserved keyword) is rejected in one place too.
	if err := ValidateIfaceName(name); err != nil {
		return "", err
	}
	return name, nil
}
