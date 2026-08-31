// Design: docs/architecture/plugin/plugin-system.md -- memlock plugin registration
// Overview: memlock.go -- why the executable is locked
// Related: memlock_linux.go -- the init() that takes the lock and records the outcome
// Related: doctor_linux.go -- the pre-flight check this registration declares

//go:build linux

package memlock

import (
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// pluginName is the one spelling of this plugin's name. The registration and
// the setup record are two writes keyed by it, so they MUST agree.
const pluginName = "memlock"

func init() { //nolint:gochecknoinits // plugin registration
	// The doctor check is a PRE-FLIGHT probe of the host, not a report of the
	// lock. `ze doctor` runs in the operator's own process, so it can never say
	// whether the DAEMON took its lock; `show plugins` answers that, by
	// replaying what the daemon's own init() recorded. What the doctor check
	// adds is the tier before ze runs: whether this host could lock the
	// executable at all. See doctor_linux.go.
	reg := registry.Registration{
		Name:        pluginName,
		Description: "Memory lock: keep the running executable resident under memory pressure",
		RunEngine:   runMemlockPlugin,
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "memlock-rlimit",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        726,
			Dependencies: []string{"kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{codeMemlockRlimitLow, codeMemlockRlimitUnknown},
			Check:        checkMemlockLimit,
		}},
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
