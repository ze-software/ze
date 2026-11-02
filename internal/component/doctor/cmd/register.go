// Design: docs/architecture/api/commands.md — doctor command registration

package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// init registers the daemon form of doctor under both of its paths. The two
// RPCs are one handler over the producer `ze doctor` runs locally
// (doctor.Answer), so `doctor` and `show doctor` answer alike on the daemon.
func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-doctor:show-doctor",
			Handler:    HandleDoctor,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-doctor:doctor",
			Handler:    HandleDoctor,
		},
	)
}
