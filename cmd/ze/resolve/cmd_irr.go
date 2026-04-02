// Design: docs/architecture/resolve.md -- IRR resolution CLI
package resolve

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/irr"
)

var irrOperations = []string{"as-set", "prefix"}

func cmdIRR(ctx context.Context, args []string) int {
	if len(args) > 0 && isHelp(args[0]) {
		irrUsage()
		return exitOK
	}

	fs := flag.NewFlagSet("ze resolve irr", flag.ContinueOnError)
	serverFlag := fs.String("server", "", "IRR whois server (default: whois.radb.net:43)")
	fs.Usage = func() { irrUsage() }

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	remaining := fs.Args()
	if len(remaining) < 2 {
		irrUsage()
		return exitError
	}

	op := remaining[0]
	name := remaining[1]

	c := irr.NewIRR(*serverFlag)

	switch op {
	case "as-set":
		asns, err := c.ResolveASSet(ctx, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		if len(asns) == 0 {
			fmt.Fprintf(os.Stderr, "no members found for %s\n", name)
			return exitError
		}
		for _, asn := range asns {
			fmt.Printf("AS%d\n", asn)
		}

	case "prefix":
		pl, err := c.LookupPrefixes(ctx, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		if pl.Empty() {
			fmt.Fprintf(os.Stderr, "no prefixes found for %s\n", name)
			return exitError
		}
		for _, p := range pl.IPv4 {
			fmt.Println(p)
		}
		for _, p := range pl.IPv6 {
			fmt.Println(p)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown irr operation: %s\n", op)
		fmt.Fprintf(os.Stderr, "valid operations: %s\n", strings.Join(irrOperations, ", "))
		return exitError
	}

	return exitOK
}

func irrUsage() {
	p := helpfmt.Page{
		Command: "ze resolve irr",
		Summary: "IRR AS-SET expansion and prefix lookup",
		Usage:   []string{"ze resolve irr [--server <host>] <operation> <as-set-name>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Operations", Entries: []helpfmt.HelpEntry{
				{Name: "as-set <name>", Desc: "Expand AS-SET to member ASNs"},
				{Name: "prefix <name>", Desc: "Lookup announced prefixes for AS-SET"},
			}},
			{Title: "Flags", Entries: []helpfmt.HelpEntry{
				{Name: "--server <host>", Desc: "IRR whois server (default: whois.radb.net:43)"},
			}},
		},
		Examples: []string{
			"ze resolve irr as-set AS-CLOUDFLARE",
			"ze resolve irr prefix AS-CLOUDFLARE",
			"ze resolve irr --server rr.ntt.net as-set AS-EXAMPLE",
		},
	}
	p.Write()
}
