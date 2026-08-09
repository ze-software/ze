// Design: docs/architecture/mpls/mpls-kernel.md -- `show mpls forwarding` CLI (AC-7)
// Related: forwarding_linux.go -- kernel reader (AF_MPLS swap/pop + IP-encap push)
//
// `show mpls forwarding` lists the kernel's installed MPLS label-switching
// entries: the AF_MPLS routing table (swap/pop, keyed by incoming label) AND the
// label-imposition (push) routes, which are ordinary IP routes carrying an MPLS
// label encap (keyed by FEC prefix, e.g. a BGP labeled-unicast or LDP/RSVP-TE
// ingress). Reading the kernel directly (rather than a daemon's in-memory view)
// reports the authoritative dataplane state, matching how `show route`
// works for IP. The kernel reader is platform-specific: forwarding_linux.go /
// forwarding_other.go.
package mpls

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const defaultForwardingLimit = 100_000

// forwardingEntry is one row of the MPLS forwarding table. A swap/pop entry is
// keyed by InLabel; a push entry is keyed by FEC (the IP prefix whose traffic
// gets the label stack imposed) and has no InLabel.
type forwardingEntry struct {
	InLabel   int    `json:"in-label,omitempty"`
	FEC       string `json:"fec,omitempty"`
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

func handleShowMPLSForwarding(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const usage = "usage: show mpls forwarding [limit N]"
	var tb textbuf.Buffer
	limit := defaultForwardingLimit
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "limit":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Error: "limit requires a value"}, nil
			}
			n, parseErr := strconv.Atoi(args[i+1])
			if parseErr != nil || n <= 0 {
				msg := tb.Str("invalid limit ").Quoted(args[i+1]).Str(": must be a positive integer").String()
				return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil //nolint:nilerr // operational error via Response
			}
			limit = n
			i++
		default:
			msg := tb.Reset().Str("unexpected argument ").Quoted(args[i]).Str("; ").Str(usage).String()
			return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil
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
	// entries is the response's list contract: consumers (and the .ci test)
	// require it to marshal as a JSON array even on a host with no MPLS routes.
	// A nil slice marshals to `null`, so pin it to an empty array. The Linux
	// reader already returns a non-nil slice; forwarding_other.go returns nil.
	if entries == nil {
		entries = []forwardingEntry{}
	}
	data := map[string]any{"entries": entries}
	if truncated {
		data["truncated"] = true
		data["limit"] = limit
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}
