// Design: docs/architecture/api/commands.md -- show doctor RPC handler (plugin self-containment)

package cmd

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

// HandleShowDoctor is the RPC handler for ze-show:doctor.
// Registration is deferred until the central show.go entry is removed.
func HandleShowDoctor(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	var configPath string
	if len(args) > 0 {
		configPath = args[0]
	}

	diags := diagnostic.RunDoctorChecks(configPath)
	if diags == nil {
		diags = []diagnostic.Diagnostic{}
	}

	ready := true
	for i := range diags {
		if diags[i].Severity == diagnostic.SeverityError {
			ready = false
			break
		}
	}

	result := diagnostic.NewDoctorResult(ready, diags)
	return &plugin.Response{Status: plugin.StatusDone, Data: result}, nil
}
