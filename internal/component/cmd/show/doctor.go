// Design: plan/spec-doctor-improvements.md -- AC-14 show doctor from SSH CLI

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func handleShowDoctor(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
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
