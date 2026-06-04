// Design: docs/features/interfaces.md -- Interface management RPC handlers
// Related: cmd.go -- Interface migrate handler and registration

package cmd

import (
	"regexp"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

func handleCreateDummy(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" {
		return errResp("usage: create interface dummy <name>")
	}
	if info, _ := iface.GetInterface(name); info != nil {
		if info.Type != "dummy" {
			return errResp("interface " + name + " exists with type " + info.Type + ", not dummy")
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"message": "interface " + name + " already exists", "created": false},
		}, nil
	}
	if err := iface.CreateDummy(name); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "created dummy interface " + name, "created": true},
	}, nil
}

func handleCreateVeth(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" || len(args) == 0 {
		return errResp("usage: create interface veth <name> <peer>")
	}
	peer := args[0]
	if err := iface.CreateVeth(name, peer); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "created veth pair " + name + " <-> " + peer},
	}, nil
}

func handleCreateBridge(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" {
		return errResp("usage: create interface bridge <name>")
	}
	if info, _ := iface.GetInterface(name); info != nil {
		if info.Type != "bridge" {
			return errResp("interface " + name + " exists with type " + info.Type + ", not bridge")
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"message": "interface " + name + " already exists", "created": false},
		}, nil
	}
	if err := iface.CreateBridge(name); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "created bridge interface " + name, "created": true},
	}, nil
}

func handleDelete(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" {
		return errResp("usage: delete interface <name>")
	}
	if err := iface.DeleteInterface(name); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "deleted interface " + name},
	}, nil
}

func handleAddrAdd(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" || len(args) == 0 {
		return errResp("usage: create interface <name> address <prefix>")
	}
	prefix := args[0]
	if err := iface.AddAddress(name, prefix); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "added " + prefix + " to " + name},
	}, nil
}

func handleAddrDel(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" || len(args) == 0 {
		return errResp("usage: delete interface <name> address <prefix>")
	}
	prefix := args[0]
	if err := iface.RemoveAddress(name, prefix); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "removed " + prefix + " from " + name},
	}, nil
}

func handleUnitAdd(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" || len(args) == 0 {
		return errResp("usage: create interface <name> unit <vid>")
	}
	vidStr := args[0]
	vid, parseErr := strconv.Atoi(vidStr)
	if parseErr != nil || vid < 1 || vid > 4094 {
		return errResp("invalid VLAN ID " + vidStr + " (must be 1-4094)")
	}
	if err := iface.CreateVLAN(name, vid); err != nil {
		return errResp(err.Error())
	}
	var bData textbuf.Buffer
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": bData.Reset().Str("created unit ").Str(name).Byte('.').Int(int64(vid)).String()},
	}, nil
}

func handleUnitDel(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" {
		return errResp("usage: delete interface <name> unit")
	}
	if err := iface.DeleteInterface(name); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "deleted unit " + name},
	}, nil
}

// handleInterfaceUp brings an interface administratively up.
func handleInterfaceUp(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" {
		return errResp("usage: request interface <name> up")
	}
	if err := iface.SetAdminUp(name); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "interface " + name + " up"},
	}, nil
}

// handleInterfaceDown brings an interface administratively down.
func handleInterfaceDown(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" {
		return errResp("usage: request interface <name> down")
	}
	if err := iface.SetAdminDown(name); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "interface " + name + " down"},
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
func handleInterfaceMTU(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" || len(args) == 0 {
		return errResp("usage: request interface <name> mtu <bytes>")
	}
	mtu, parseErr := strconv.Atoi(args[0])
	if parseErr != nil {
		var b textbuf.Buffer
		return errResp(b.Reset().Str("invalid MTU ").Str(args[0]).Str(": ").Str(parseErr.Error()).String())
	}
	if mtu < MTUMin || mtu > MTUMax {
		var bMsg textbuf.Buffer
		return errResp(bMsg.Reset().Str("MTU ").Int(int64(mtu)).Str(" out of range ").Int(int64(MTUMin)).Str("..").Int(int64(MTUMax)).String())
	}
	if err := iface.SetMTU(name, mtu); err != nil {
		return errResp(err.Error())
	}
	var bMtu textbuf.Buffer
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": bMtu.Reset().Str("interface ").Str(name).Str(" mtu ").Int(int64(mtu)).String()},
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
func handleInterfaceMAC(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ctx.Selector("name")
	if name == "" || len(args) == 0 {
		return errResp("usage: request interface <name> mac <address>")
	}
	addr := args[0]
	if !IsValidMACAddress(addr) {
		return errResp("invalid MAC address " + addr + " (expected xx:xx:xx:xx:xx:xx)")
	}
	if err := iface.SetMACAddress(name, addr); err != nil {
		return errResp(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "interface " + name + " mac " + addr},
	}, nil
}
