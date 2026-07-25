// Design: docs/features/interfaces.md — Interface RPC handlers for daemon dispatch
// Related: manage.go — command registration and server wiring for this package
//
// Package cmd registers interface RPCs (migrate) with the plugin server.
// Separated from the iface package to avoid an import cycle:
// plugin/all -> iface -> plugin/server -> plugin/all.
//
// The `show interface` family handlers live alongside in show_interface.go and
// interface_rate.go (ze-show:interface and friends, ze-monitor:interface-rate);
// "show" is a top-level verb ("ze show interface", not "ze interface show"), but
// the handlers are owned here because they read interface state through the
// iface backend. See ai/rules/plugin-self-containment.md.
package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

var (
	errFromRequiresAValue          = errors.New("--from requires a value")
	errToRequiresAValue            = errors.New("--to requires a value")
	errAddressRequiresAValue       = errors.New("--address requires a value")
	errCreateRequiresAValue        = errors.New("--create requires a value")
	errTimeoutRequiresAValue       = errors.New("--timeout requires a value")
	errFromToAndAddressAreRequired = errors.New("--from, --to, and --address are required")
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-create-dummy", Handler: handleCreateDummy},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-create-veth", Handler: handleCreateVeth},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-create-bridge", Handler: handleCreateBridge},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-delete", Handler: handleDelete},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-addr-add", Handler: handleAddrAdd},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-addr-del", Handler: handleAddrDel},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-unit-add", Handler: handleUnitAdd},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-unit-del", Handler: handleUnitDel},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-up", Handler: handleInterfaceUp},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-down", Handler: handleInterfaceDown},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-mtu", Handler: handleInterfaceMTU},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-mac", Handler: handleInterfaceMAC},
		pluginserver.RPCRegistration{WireMethod: "ze-iface:interface-migrate", Handler: handleInterfaceMigrate},
	)
}

// errResp builds an error response for operational failures. The Go error
// return is nil so the framework uses the Response (not the raw error).
func errResp(msg string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil
}

// handleInterfaceMigrate performs a make-before-break IP migration.
// Accepts --from, --to, --address, --create, and --timeout flags.
func handleInterfaceMigrate(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	eb := iface.GetEventBus()
	if eb == nil {
		return errResp("interface plugin event bus not available")
	}

	cfg, timeout, err := parseMigrateArgs(args)
	if err != nil {
		return errResp(err.Error())
	}

	if err := iface.MigrateInterface(cfg, eb, timeout); err != nil {
		return errResp(err.Error())
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"message": "migration complete: " + cfg.OldIface + " -> " + cfg.NewIface + " (" + cfg.Address + ")"},
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
				return cfg, 0, errFromRequiresAValue
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
				return cfg, 0, errToRequiresAValue
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
				return cfg, 0, errAddressRequiresAValue
			}
			i++
			cfg.Address = args[i]
		case "--create":
			if i+1 >= len(args) {
				return cfg, 0, errCreateRequiresAValue
			}
			i++
			cfg.NewIfaceType = args[i]
		case "--timeout":
			if i+1 >= len(args) {
				return cfg, 0, errTimeoutRequiresAValue
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
		return cfg, 0, errFromToAndAddressAreRequired
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
