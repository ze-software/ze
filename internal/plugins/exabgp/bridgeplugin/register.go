// Register the internal exabgp-bridge plugin with the plugin registry.
//
// This lives under internal/plugins/exabgp/ (not a top-level plugin dir) so the
// engine-boundary lint that confines ExaBGP naming to the compat tree stays
// satisfied, while still being discovered by the plugin/all generator (which
// walks internal/plugins recursively). The sibling CLI package
// (internal/plugins/exabgp) is excluded from plugin/all as a generator marker;
// this subpackage is the runtime plugin.

package bridgeplugin

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	bridgeyang "github.com/ze-software/ze/internal/plugins/exabgp/bridgeplugin/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// errRunRequired is returned when the exabgp.bridge container is committed
// without a `run` command (nothing for the bridge to launch).
var errRunRequired = errors.New("exabgp-bridge: 'run' is required to start the bridge")

var loggerPtr atomic.Pointer[slog.Logger]

// logger returns the plugin's engine logger (a discard logger until the engine
// wires the real one via ConfigureEngineLogger).
func logger() *slog.Logger { return loggerPtr.Load() }

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	reg := registry.Registration{
		Name:                    "exabgp-bridge",
		Description:             "In-process ExaBGP compatibility bridge: runs an operator ExaBGP-format script as a subprocess and translates to/from ze events (RFC-agnostic transport shim)",
		Features:                "yang",
		YANG:                    bridgeyang.ZeExabgpBridgeConfYANG,
		ConfigRoots:             []string{configRoot},
		InProcessConfigVerifier: verifyConfig,
		RunEngine:               runInternalBridge,
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	reg.ConfigureEngineLogger = func(loggerName string) {
		if l := slogutil.Logger(loggerName); l != nil {
			loggerPtr.Store(l)
		}
	}

	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "exabgp-bridge: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// verifyConfig is the offline verifier: it parses and validates the committed
// exabgp.bridge config without spawning anything.
func verifyConfig(sections []rpc.ConfigSection) error {
	for _, s := range sections {
		if s.Root != configRoot {
			continue
		}
		if _, err := parseConfig(s.Data); err != nil {
			return err
		}
	}
	return nil
}
