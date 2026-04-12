// Design: docs/architecture/api/commands.md -- show policy introspection handlers
// Related: show.go -- show verb RPC registration
//
// Policy introspection commands: list filter types/instances, show peer chains.
// Read-only queries with no state mutation.

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
		pluginserver.RPCRegistration{
			WireMethod:       "ze-show:policy-chain",
			Handler:          handleShowPolicyChain,
			RequiresSelector: true,
		},
	)
}

// handleShowPolicyList returns all registered filter types and their plugin names.
// Used by `show policy list`.
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
		Data: map[string]any{
			"filter-types": entries,
			"count":        len(entries),
		},
	}, nil
}

// handleShowPolicyChain returns the effective import/export filter chains for
// a peer after group inheritance. Used by `show policy chain peer X [import|export]`.
func handleShowPolicyChain(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if ctx.Reactor() == nil {
		return &plugin.Response{Status: plugin.StatusError, Data: "reactor not available"}, nil
	}

	allPeers := ctx.Reactor().Peers()
	selector := ctx.PeerSelector()

	var matched []plugin.PeerInfo
	for i := range allPeers {
		p := &allPeers[i]
		if selector == "*" || p.Name == selector || p.Address.String() == selector {
			matched = append(matched, *p)
		}
	}

	if len(matched) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "peer not found: " + selector,
		}, nil
	}

	// Optional direction filter from args.
	direction := ""
	if len(args) > 0 {
		direction = args[0]
	}

	type peerChain struct {
		Peer   string   `json:"peer"`
		Name   string   `json:"name,omitempty"`
		Import []string `json:"import,omitempty"`
		Export []string `json:"export,omitempty"`
	}

	chains := make([]peerChain, 0, len(matched))
	for i := range matched {
		p := &matched[i]
		entry := peerChain{
			Peer: p.Address.String(),
			Name: p.Name,
		}
		if direction == "" || direction == "import" {
			entry.Import = p.ImportFilters
		}
		if direction == "" || direction == "export" {
			entry.Export = p.ExportFilters
		}
		chains = append(chains, entry)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"chains": chains,
		},
	}, nil
}
