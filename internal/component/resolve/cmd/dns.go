// Design: docs/architecture/api/commands.md -- clear verb RPC registration
// Related: show_dns.go -- show dns cache stats/entries (read-only)

package cmd

import (
	"strings"

	mdns "github.com/miekg/dns"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const (
	clearActionAll    = "all"
	clearActionStats  = "stats"
	clearActionRecord = "record"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-clear:dns-cache", Handler: handleClearDNSCache},
	)
}

func handleClearDNSCache(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.DNS == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "DNS cache not available"}, nil
	}

	action := clearActionAll
	name := ""
	typeName := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case clearActionStats:
			action = clearActionStats
		case clearActionRecord:
			action = clearActionRecord
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		case "type":
			if i+1 < len(args) {
				i++
				typeName = args[i]
			}
		}
	}

	if action == clearActionRecord && name == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear dns cache record: missing name"}, nil
	}

	var result map[string]any

	switch action {
	case clearActionRecord:
		if typeName != "" {
			qtype, ok := mdns.StringToType[strings.ToUpper(typeName)]
			if !ok {
				result = map[string]any{"action": "delete-entry", "error": "unknown type: " + typeName}
			} else {
				found := resolvers.DNS.CacheDelete(name, qtype)
				result = map[string]any{"action": "delete-entry", "name": name, "type": typeName, "found": found}
			}
		} else {
			removed := resolvers.DNS.CacheDeleteByName(name)
			result = map[string]any{"action": "delete-entry", "name": name, "removed": removed}
		}
	case clearActionStats:
		resolvers.DNS.CacheResetStats()
		result = map[string]any{"action": "reset-stats"}
	default:
		resolvers.DNS.CacheClear()
		result = map[string]any{"action": "clear-all"}
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(result)}, nil
}
