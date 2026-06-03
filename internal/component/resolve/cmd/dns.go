// Design: docs/architecture/api/commands.md -- clear verb RPC registration
// Related: show_dns.go -- show dns cache stats/entries (read-only)

package cmd

import (
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

var dnsCacheClearProvider func(action, name, typeName string) map[string]any

func RegisterDNSCacheClearProvider(fn func(action, name, typeName string) map[string]any) {
	dnsCacheClearProvider = fn
}

func handleClearDNSCache(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if dnsCacheClearProvider == nil {
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

	result := dnsCacheClearProvider(action, name, typeName)
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(result)}, nil
}
