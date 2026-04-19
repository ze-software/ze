// Design: docs/features/interfaces.md -- kernel routing table CLI entry point

package iface

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"text/tabwriter"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	ifacepkg "codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// defaultRouteLimit bounds the offline `ze interface routes` dump. The
// backend slice is capped so a full-DFZ dump on a busy host cannot
// blow the Go heap through a single read. Operators who want more can
// raise this via --limit.
const defaultRouteLimit = 1000

// cmdRoutes lists entries from the kernel routing table. An optional
// positional CIDR restricts the dump. --limit caps the result size.
// Returns exit code.
func cmdRoutes(args []string) int {
	fs := flag.NewFlagSet("ze interface routes", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	limit := fs.Int("limit", defaultRouteLimit, "Maximum number of routes to return (must be > 0)")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze interface routes",
			Summary: "List kernel routing table entries",
			Usage:   []string{"ze interface routes [<cidr>] [--limit N] [--json]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--json", Desc: "Output in JSON format"},
					{Name: "--limit N", Desc: fmt.Sprintf("Maximum number of routes (default %d, must be > 0)", defaultRouteLimit)},
				}},
			},
			Examples: []string{
				"ze interface routes",
				"ze interface routes 10.0.0.0/8",
				"ze interface routes --limit 50000",
				"ze interface routes --json",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if *limit <= 0 {
		fmt.Fprintf(os.Stderr, "error: --limit must be > 0\n")
		return 1
	}

	remaining := fs.Args()
	filter := ""
	switch len(remaining) {
	case 0:
		// default: full dump (capped by --limit)
	case 1:
		if remaining[0] != "default" {
			if _, err := netip.ParsePrefix(remaining[0]); err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid prefix %q: %v\n", remaining[0], err)
				return 1
			}
		}
		filter = remaining[0]
	default:
		fmt.Fprintf(os.Stderr, "error: too many arguments\n")
		fs.Usage()
		return 1
	}

	// Request one extra row so we can flag truncation without a second
	// call. Same pattern as handleShowIPRoute in internal/component/cmd/show.
	// Clamp against int overflow: the backend treats limit <= 0 as
	// unbounded, so *limit == math.MaxInt would overflow to MinInt and
	// silently lift the cap. The clamp keeps the cap meaningful even
	// with a pathological --limit.
	backendLimit := *limit + 1
	if *limit > 1<<30 { // arbitrary sane ceiling; 1 B rows = ~O(GiB) anyway
		backendLimit = *limit
	}
	routes, err := ifacepkg.ListKernelRoutes(filter, backendLimit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	truncated := false
	if len(routes) > *limit {
		routes = routes[:*limit]
		truncated = true
	}

	if *jsonOutput {
		out := map[string]any{"routes": routes}
		if truncated {
			out["truncated"] = true
			out["limit"] = *limit
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	printRow(w, "DESTINATION", "NEXTHOP", "DEVICE", "PROTOCOL", "METRIC", "FAMILY")
	printRow(w, "-----------", "-------", "------", "--------", "------", "------")
	for _, r := range routes {
		nh := r.NextHop
		if nh == "" {
			nh = "-"
		}
		dev := r.Device
		if dev == "" {
			dev = "-"
		}
		printRow(w, r.Destination, nh, dev, r.Protocol, fmt.Sprint(r.Metric), r.Family)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if truncated {
		fmt.Fprintf(os.Stderr, "warning: result truncated at --limit %d; raise --limit to see more\n", *limit)
	}
	return 0
}
