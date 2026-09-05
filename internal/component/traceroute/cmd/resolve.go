// Design: docs/architecture/resolve.md -- resolve traceroute command handler
// Related: traceroute.go -- doTracerouteCtx internal ICMP engine shared with show traceroute

package cmd

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/probe"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errResolveTargetEmpty = errors.New("target must not be empty")

// tracerouteRequest is the option set `resolve traceroute` parses out of its
// arguments, before any of it reaches the network.
type tracerouteRequest struct {
	source  netip.Addr
	maxHops int
	timeout time.Duration
	probes  int
}

// handleResolveTraceroute is the RPC handler for `resolve traceroute`
// (ze-resolve:traceroute): ICMP traceroute with optional source binding.
func handleResolveTraceroute(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	target, errResp := requireResolveArg(args, "target")
	if errResp != nil {
		return errResp, nil
	}
	if err := validateResolveTarget(target); err != nil {
		return errResolveResponse(err.Error()), nil
	}

	req, errResp := parseResolveTracerouteArgs(args)
	if errResp != nil {
		return errResp, nil
	}

	// The source address decides the family the target resolves in. A source is
	// bindable only on a socket of its own family, so a target resolved in the
	// other family would fail at the bind naming neither argument.
	family := probe.FamilyOf(req.source)
	dest, err := probe.ResolveTarget(target, family)
	if err != nil {
		var tb textbuf.Buffer
		if errors.Is(err, probe.ErrFamilyMismatch) {
			tb.Str("traceroute: source ").Str(req.source.String()).Str(" is ").Str(family.String())
			tb.Str(" but target ").Str(strconv.Quote(target)).Str(" has no ").Str(family.String()).Str(" address")
			return errResolveResponse(tb.String()), nil
		}
		tb.Str("traceroute: invalid target ").Str(strconv.Quote(target)).Str(": ").Err(err)
		return errResolveResponse(tb.String()), nil
	}

	hops, trErr := doTracerouteCtx(ctx.Context(), dest, req.maxHops, req.timeout, req.probes, tracerouteOpts{source: req.source})
	if trErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: trErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{
		fieldHops: hops,
	}}, nil
}

// parseResolveTracerouteArgs reads the keyword-value options that follow the
// target. It runs before the target is resolved, because the source it reads
// decides which address family the target is resolved in.
func parseResolveTracerouteArgs(args []string) (tracerouteRequest, *plugin.Response) {
	req := tracerouteRequest{
		maxHops: defaultTracerouteMaxHops,
		timeout: defaultTracerouteTimeout,
		probes:  defaultTracerouteProbes,
	}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "source":
			if i+1 >= len(args) {
				return req, errResolveResponse("traceroute: \"source\" requires a value")
			}
			i++
			source, err := parseSourceIP(args[i])
			if err != nil {
				return req, errResolveResponse(err.Error())
			}
			req.source = source
		case "max-hops":
			if i+1 >= len(args) {
				return req, errResolveResponse("traceroute: \"max-hops\" requires a value")
			}
			i++
			n, parseErr := strconv.Atoi(args[i])
			if parseErr != nil {
				return req, errResolveResponse("traceroute: max-hops requires a number")
			}
			if n < 1 || n > maxTracerouteMaxHops {
				var tb textbuf.Buffer
				tb.Str("traceroute: max-hops must be 1-").Int(int64(maxTracerouteMaxHops))
				return req, errResolveResponse(tb.String())
			}
			req.maxHops = n
		case argTimeout:
			if i+1 >= len(args) {
				return req, errResolveResponse("traceroute: \"timeout\" requires a value (e.g. 2s)")
			}
			i++
			d, parseErr := time.ParseDuration(args[i])
			if parseErr != nil {
				return req, errResolveResponse("traceroute: timeout requires a duration (e.g. 2s)")
			}
			if d < time.Second || d > maxTracerouteTimeout {
				var tb textbuf.Buffer
				tb.Str("traceroute: timeout must be 1s-").Str(maxTracerouteTimeout.String())
				return req, errResolveResponse(tb.String())
			}
			req.timeout = d
		case "probes":
			if i+1 >= len(args) {
				return req, errResolveResponse("traceroute: \"probes\" requires a value")
			}
			i++
			n, parseErr := strconv.Atoi(args[i])
			if parseErr != nil {
				return req, errResolveResponse("traceroute: probes requires a number")
			}
			if n < 1 || n > maxTracerouteProbes {
				var tb textbuf.Buffer
				tb.Str("traceroute: probes must be 1-").Int(int64(maxTracerouteProbes))
				return req, errResolveResponse(tb.String())
			}
			req.probes = n
		default:
			var tb textbuf.Buffer
			tb.Str("traceroute: unknown option ").Str(strconv.Quote(args[i]))
			return req, errResolveResponse(tb.String())
		}
	}
	return req, nil
}

// requireResolveArg returns args[0] or a usage error response.
func requireResolveArg(args []string, name string) (string, *plugin.Response) {
	if len(args) == 0 {
		var tb textbuf.Buffer
		tb.Str("usage: resolve ... <").Str(name).Byte('>')
		return "", &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.String(),
		}
	}
	return args[0], nil
}

func errResolveResponse(msg string) *plugin.Response {
	return &plugin.Response{Status: plugin.StatusError, Error: msg}
}

// validateResolveTarget accepts an IP literal or a hostname of letters, digits,
// dot, and hyphen up to the RFC 1035 length ceiling.
func validateResolveTarget(s string) error {
	if s == "" {
		return errResolveTargetEmpty
	}
	if net.ParseIP(s) != nil {
		return nil
	}
	if len(s) > 253 {
		var tb textbuf.Buffer
		tb.Str("target ").Str(strconv.Quote(s)).Str(": exceeds 253-character hostname limit")
		return errors.New(tb.String())
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '.' {
			var tb textbuf.Buffer
			tb.Str("target ").Str(strconv.Quote(s)).Str(": invalid character ").Str(strconv.QuoteRune(c))
			return errors.New(tb.String())
		}
	}
	return nil
}

// parseSourceIP parses the source address the socket will bind. One parse both
// validates the argument and produces the value, so a rejected address can
// never reach the engine as an unset one. An IPv4-mapped IPv6 source is
// unmapped, because it binds an IPv4 socket.
func parseSourceIP(s string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("source ").Str(strconv.Quote(s)).Str(": not a valid IP address")
		return netip.Addr{}, errors.New(tb.String())
	}
	return addr.Unmap(), nil
}
