// Design: docs/architecture/resolve.md -- PeeringDB resolution CLI
package resolve

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/peeringdb"
)

const defaultPeeringDBURL = "https://www.peeringdb.com"

var peeringDBOperations = []string{"max-prefix", "as-set"}

func cmdPeeringDB(ctx context.Context, args []string) int {
	if len(args) > 0 && isHelp(args[0]) {
		peeringDBUsage()
		return exitOK
	}

	fs := flag.NewFlagSet("ze resolve peeringdb", flag.ContinueOnError)
	urlFlag := fs.String("url", defaultPeeringDBURL, "PeeringDB API base URL")
	fs.Usage = func() { peeringDBUsage() }

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	remaining := fs.Args()
	if len(remaining) < 2 {
		peeringDBUsage()
		return exitError
	}

	op := remaining[0]
	asnStr := remaining[1]

	asn, err := strconv.ParseUint(asnStr, 10, 32)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid ASN: %s\n", asnStr)
		return exitError
	}

	c := peeringdb.NewPeeringDB(*urlFlag)

	switch op {
	case "max-prefix":
		counts, lookupErr := c.LookupASN(ctx, uint32(asn))
		if lookupErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", lookupErr)
			return exitError
		}
		fmt.Printf("ipv4: %d\n", counts.IPv4)
		fmt.Printf("ipv6: %d\n", counts.IPv6)
		if counts.Suspicious() {
			fmt.Fprintf(os.Stderr, "warning: both counts are zero (ASN may not be in PeeringDB)\n")
		}

	case "as-set":
		sets, lookupErr := c.LookupASSet(ctx, uint32(asn))
		if lookupErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", lookupErr)
			return exitError
		}
		if len(sets) == 0 {
			fmt.Fprintf(os.Stderr, "no AS-SET registered for AS%d\n", asn)
			return exitError
		}
		for _, s := range sets {
			fmt.Println(s)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown peeringdb operation: %s\n", op)
		fmt.Fprintf(os.Stderr, "valid operations: %s\n", strings.Join(peeringDBOperations, ", "))
		return exitError
	}

	return exitOK
}

func peeringDBUsage() {
	p := helpfmt.Page{
		Command: "ze resolve peeringdb",
		Summary: "PeeringDB prefix count and AS-SET queries",
		Usage:   []string{"ze resolve peeringdb [--url <url>] <operation> <asn>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Operations", Entries: []helpfmt.HelpEntry{
				{Name: "max-prefix <asn>", Desc: "IPv4 and IPv6 prefix counts"},
				{Name: "as-set <asn>", Desc: "Registered IRR AS-SET names"},
			}},
			{Title: "Flags", Entries: []helpfmt.HelpEntry{
				{Name: "--url <url>", Desc: "PeeringDB API base URL (default: " + defaultPeeringDBURL + ")"},
			}},
		},
		Examples: []string{
			"ze resolve peeringdb max-prefix 13335",
			"ze resolve peeringdb as-set 13335",
			"ze resolve peeringdb --url http://localhost:8080 max-prefix 65001",
		},
	}
	p.Write()
}
