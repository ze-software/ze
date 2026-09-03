// Design: docs/architecture/resolve.md -- RIR command registration
// Related: rir.go -- the two handlers registered here
//
// The wire methods are declared in the command tree of
// internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang, which maps each one
// to its CLI path. A handler no node names, and a node naming no handler, both
// fail the contract gate in internal/le/docvalid.

package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:resolve-rir", Handler: handleRIRASN},
		pluginserver.RPCRegistration{WireMethod: "ze-update:resolve-rir", Handler: handleRIRRefresh},
	)
}
