// Design: docs/architecture/diagnostics/path-mtu.md -- show mtu registration
// Overview: doc.go -- package doc + schema imports
//
// register.go wires the mtu module into the plugin server RPC registry from
// init(), for the daemon-side show mtu handler, and declares the answer shape
// so the pipe layer publishes the operators one document supports.
//
// The module is reached by the daemon through internal/le/plugin/imports/pluginimports.go
// rpcDirs (internal/component/mtu/cmd) and by the `ze` binary through plugin/all.

package cmd

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// wireMethodShowMTU is the one RPC this module registers. The YANG node
// ze:command in ze-mtu-cmd.yang names the same string, which is how the
// dispatcher maps `show mtu` onto handleShowMTU.
const wireMethodShowMTU = "ze-show:mtu"

// commandPathShowMTU is the CLI path the answer shape is declared under.
const commandPathShowMTU = "show mtu"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireMethodShowMTU, Handler: handleShowMTU},
	)

	// The answer is one document holding lists, so every row operator is
	// refused by name before the command runs, and `| table` renders the
	// document as a key-value table with each list as a nested table.
	command.RegisterShape([]string{commandPathShowMTU}, command.ShapeDoc)

	// The local-state readers are a runtime dependency, so ze doctor tries them
	// before the first run (doctor.go). The order sits after the probe layer's
	// socket check (760), which the measurement itself depends on.
	check := diagnostic.DoctorCheck{
		Name:         doctorCheckName,
		Phase:        diagnostic.DoctorPhasePreConfig,
		Order:        770,
		Component:    "mtu",
		Dependencies: []string{"capabilities"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeMTULocalState},
		Check:        checkMTULocalState,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "mtu: doctor check registration: %v\n", err)
		os.Exit(2)
	}
}
