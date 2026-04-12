// Design: docs/architecture/api/commands.md -- show policy introspection handlers
// Related: show.go -- show verb RPC registration
//
// Policy introspection commands: list filter types/instances, show peer chains.
// Read-only queries with no state mutation.

package show

import (
	"fmt"
	"net/netip"
	"sort"
	"strconv"

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
	matched := filterPeersByPolicySelector(allPeers, selector)

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
		if direction != "import" && direction != "export" {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("invalid direction %q (expected import or export)", direction),
			}, nil
		}
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

// filterPeersByPolicySelector returns peers matching the selector.
// Supports: "*" (all), IP address (parsed), peer name (string), ASN ("as65001").
// Mirrors the logic in bgp/plugins/cmd/peer/peer.go:filterPeersBySelector
// without importing that package.
func filterPeersByPolicySelector(peers []plugin.PeerInfo, selector string) []plugin.PeerInfo {
	if selector == "*" {
		return peers
	}

	// Try parsed IP address match.
	if filterIP, err := netip.ParseAddr(selector); err == nil {
		for i := range peers {
			if peers[i].Address == filterIP {
				return []plugin.PeerInfo{peers[i]}
			}
		}
		return nil
	}

	// Try peer name match.
	for i := range peers {
		if peers[i].Name == selector {
			return []plugin.PeerInfo{peers[i]}
		}
	}

	// Try ASN selector: "as<N>" (case-insensitive).
	if len(selector) > 2 && (selector[0] == 'a' || selector[0] == 'A') && (selector[1] == 's' || selector[1] == 'S') {
		if asn, err := strconv.ParseUint(selector[2:], 10, 32); err == nil {
			var matched []plugin.PeerInfo
			for i := range peers {
				if uint64(peers[i].PeerAS) == asn {
					matched = append(matched, peers[i])
				}
			}
			return matched
		}
	}

	return nil
}
