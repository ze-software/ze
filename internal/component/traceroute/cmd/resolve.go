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

	dest, err := probe.ResolveTarget(target)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("traceroute: invalid target ").Str(strconv.Quote(target)).Str(": ").Err(err)
		return errResolveResponse(tb.String()), nil
	}

	maxHops := defaultTracerouteMaxHops
	timeout := defaultTracerouteTimeout
	probes := defaultTracerouteProbes
	var opts tracerouteOpts

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "source":
			if i+1 >= len(args) {
				return errResolveResponse("traceroute: \"source\" requires a value"), nil
			}
			i++
			if err := validateSourceIP(args[i]); err != nil {
				return errResolveResponse(err.Error()), nil
			}
			opts.source, _ = netip.ParseAddr(args[i])
		case "max-hops":
			if i+1 >= len(args) {
				return errResolveResponse("traceroute: \"max-hops\" requires a value"), nil
			}
			i++
			n, parseErr := strconv.Atoi(args[i])
			if parseErr != nil {
				return errResolveResponse("traceroute: max-hops requires a number"), nil //nolint:nilerr // operational error in Response
			}
			if n < 1 || n > maxTracerouteMaxHops {
				var tb textbuf.Buffer
				tb.Str("traceroute: max-hops must be 1-").Int(int64(maxTracerouteMaxHops))
				return errResolveResponse(tb.String()), nil
			}
			maxHops = n
		case argTimeout:
			if i+1 >= len(args) {
				return errResolveResponse("traceroute: \"timeout\" requires a value (e.g. 2s)"), nil
			}
			i++
			d, parseErr := time.ParseDuration(args[i])
			if parseErr != nil {
				return errResolveResponse("traceroute: timeout requires a duration (e.g. 2s)"), nil //nolint:nilerr // operational error in Response
			}
			if d < time.Second || d > maxTracerouteTimeout {
				var tb textbuf.Buffer
				tb.Str("traceroute: timeout must be 1s-").Str(maxTracerouteTimeout.String())
				return errResolveResponse(tb.String()), nil
			}
			timeout = d
		case "probes":
			if i+1 >= len(args) {
				return errResolveResponse("traceroute: \"probes\" requires a value"), nil
			}
			i++
			n, parseErr := strconv.Atoi(args[i])
			if parseErr != nil {
				return errResolveResponse("traceroute: probes requires a number"), nil //nolint:nilerr // operational error in Response
			}
			if n < 1 || n > maxTracerouteProbes {
				var tb textbuf.Buffer
				tb.Str("traceroute: probes must be 1-").Int(int64(maxTracerouteProbes))
				return errResolveResponse(tb.String()), nil
			}
			probes = n
		default:
			var tb textbuf.Buffer
			tb.Str("traceroute: unknown option ").Str(strconv.Quote(args[i]))
			return errResolveResponse(tb.String()), nil
		}
	}

	hops, trErr := doTracerouteCtx(ctx.Context(), dest, maxHops, timeout, probes, opts)
	if trErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: trErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{
		"hops": hops,
	}}, nil
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

func validateSourceIP(s string) error {
	if net.ParseIP(s) == nil {
		var tb textbuf.Buffer
		tb.Str("source ").Str(strconv.Quote(s)).Str(": not a valid IP address")
		return errors.New(tb.String())
	}
	return nil
}
