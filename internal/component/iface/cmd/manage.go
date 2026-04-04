// Design: docs/features/interfaces.md -- Interface management RPC handlers
// Related: cmd.go -- Interface migrate handler and registration

package cmd

import (
	"fmt"
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
