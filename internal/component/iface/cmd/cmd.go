// Design: docs/features/interfaces.md — Interface RPC handlers for daemon dispatch
// Related: manage.go — command registration and server wiring for this package
//
// Package cmd registers interface RPCs (migrate) with the plugin server.
// Separated from the iface package to avoid an import cycle:
// plugin/all -> iface -> plugin/server -> plugin/all.
//
// The `show interface` family handlers live alongside in show_interface.go and
// interface_rate.go (ze-show:interface-*, ze-monitor:interface-rate);
// "show" is a top-level verb ("ze show interface", not "ze interface show"), but
// the handlers are owned here because they read interface state through the
// iface backend. See ai/rules/plugins.md.
package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// Keys in the plugin.Map payload these handlers return. The CLI renderers and
// the JSON output both read them, so they are an operator-facing surface.
const (
	fieldInterfaces = "interfaces"
	fieldMessage    = "message"
	fieldName       = "name"
)

// Keywords `request interface migrate` takes, each one before its own value.
// Container migrate of internal/component/iface/yang/ze-iface-cmd.yang declares
// the same five as modifier groups, so completion, the published usage line and
// this parser all state one grammar.
const (
	migrateKeywordFrom    = "from"
	migrateKeywordTo      = "to"
	migrateKeywordAddress = "address"
	migrateKeywordCreate  = "create"
	migrateKeywordTimeout = "timeout"
)

// migrateKeywords is the closed set a token is tested against before it is
// read as a keyword. A token that is in neither this set nor a keyword's value
// slot is refused, never ignored.
var migrateKeywords = []string{
	migrateKeywordFrom,
	migrateKeywordTo,
	migrateKeywordAddress,
	migrateKeywordCreate,
	migrateKeywordTimeout,
}

// migrateRequired lists the keywords the command cannot run without, in the
// order an operator types them.
var migrateRequired = []string{migrateKeywordFrom, migrateKeywordTo, migrateKeywordAddress}

// migrateGrammar is the form every refusal quotes back, so an operator reads
// what was expected without opening the schema.
const migrateGrammar = "from <name>.<unit> to <name>.<unit> address <cidr> [create <dummy|veth|bridge>] [timeout <duration>]"

// migrateTimeoutDefault is how long phase 3 waits for BGP readiness on the
// destination address when the operator names no timeout.
const migrateTimeoutDefault = 30 * time.Second

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
// Reads the keyword grammar migrateGrammar states.
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
		Data:   plugin.Map{fieldMessage: "migration complete: " + cfg.OldIface + " -> " + cfg.NewIface + " (" + cfg.Address + ")"},
	}, nil
}

// parseMigrateArgs reads the keyword/value pairs of `request interface migrate`
// and answers the migration they describe plus the BGP readiness wait.
//
// Every token is one of migrateKeywords or the value of the keyword before it.
// A token that is neither is REFUSED rather than skipped: an operator who
// misspells a keyword must be told, not handed a migration that moves an
// address they did not name.
//
// The loop is bounded by the token count the dispatcher parsed from one command
// line, and it steps by two because a keyword and its value are one unit.
func parseMigrateArgs(args []string) (iface.MigrateConfig, time.Duration, error) {
	var cfg iface.MigrateConfig
	timeout := migrateTimeoutDefault
	seen := make(map[string]bool, len(migrateKeywords))

	for i := 0; i < len(args); i += 2 {
		keyword := args[i]
		if !slices.Contains(migrateKeywords, keyword) {
			return cfg, 0, fmt.Errorf("unknown keyword %q, expected: %s", keyword, migrateGrammar)
		}
		if seen[keyword] {
			return cfg, 0, fmt.Errorf("keyword %q given twice, expected: %s", keyword, migrateGrammar)
		}
		if i+1 >= len(args) {
			return cfg, 0, fmt.Errorf("keyword %q has no value, expected: %s", keyword, migrateGrammar)
		}
		seen[keyword] = true

		value := args[i+1]
		switch keyword {
		case migrateKeywordFrom:
			name, unit, ok := parseIfaceUnit(value)
			if !ok {
				return cfg, 0, fmt.Errorf("invalid %s value %q, expected <name>.<unit>", keyword, value)
			}
			cfg.OldIface = name
			cfg.OldUnit = unit
		case migrateKeywordTo:
			name, unit, ok := parseIfaceUnit(value)
			if !ok {
				return cfg, 0, fmt.Errorf("invalid %s value %q, expected <name>.<unit>", keyword, value)
			}
			cfg.NewIface = name
			cfg.NewUnit = unit
		case migrateKeywordAddress:
			cfg.Address = value
		case migrateKeywordCreate:
			cfg.NewIfaceType = value
		case migrateKeywordTimeout:
			d, err := time.ParseDuration(value)
			if err != nil {
				return cfg, 0, fmt.Errorf("invalid %s value %q, expected a number and a unit such as 30s: %w", keyword, value, err)
			}
			timeout = d
		}
	}

	for _, keyword := range migrateRequired {
		if !seen[keyword] {
			return cfg, 0, fmt.Errorf("keyword %q is missing, expected: %s", keyword, migrateGrammar)
		}
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
