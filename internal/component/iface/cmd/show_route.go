// Design: docs/guide/command-reference.md -- `show route` operational command.
// Owned by the iface component: `show route` reads the kernel routing table
// through the iface backend (iface.ListKernelRoutes). See
// ai/rules/plugins.md and
// docs/architecture/cli/command-namespacing.md (object-rooted commands).

package cmd

import (
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// defaultRouteLimit caps the number of entries returned by `show route`
// so an operator on a full DFZ does not turn a single read into a
// multi-hundred-megabyte RPC response. Callers who want more can raise
// the cap via `limit N`; when the kernel FIB has more rows than the
// cap the handler trims the response and sets `truncated: true, limit: N`
// in the envelope so the caller can retry with a larger limit if desired.
const defaultRouteLimit = 100_000

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:route",
			Handler:    handleShowRoute,
		},
	)
}

// handleShowRoute returns the kernel routing table. A single positional
// argument restricts the output to one CIDR; use "default" for the
// 0.0.0.0/0 / ::/0 entries. The optional `limit N` caps the response
// size -- default 100 000 rows so a single read on a full DFZ does not
// produce a multi-hundred-megabyte RPC reply. Backends that do not own
// the kernel FIB (VPP today) reject per exact-or-reject -- kernel routes
// are not authoritative for the VPP fastpath and returning them would
// mislead operators.
//
// Invalid prefixes reject with the usage line rather than silently
// returning an empty result. "default" is accepted as a synonym for the
// 0.0.0.0/0 / ::/0 entries.
func handleShowRoute(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return dumpKernelRoutes(args, "usage: show route [<cidr>|default] [limit N]", defaultRouteLimit)
}

// dumpKernelRoutes implements the shared argument parsing + truncation
// envelope for `show route` and `show route lookup`'s sibling reads.
// `defaultLimit` lets callers pick a cap appropriate for their audience
// (interactive operator vs. programmatic scrape).
func dumpKernelRoutes(args []string, usage string, defaultLimit int) (*plugin.Response, error) {
	var tb textbuf.Buffer
	filter := ""
	limit := defaultLimit
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "limit":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Error: "limit requires a value"}, nil
			}
			n, parseErr := strconv.Atoi(args[i+1])
			if parseErr != nil || n <= 0 {
				msg := tb.Str("invalid limit ").Str(strconv.Quote(args[i+1])).Str(": must be a positive integer").String()
				return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil //nolint:nilerr // operational error via Response
			}
			limit = n
			i++
		case strings.HasPrefix(args[i], "--"):
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  tb.Reset().Str("unknown flag ").Str(strconv.Quote(args[i])).Str("; use 'limit N' (no dashes); ").Str(usage).String(),
			}, nil
		default:
			if filter != "" {
				return &plugin.Response{
					Status: plugin.StatusError,
					Error:  tb.Reset().Str("too many positional arguments; ").Str(usage).String(),
				}, nil
			}
			if args[i] != "default" {
				if _, err := netip.ParsePrefix(args[i]); err != nil {
					msg := tb.Reset().Str("invalid prefix ").Str(strconv.Quote(args[i])).Str(": ").Err(err).String()
					return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil //nolint:nilerr // operational error via Response
				}
			}
			filter = args[i]
		}
	}

	// Ask the backend for one more than `limit` so we can still detect
	// "there was more" without a separate count call; the backend stops
	// populating the Go slice once the cap is hit, bounding the real
	// allocation cost rather than just the response-size cost.
	routes, err := iface.ListKernelRoutes(filter, limit+1)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error via Response
	}
	truncated := false
	if len(routes) > limit {
		routes = routes[:limit]
		truncated = true
	}
	data := map[string]any{"routes": routes}
	if truncated {
		data["truncated"] = true
		data["limit"] = limit
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}
