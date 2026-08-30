// Design: docs/architecture/api/commands.md -- clear verb RPC registration
// Related: show_dns.go -- show dns cache stats/entries (read-only)

package cmd

import (
	"strings"

	mdns "github.com/miekg/dns"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-clear:dns-cache", Handler: handleClearDNSCache},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:dns-cache-record", Handler: handleClearDNSCacheRecord},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:dns-cache-stats", Handler: handleClearDNSCacheStats},
	)
}

func dnsCacheUnavailableResponse() *plugin.Response {
	if resolvers == nil || resolvers.DNS == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: msgCacheUnavailable}
	}
	return nil
}

func handleClearDNSCache(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resp := dnsCacheUnavailableResponse(); resp != nil {
		return resp, nil
	}
	if len(args) != 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear dns cache: unexpected arguments"}, nil
	}
	resolvers.DNS.CacheClear()
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{keyAction: "clear-all"}}, nil
}

func handleClearDNSCacheStats(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resp := dnsCacheUnavailableResponse(); resp != nil {
		return resp, nil
	}
	if len(args) != 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear dns cache stats: unexpected arguments"}, nil
	}
	resolvers.DNS.CacheResetStats()
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{keyAction: "reset-stats"}}, nil
}

func handleClearDNSCacheRecord(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resp := dnsCacheUnavailableResponse(); resp != nil {
		return resp, nil
	}
	if len(args) == 0 || args[0] == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear dns cache record: missing name"}, nil
	}
	if len(args) != 1 && len(args) != 3 {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear dns cache record: unexpected arguments"}, nil
	}
	name := args[0]
	typeName := ""
	if len(args) == 3 {
		if args[1] != argType {
			return &plugin.Response{Status: plugin.StatusError, Error: "clear dns cache record: expected type <record-type>"}, nil
		}
		typeName = args[2]
	}
	if typeName == "" {
		removed := resolvers.DNS.CacheDeleteByName(name)
		result := plugin.Map{keyAction: actionDeleteEntry, keyName: name, "removed": removed}
		return &plugin.Response{Status: plugin.StatusDone, Data: result}, nil
	}
	qtype, ok := mdns.StringToType[strings.ToUpper(typeName)]
	if !ok {
		result := plugin.Map{keyAction: actionDeleteEntry, "error": "unknown type: " + typeName}
		return &plugin.Response{Status: plugin.StatusDone, Data: result}, nil
	}
	found := resolvers.DNS.CacheDelete(name, qtype)
	result := plugin.Map{keyAction: actionDeleteEntry, keyName: name, keyType: typeName, "found": found}
	return &plugin.Response{Status: plugin.StatusDone, Data: result}, nil
}
