// Design: docs/architecture/resolve.md -- resolve command handlers for dispatcher
//
// Package cmd registers resolve operations as dispatcher commands.
// Handlers use a package-level *Resolvers set via SetResolvers() at hub startup.
// Once registered, resolve commands appear as auto-generated MCP tools.
package cmd

import (
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve"

	// Blank import triggers YANG schema registration.
	_ "github.com/ze-software/ze/internal/component/resolve/yang"
)

// argType is the CLI argument keyword that introduces a DNS record type.
const argType = "type"

// The response payload keys. keyType holds the same text as argType and names
// something else: one is what an operator types, the other is what the JSON
// carries.
const (
	keyAction  = "action"
	keyCount   = "count"
	keyEntries = "entries"
	keyName    = "name"
	keyRecords = "records"
	keyType    = "type"
)

// actionDeleteEntry is the action a cache-record deletion reports.
const actionDeleteEntry = "delete-entry"

// msgCacheUnavailable is what every handler reports when the DNS cache is absent.
const msgCacheUnavailable = "DNS cache not available"

// resolvers holds the shared resolver instances. Set once at hub startup
// via SetResolvers, read by handler functions. Safe because SetResolvers
// is called before the dispatcher starts accepting requests.
var resolvers *resolve.Resolvers

// SetResolvers sets the shared resolver instances for command handlers.
// MUST be called before the dispatcher starts accepting requests.
func SetResolvers(r *resolve.Resolvers) {
	resolvers = r
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:dns-a", Handler: handleDNSA},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:dns-aaaa", Handler: handleDNSAAAA},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:dns-txt", Handler: handleDNSTXT},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:dns-ptr", Handler: handleDNSPTR},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:cymru-asn-name", Handler: handleCymruASNName},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:peeringdb-max-prefix", Handler: handlePeeringDBMaxPrefix},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:peeringdb-as-set", Handler: handlePeeringDBASSet},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:irr-expand", Handler: handleIRRExpand},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:irr-prefix", Handler: handleIRRPrefix},
	)
}

func requireArg(args []string, name string) (string, *plugin.Response) {
	if len(args) == 0 {
		return "", &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: resolve ... <" + name + ">",
		}
	}
	return args[0], nil
}

func requireASN(args []string) (uint32, *plugin.Response) {
	s, errResp := requireArg(args, "asn")
	if errResp != nil {
		return 0, errResp
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("invalid ASN %q: %v", s, err),
		}
	}
	return uint32(n), nil
}

func errResponse(msg string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil
}

func dnsResult(records []string, resolveErr error) (*plugin.Response, error) {
	if resolveErr != nil {
		return errResponse(resolveErr.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{keyRecords: records},
	}, nil
}

// DNS handlers.

func handleDNSA(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.DNS == nil {
		return errResponse("DNS resolver not available")
	}
	name, errResp := requireArg(args, "hostname")
	if errResp != nil {
		return errResp, nil
	}
	records, err := resolvers.DNS.ResolveA(name)
	return dnsResult(records, err)
}

func handleDNSAAAA(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.DNS == nil {
		return errResponse("DNS resolver not available")
	}
	name, errResp := requireArg(args, "hostname")
	if errResp != nil {
		return errResp, nil
	}
	records, err := resolvers.DNS.ResolveAAAA(name)
	return dnsResult(records, err)
}

func handleDNSTXT(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.DNS == nil {
		return errResponse("DNS resolver not available")
	}
	name, errResp := requireArg(args, "hostname")
	if errResp != nil {
		return errResp, nil
	}
	records, err := resolvers.DNS.ResolveTXT(name)
	return dnsResult(records, err)
}

func handleDNSPTR(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.DNS == nil {
		return errResponse("DNS resolver not available")
	}
	addr, errResp := requireArg(args, "address")
	if errResp != nil {
		return errResp, nil
	}
	records, err := resolvers.DNS.ResolvePTR(addr)
	return dnsResult(records, err)
}

// Cymru handler.

func handleCymruASNName(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.Cymru == nil {
		return errResponse("Cymru resolver not available")
	}
	asn, errResp := requireASN(args)
	if errResp != nil {
		return errResp, nil
	}
	name, err := resolvers.Cymru.LookupASNName(ctx.Context(), asn)
	if err != nil {
		return errResponse(err.Error())
	}
	if name == "" {
		name = "(unknown)"
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{keyName: name}}, nil
}

// PeeringDB handlers.

func handlePeeringDBMaxPrefix(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.PeeringDB == nil {
		return errResponse("PeeringDB client not available")
	}
	asn, errResp := requireASN(args)
	if errResp != nil {
		return errResp, nil
	}
	counts, err := resolvers.PeeringDB.LookupASN(ctx.Context(), asn)
	if err != nil {
		return errResponse(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"asn":  asn,
			"ipv4": counts.IPv4,
			"ipv6": counts.IPv6,
		},
	}, nil
}

func handlePeeringDBASSet(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.PeeringDB == nil {
		return errResponse("PeeringDB client not available")
	}
	asn, errResp := requireASN(args)
	if errResp != nil {
		return errResp, nil
	}
	sets, err := resolvers.PeeringDB.LookupASSet(ctx.Context(), asn)
	if err != nil {
		return errResponse(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"sets": sets},
	}, nil
}

// IRR handlers.

func handleIRRExpand(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.IRR == nil {
		return errResponse("IRR client not available")
	}
	asSet, errResp := requireArg(args, "as-set")
	if errResp != nil {
		return errResp, nil
	}
	asns, err := resolvers.IRR.ResolveASSet(ctx.Context(), asSet)
	if err != nil {
		return errResponse(err.Error())
	}
	parts := make([]string, len(asns))
	for i, a := range asns {
		parts[i] = strconv.FormatUint(uint64(a), 10)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"members": parts},
	}, nil
}

func handleIRRPrefix(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.IRR == nil {
		return errResponse("IRR client not available")
	}
	asSet, errResp := requireArg(args, "as-set")
	if errResp != nil {
		return errResp, nil
	}
	prefixes, err := resolvers.IRR.LookupPrefixes(ctx.Context(), asSet)
	if err != nil {
		return errResponse(err.Error())
	}
	var lines []string
	for _, p := range prefixes.IPv4 {
		lines = append(lines, p.String())
	}
	for _, p := range prefixes.IPv6 {
		lines = append(lines, p.String())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"prefixes": lines},
	}, nil
}
