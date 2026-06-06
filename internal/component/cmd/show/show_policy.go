// Design: docs/architecture/api/commands.md -- show policy list handler (cross-plugin)

package show

import (
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:policy-list",
			Handler:    handleShowPolicyList,
		},
	)
}

func handleShowPolicyList(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	typesMap := registry.FilterTypesMap()

	type filterTypeEntry struct {
		Type   string `json:"type"`
		Plugin string `json:"plugin"`
	}

	entries := make([]filterTypeEntry, 0, len(typesMap))
	for filterType, pluginName := range typesMap {
		entries = append(entries, filterTypeEntry{
			Type:   filterType,
			Plugin: pluginName,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Type < entries[j].Type })

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"filter-types": entries,
			"count":        len(entries),
		},
	}, nil
}
