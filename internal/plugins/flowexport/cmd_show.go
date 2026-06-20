// Design: docs/architecture/api/commands.md — show flow-export handler.
// Owned by the flow-export component so that removing it removes the
// `show flow-export` command, its schema, and this handler together. See
// ai/rules/plugin-self-containment.md.

package flowexport

import (
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
	exp := GetExporter()
	if exp == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"status": "not-configured"},
		}, nil
	}

	collectors := exp.Status()
	switch len(args) {
	case 0:
		result := make([]plugin.Map, 0, len(collectors))
		for _, c := range collectors {
			result = append(result, plugin.Map(c))
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[plugin.Map](result)}, nil
	case 2:
		if args[0] == "name" {
			name := args[1]
			for _, c := range collectors {
				if n, ok := c["name"].(string); ok && n == name {
					return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(c)}, nil
				}
			}
			return &plugin.Response{Status: plugin.StatusError, Error: "collector not found: " + name}, nil
		}
	}
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "usage: show flow-export [name <name>]",
	}, nil
}
