// Design: docs/architecture/resolve.md -- resolve ping command handler
// Related: ping.go -- doPing internal ICMP engine shared with show ping

package cmd

import (
	"errors"
	"net"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/probe"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errResolveTargetEmpty = errors.New("target must not be empty")

// handleResolvePing is the RPC handler for `resolve ping` (ze-resolve:ping):
// ICMP echo requests with optional source binding, count, and payload size.
func handleResolvePing(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
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
		tb.Str("ping: invalid destination ").Str(strconv.Quote(target)).Str(": ").Err(err)
		return errResolveResponse(tb.String()), nil
	}

	count := 4
	timeout := defaultPingTimeout
	var opts pingOpts

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "source":
			if i+1 >= len(args) {
				return errResolveResponse("ping: \"source\" requires a value"), nil
			}
			i++
			if err := validateSourceIP(args[i]); err != nil {
				return errResolveResponse(err.Error()), nil
			}
			opts.source, _ = netip.ParseAddr(args[i])
		case argCount:
			if i+1 >= len(args) {
				return errResolveResponse("ping: \"count\" requires a value"), nil
			}
			i++
			if err := validateUint(args[i], "count", 1, 100); err != nil {
				return errResolveResponse(err.Error()), nil
			}
			n, _ := strconv.Atoi(args[i])
			count = n
		case argSize:
			if i+1 >= len(args) {
				return errResolveResponse("ping: \"size\" requires a value"), nil
			}
			i++
			if err := validateUint(args[i], argSize, 1, maxPingSize); err != nil {
				return errResolveResponse(err.Error()), nil
			}
			n, _ := strconv.ParseUint(args[i], 10, 64)
			opts.size = int(n)
		default:
			var tb textbuf.Buffer
			tb.Str("ping: unknown option ").Str(strconv.Quote(args[i]))
			return errResolveResponse(tb.String()), nil
		}
	}

	results, pingErr := doPingCtx(ctx.Context(), dest, count, timeout, opts)
	if pingErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: pingErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(results)}, nil
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

func validateUint(s, name string, lo, hi uint64) error {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str(name).Str(" ").Str(strconv.Quote(s)).Str(": not a valid number")
		return errors.New(tb.String())
	}
	if n < lo || n > hi {
		var tb textbuf.Buffer
		tb.Str(name).Str(" ").Int(int64(n)).Str(": out of range ").Int(int64(lo)).Str("..").Int(int64(hi))
		return errors.New(tb.String())
	}
	return nil
}
