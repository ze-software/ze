// Design: plan/spec-flow-export-1-counter-export.md -- show flow-export CLI command

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/flowexport"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:flow-export",
			Handler:    handleShowFlowExport,
		},
	)
}

func handleShowFlowExport(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	exp := flowexport.GetExporter()
	if exp == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"status": "not-configured"},
		}, nil
	}

	collectors := exp.Status()
	if len(args) > 0 {
		name := args[0]
		for _, c := range collectors {
			if n, ok := c["name"].(string); ok && n == name {
				return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(c)}, nil
			}
		}
		return &plugin.Response{Status: plugin.StatusError, Error: "collector not found: " + name}, nil
	}

	result := make([]plugin.Map, 0, len(collectors))
	for _, c := range collectors {
		result = append(result, plugin.Map(c))
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[plugin.Map](result)}, nil
}
