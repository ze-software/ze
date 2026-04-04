// Design: plan/spec-iface-0-umbrella.md — Interface RPC handlers for daemon dispatch
//
// Package cmd registers interface RPCs (show, migrate) with the plugin server.
// Separated from the iface package to avoid an import cycle:
// plugin/all -> iface -> plugin/server -> plugin/all.
package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-show", Handler: handleInterfaceShow},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-migrate", Handler: handleInterfaceMigrate},
	)
}

// errResp builds an error response for operational failures. The Go error
// return is nil so the framework uses the Response (not the raw error).
func errResp(msg string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Data: msg}, nil
}

// handleInterfaceShow lists all interfaces or shows one by name.
// Args: optional interface name.
func handleInterfaceShow(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) > 0 {
		name := args[0]
		info, err := iface.GetInterface(name)
		if err != nil {
			return errResp(err.Error())
		}
		data, err := json.Marshal(info)
		if err != nil {
			return nil, fmt.Errorf("interface show: marshal: %w", err)
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
	}

	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return errResp(err.Error())
	}
	data, err := json.Marshal(ifaces)
	if err != nil {
		return nil, fmt.Errorf("interface show: marshal: %w", err)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
}

// handleInterfaceMigrate performs a make-before-break IP migration.
// Accepts --from, --to, --address, --create, and --timeout flags.
func handleInterfaceMigrate(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	bus := iface.GetBus()
	if bus == nil {
		return errResp("interface plugin bus not available")
	}

	cfg, timeout, err := parseMigrateArgs(args)
	if err != nil {
		return errResp(err.Error())
	}

	if err := iface.MigrateInterface(cfg, bus, timeout); err != nil {
		return errResp(err.Error())
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("migration complete: %s -> %s (%s)", cfg.OldIface, cfg.NewIface, cfg.Address),
	}, nil
}

// parseMigrateArgs parses --from/--to/--address/--create/--timeout from args.
func parseMigrateArgs(args []string) (iface.MigrateConfig, time.Duration, error) {
	var cfg iface.MigrateConfig
	timeout := 30 * time.Second

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				return cfg, 0, fmt.Errorf("--from requires a value")
			}
			i++
			name, unit, ok := parseIfaceUnit(args[i])
			if !ok {
				return cfg, 0, fmt.Errorf("invalid --from value %q (expected <name>.<unit>)", args[i])
			}
			cfg.OldIface = name
			cfg.OldUnit = unit
		case "--to":
			if i+1 >= len(args) {
				return cfg, 0, fmt.Errorf("--to requires a value")
			}
			i++
			name, unit, ok := parseIfaceUnit(args[i])
			if !ok {
				return cfg, 0, fmt.Errorf("invalid --to value %q (expected <name>.<unit>)", args[i])
			}
			cfg.NewIface = name
			cfg.NewUnit = unit
		case "--address":
			if i+1 >= len(args) {
				return cfg, 0, fmt.Errorf("--address requires a value")
			}
			i++
			cfg.Address = args[i]
		case "--create":
			if i+1 >= len(args) {
				return cfg, 0, fmt.Errorf("--create requires a value")
			}
			i++
			cfg.NewIfaceType = args[i]
		case "--timeout":
			if i+1 >= len(args) {
				return cfg, 0, fmt.Errorf("--timeout requires a value")
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return cfg, 0, fmt.Errorf("invalid --timeout: %w", err)
			}
			timeout = d
		default:
			return cfg, 0, fmt.Errorf("unknown argument %q", args[i])
		}
	}

	if cfg.OldIface == "" || cfg.NewIface == "" || cfg.Address == "" {
		return cfg, 0, fmt.Errorf("--from, --to, and --address are required")
	}

	return cfg, timeout, nil
}

// parseIfaceUnit splits "<name>.<unit>" into name and unit number.
func parseIfaceUnit(s string) (string, int, bool) {
	idx := strings.LastIndex(s, ".")
	if idx <= 0 || idx == len(s)-1 {
		return "", 0, false
	}

	name := s[:idx]
	unitStr := s[idx+1:]

	unit, err := strconv.Atoi(unitStr)
	if err != nil || unit < 0 {
		return "", 0, false
	}

	return name, unit, true
}
