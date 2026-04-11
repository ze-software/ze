// Design: docs/architecture/resolve.md -- resolve command handlers for dispatcher
//
// Package cmd registers resolve operations as dispatcher commands.
// Handlers use a package-level *Resolvers set via SetResolvers() at hub startup.
// Once registered, resolve commands appear as auto-generated MCP tools.
package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve"

	// Blank import triggers YANG schema registration.
	_ "codeberg.org/thomas-mangin/ze/internal/component/resolve/cmd/schema"
)

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
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:ping", Handler: handlePing},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:traceroute", Handler: handleTraceroute},
	)
}

func requireArg(args []string, name string) (string, *plugin.Response) {
	if len(args) == 0 {
		return "", &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("usage: resolve ... <%s>", name),
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
			Data:   fmt.Sprintf("invalid ASN %q: %v", s, err),
		}
	}
	return uint32(n), nil
}

func errResponse(msg string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Data: msg}, nil
}

func dnsResult(records []string, resolveErr error) (*plugin.Response, error) {
	if resolveErr != nil {
		return errResponse(resolveErr.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   strings.Join(records, "\n"),
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

func handleCymruASNName(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.Cymru == nil {
		return errResponse("Cymru resolver not available")
	}
	asn, errResp := requireASN(args)
	if errResp != nil {
		return errResp, nil
	}
	name, err := resolvers.Cymru.LookupASNName(context.Background(), asn)
	if err != nil {
		return errResponse(err.Error())
	}
	if name == "" {
		name = "(unknown)"
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: name}, nil
}

// PeeringDB handlers.

func handlePeeringDBMaxPrefix(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.PeeringDB == nil {
		return errResponse("PeeringDB client not available")
	}
	asn, errResp := requireASN(args)
	if errResp != nil {
		return errResp, nil
	}
	counts, err := resolvers.PeeringDB.LookupASN(context.Background(), asn)
	if err != nil {
		return errResponse(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"asn":  asn,
			"ipv4": counts.IPv4,
			"ipv6": counts.IPv6,
		},
	}, nil
}

func handlePeeringDBASSet(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.PeeringDB == nil {
		return errResponse("PeeringDB client not available")
	}
	asn, errResp := requireASN(args)
	if errResp != nil {
		return errResp, nil
	}
	sets, err := resolvers.PeeringDB.LookupASSet(context.Background(), asn)
	if err != nil {
		return errResponse(err.Error())
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   strings.Join(sets, "\n"),
	}, nil
}

// IRR handlers.

func handleIRRExpand(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.IRR == nil {
		return errResponse("IRR client not available")
	}
	asSet, errResp := requireArg(args, "as-set")
	if errResp != nil {
		return errResp, nil
	}
	asns, err := resolvers.IRR.ResolveASSet(context.Background(), asSet)
	if err != nil {
		return errResponse(err.Error())
	}
	parts := make([]string, len(asns))
	for i, a := range asns {
		parts[i] = strconv.FormatUint(uint64(a), 10)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   strings.Join(parts, "\n"),
	}, nil
}

func handleIRRPrefix(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if resolvers == nil || resolvers.IRR == nil {
		return errResponse("IRR client not available")
	}
	asSet, errResp := requireArg(args, "as-set")
	if errResp != nil {
		return errResp, nil
	}
	prefixes, err := resolvers.IRR.LookupPrefixes(context.Background(), asSet)
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
		Data:   strings.Join(lines, "\n"),
	}, nil
}

// handlePing runs an ICMP ping to the target address.
// Syntax: resolve ping <target> [source <ip>] [count <n>] [size <bytes>].
func handlePing(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	target, errResp := requireArg(args, "target")
	if errResp != nil {
		return errResp, nil
	}

	cmdArgs := []string{"-c", "4", "-W", "2"}

	// Parse optional args.
	for i := 1; i < len(args); i++ {
		if i+1 >= len(args) {
			break
		}
		switch args[i] {
		case "source":
			i++
			cmdArgs = append(cmdArgs, "-I", args[i])
		case "count":
			i++
			cmdArgs = append(cmdArgs, "-c", args[i])
		case "size":
			i++
			cmdArgs = append(cmdArgs, "-s", args[i])
		}
	}
	cmdArgs = append(cmdArgs, target)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	out, err := execCommand(ctx, "ping", cmdArgs...)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: map[string]any{
				"target": target,
				"output": string(out),
				"error":  err.Error(),
			},
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"target": target,
			"output": string(out),
		},
	}, nil
}

// handleTraceroute runs a traceroute to the target address.
// Syntax: resolve traceroute <target> [source <ip>].
func handleTraceroute(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	target, errResp := requireArg(args, "target")
	if errResp != nil {
		return errResp, nil
	}

	cmdArgs := []string{"-n", "-w", "2", "-q", "1"}

	// Parse optional source arg.
	for i := 1; i < len(args); i++ {
		if args[i] == "source" && i+1 < len(args) {
			i++
			cmdArgs = append(cmdArgs, "-s", args[i])
		}
	}
	cmdArgs = append(cmdArgs, target)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := execCommand(ctx, "traceroute", cmdArgs...)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: map[string]any{
				"target": target,
				"output": string(out),
				"error":  err.Error(),
			},
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"target": target,
			"output": string(out),
		},
	}, nil
}

// execCommand runs an OS command with context timeout and returns combined output.
//
// Security note: `name` is a fixed allowlist (`ping`, `traceroute`) picked by
// the caller -- never user-controlled. `args` may contain operator-supplied
// values (target, source, count, size) that are NOT validated today, but
// `exec.CommandContext` does not invoke a shell so there is no injection
// channel: every element of `args` is passed as a discrete argv[N] to the
// child process. The worst an operator can do is make ping/traceroute reject
// a malformed argument with its own error.
//
// TODO: validate `args` format (IP/hostname for target, digit strings for
// count/size) so operator mistakes surface as ze errors rather than opaque
// ping output.
func execCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec // fixed-allowlist `name`, exec.CommandContext bypasses the shell so args are argv literals
}
