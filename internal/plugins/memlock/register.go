// Design: docs/architecture/plugin/plugin-system.md -- memlock plugin registration
// Overview: memlock.go -- why the executable is locked
// Related: memlock_linux.go -- the init() that takes the lock

//go:build linux

package memlock

import (
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// diagnosticNotLocked names the fault an operator sees in `ze doctor` output.
// Its description lives in internal/core/diagnostic/codes.go, which is what
// `ze explain doctor-memlock-not-locked` reads.
const diagnosticNotLocked = "doctor-memlock-not-locked"

func init() { //nolint:gochecknoinits // plugin registration
	reg := registry.Registration{
		Name:        "memlock",
		Description: "Memory lock: keep the running executable resident under memory pressure",
		RunEngine:   runMemlockPlugin,
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:  "memlock-executable-locked",
			Phase: rpc.DoctorPhasePostConfig,
			// The check reads no config, so it runs first in the plugin band.
			Order:        719,
			Dependencies: []string{"kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{diagnosticNotLocked},
			Check:        checkExecutableLocked,
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
	p := sdk.NewWithConn("memlock", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		fmt.Fprintf(os.Stderr, "memlock: plugin failed: %v\n", err)
		return 1
	}

	return 0
}

// checkExecutableLocked reports a lock this process failed to take. The most
// common cause is RLIMIT_MEMLOCK: the systemd default is 8 MiB and a ze binary
// is several times that, and the whole mapped size is charged even though
// MLOCK_ONFAULT leaves the unread pages unfaulted.
//
// The check reads the doctor process's own lock, which is the daemon's when
// `ze doctor` runs against a daemon on the same limits, and the operator's
// shell otherwise. The remedy it names is the one that fixes both.
func checkExecutableLocked(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	if lockErr == nil {
		return nil
	}

	var tb textbuf.Buffer
	message := tb.Str("the executable is not locked in memory, so its pages can be evicted under memory pressure: ").
		Err(lockErr).
		Str("; raise RLIMIT_MEMLOCK above the size of the ze binary, which the ze.service unit does with LimitMEMLOCK=infinity").
		String()

	return []rpc.DoctorCheckDiagnostic{{
		Code:     diagnosticNotLocked,
		Severity: "warning",
		Message:  message,
	}}
}
