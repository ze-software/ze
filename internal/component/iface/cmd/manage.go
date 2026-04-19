// Design: docs/features/interfaces.md -- Interface management RPC handlers
// Related: cmd.go -- Interface migrate handler and registration

package cmd

import (
	"fmt"
	"regexp"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func handleCreateDummy(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return errResp("usage: interface create-dummy <name>")
	}
	if err := iface.CreateDummy(args[0]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("created dummy interface %s", args[0]),
	}, nil
}

func handleCreateVeth(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 2 {
		return errResp("usage: interface create-veth <name> <peer>")
	}
	if err := iface.CreateVeth(args[0], args[1]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("created veth pair %s <-> %s", args[0], args[1]),
	}, nil
}

func handleCreateBridge(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return errResp("usage: interface create-bridge <name>")
	}
	if err := iface.CreateBridge(args[0]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("created bridge interface %s", args[0]),
	}, nil
}

func handleDelete(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return errResp("usage: interface delete <name>")
	}
	if err := iface.DeleteInterface(args[0]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("deleted interface %s", args[0]),
	}, nil
}

func handleAddrAdd(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 2 {
		return errResp("usage: interface addr-add <name> <cidr>")
	}
	if err := iface.AddAddress(args[0], args[1]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("added %s to %s", args[1], args[0]),
	}, nil
}

func handleAddrDel(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 2 {
		return errResp("usage: interface addr-del <name> <cidr>")
	}
	if err := iface.RemoveAddress(args[0], args[1]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("removed %s from %s", args[1], args[0]),
	}, nil
}

func handleUnitAdd(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 2 {
		return errResp("usage: interface unit-add <name> <vlan-id>")
	}
	vid, parseErr := strconv.Atoi(args[1])
	if parseErr != nil || vid < 1 || vid > 4094 {
		return errResp(fmt.Sprintf("invalid VLAN ID %q (must be 1-4094)", args[1]))
	}
	if err := iface.CreateVLAN(args[0], vid); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("created unit %s.%d", args[0], vid),
	}, nil
}

func handleUnitDel(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return errResp("usage: interface unit-del <name.unit>")
	}
	if err := iface.DeleteInterface(args[0]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("deleted unit %s", args[0]),
	}, nil
}

// handleInterfaceUp brings an interface administratively up.
func handleInterfaceUp(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return errResp("usage: interface up <name>")
	}
	if err := iface.SetAdminUp(args[0]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("interface %s up", args[0]),
	}, nil
}

// handleInterfaceDown brings an interface administratively down.
func handleInterfaceDown(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return errResp("usage: interface down <name>")
	}
	if err := iface.SetAdminDown(args[0]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("interface %s down", args[0]),
	}, nil
}

// MTU bounds per Linux (net/core/dev.c MTU checks): IPv6 requires 1280+,
// but link-layer minimum is 68 (IPv4 minimum). Maximum is IP_MAX_MTU (65535).
// Names MTUMin/MTUMax are used so callers can cite the bound in errors.
const (
	// MTUMin is the smallest MTU we accept. 68 is the IPv4 minimum from
	// RFC 791; kernels reject lower values for IPv4 interfaces.
	MTUMin = 68
	// MTUMax is the largest MTU representable in the 16-bit uint link
	// attribute (IP_MAX_MTU in the kernel).
	MTUMax = 65535
)

// handleInterfaceMTU sets the MTU on an interface. Validates the
// requested MTU is within MTUMin..MTUMax before calling the backend;
// returning a range error here keeps the message consistent regardless
// of how the backend would have phrased its own rejection.
func handleInterfaceMTU(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 2 {
		return errResp("usage: interface mtu <name> <mtu>")
	}
	mtu, parseErr := strconv.Atoi(args[1])
	if parseErr != nil {
		return errResp(fmt.Sprintf("invalid MTU %q: %v", args[1], parseErr))
	}
	if mtu < MTUMin || mtu > MTUMax {
		return errResp(fmt.Sprintf("MTU %d out of range %d..%d", mtu, MTUMin, MTUMax))
	}
	if err := iface.SetMTU(args[0], mtu); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("interface %s mtu %d", args[0], mtu),
	}, nil
}

// macAddressRegexp matches the canonical xx:xx:xx:xx:xx:xx MAC form.
// Duplicates internal/component/config/validators.go macPattern intentionally
// to avoid pulling the validators package (and its YANG dependency chain)
// into the iface/cmd RPC surface; the two sites should be kept in sync if
// the accepted MAC format changes.
var macAddressRegexp = regexp.MustCompile(`^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$`)

// IsValidMACAddress reports whether s is a canonical colon-separated
// 48-bit MAC address ("xx:xx:xx:xx:xx:xx", hex, case-insensitive).
// Exposed so the offline CLI (cmd/ze/iface) validates input with the
// same rule as the daemon-side handler.
func IsValidMACAddress(s string) bool {
	return macAddressRegexp.MatchString(s)
}

// handleInterfaceMAC sets the MAC address on an interface. Validates
// the MAC format before calling the backend; malformed input rejects
// with a clear error rather than passing through to a backend syscall
// that returns a less specific EINVAL.
func handleInterfaceMAC(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 2 {
		return errResp("usage: interface mac <name> <mac>")
	}
	if !IsValidMACAddress(args[1]) {
		return errResp(fmt.Sprintf("invalid MAC address %q (expected xx:xx:xx:xx:xx:xx)", args[1]))
	}
	if err := iface.SetMACAddress(args[0], args[1]); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("interface %s mac %s", args[0], args[1]),
	}, nil
}
