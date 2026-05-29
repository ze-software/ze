// Design: plan/spec-mpls-1-kernel.md -- `show mpls forwarding` CLI (AC-7)
// Related: kernel_routes.go -- sibling `show kernel-routes` handler
//
// `show mpls forwarding` lists the kernel's installed MPLS label-switching
// entries (the AF_MPLS routing table): incoming label, the operation applied
// (swap/pop), any outgoing label stack and the next hop. Reading the kernel
// directly (rather than a daemon's in-memory view) reports the authoritative
// dataplane state, matching how `show kernel-routes` works for IP. The kernel
// reader is platform-specific: mpls_forwarding_linux.go / mpls_forwarding_other.go.
package show

import (
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const defaultMPLSForwardingLimit = 100_000

// MPLSForwardingEntry is one row of the MPLS forwarding table.
type MPLSForwardingEntry struct {
	InLabel   int    `json:"in-label"`
	Operation string `json:"operation"`
	OutLabels []int  `json:"out-labels,omitempty"`
	NextHop   string `json:"next-hop,omitempty"`
	Device    string `json:"device,omitempty"`
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:mpls-forwarding",
			Handler:    handleShowMPLSForwarding,
		},
	)
}

// handleShowMPLSForwarding returns the kernel MPLS forwarding table. The only
// flag is `--limit N`, capping the response size.
func handleShowMPLSForwarding(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const usage = "usage: show mpls forwarding [--limit N]"
	limit := defaultMPLSForwardingLimit
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Error: "--limit requires a value"}, nil
			}
			n, parseErr := strconv.Atoi(args[i+1])
			if parseErr != nil || n <= 0 {
				return &plugin.Response{Status: plugin.StatusError, Error: "invalid --limit " + strconv.Quote(args[i+1]) + ": must be a positive integer"}, nil //nolint:nilerr // operational error via Response
			}
			limit = n
			i++
		default:
			return &plugin.Response{Status: plugin.StatusError, Error: "unexpected argument " + strconv.Quote(args[i]) + "; " + usage}, nil
		}
	}

	entries, err := dumpMPLSRoutes(limit + 1)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error via Response
	}
	truncated := false
	if len(entries) > limit {
		entries = entries[:limit]
		truncated = true
	}
	data := map[string]any{"entries": entries}
	if truncated {
		data["truncated"] = true
		data["limit"] = limit
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}
