// Design: docs/architecture/api/commands.md — doctor command registration

package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// init registers show doctor, the daemon form of `ze doctor`: one handler over
// the producer `ze doctor` runs locally (doctor.Answer). doctor is an offline
// root, not a verb, so it carries no daemon path of its own; its local
// registration names show doctor as that path (registry.Meta.DaemonPath).
func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-doctor:show-doctor",
			Handler:    HandleDoctor,
		},
	)
}
