// Design: docs/architecture/plugin/plugin-system.md -- memlock plugin registration
// Overview: memlock.go -- why the executable is locked
// Related: memlock_linux.go -- the init() that takes the lock and records the outcome

//go:build linux

package memlock

import (
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// pluginName is the one spelling of this module's name. The registration and
// the setup record are two writes keyed by it, so they MUST agree.
const pluginName = "memlock"

func init() { //nolint:gochecknoinits // plugin registration
	// No doctor check. `ze doctor` runs in the operator's own process, so it
	// reports THAT process's lock and not the daemon's, and it is optional, so
	// it cannot carry a fact the daemon depends on. The lock outcome is
	// recorded by memlock_linux.go and read by `show module list`.
	reg := registry.Registration{
		Name:        pluginName,
		Description: "Memory lock: keep the running executable resident under memory pressure",
		RunEngine:   runMemlockPlugin,
	}
	reg.CLIHandler = func(_ []string) int {
		return 1
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "memlock: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// runMemlockPlugin serves the engine handshake and idles. It takes no lock:
// the lock belongs to init(), which every ze process runs whether or not this
// engine run is ever reached.
//
// The registry requires an engine run from every plugin, and this one declares
// no config root, no family and no event type, so nothing auto-loads it. An
// operator who names memlock in a `plugin { }` block gets this.
func runMemlockPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		fmt.Fprintf(os.Stderr, "memlock: plugin failed: %v\n", err)
		return 1
	}

	return 0
}
