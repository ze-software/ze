// Design: docs/architecture/api/process-protocol.md -- owned interface devices and firewall table contribution
// Related: fakeowner.go -- the requests the plugin makes and the outcome it serves

package fakeowner

import (
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Name is the plugin's registry name. It is also the owner the interface and
// firewall components read from the authenticated process, so a forked
// fakeowner MUST be declared under this name.
const Name = "fakeowner"

// devicePrefix is the device name prefix the plugin grants itself.
const devicePrefix = "zfo"

// tableName is the firewall table the plugin contributes, as the section
// names it. The kernel name carries the ze_ prefix.
const tableName = "fakeowner"

// kernelTableName is the kernel name of tableName, the one reserved.
const kernelTableName = "ze_" + tableName

func init() {
	// The grant and the reservation are made from init(), so every process of
	// the ze binary holds them, the forked plugin's and the daemon's alike.
	iface.MustGrantOwnedDeviceNames(Name, devicePrefix)
	firewall.MustReserveTableNames(Name, firewall.TableControl{
		Plugin:  "fakeowner test plugin",
		Decides: "the fakeowner plugin's fixed test table",
		Command: "ze show firewall ruleset",
	}, kernelTableName)

	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only owner of an interface bridge, a link and a firewall table, through the engine operations",
		RunEngine:   runPlugin,
		Commands:    commandDecls(),
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		if len(args) > 0 && args[0] == holdNamespaceArg {
			return holdNamespace()
		}
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}
