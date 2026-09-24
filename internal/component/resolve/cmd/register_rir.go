// Design: docs/architecture/resolve.md -- RIR command registration
// Related: rir.go -- the two handlers registered here
// Related: doctor_rir.go -- the delegation-source readiness check registered here
//
// The wire methods are declared in the command tree of
// internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang, which maps each one
// to its CLI path. A handler no node names, and a node naming no handler, both
// fail the contract gate in internal/le/doc/yangcontract.

package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:resolve-rir", Handler: handleRIRASN},
		pluginserver.RPCRegistration{WireMethod: "ze-update:resolve-rir", Handler: handleRIRRefresh},
	)

	// The delegation sources the refresh reads travel with it, so removing
	// this package removes the readiness check that judges them
	// (doctor_rir.go). A refusal is a programmer error in the declaration and
	// stops the process rather than leaving `ze doctor` quietly short of a
	// check.
	if err := diagnostic.RegisterDoctorCheck(rirSourcesDoctorCheck); err != nil {
		panic("BUG: resolve: doctor check registration refused: " + err.Error())
	}
}
