// Design: docs/architecture/resolve.md -- Team Cymru ASN resolution CLI
package resolve

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/cymru"
	resolveDNS "codeberg.org/thomas-mangin/ze/internal/component/resolve/dns"
)

func cmdCymru(args []string) int {
	if len(args) > 0 && isHelp(args[0]) {
		cymruUsage()
		return exitOK
	}

	fs := flag.NewFlagSet("ze resolve cymru", flag.ContinueOnError)
	dnsServer := fs.String("dns-server", "", "DNS server for TXT queries (default: system DNS)")
	fs.Usage = func() { cymruUsage() }

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	remaining := fs.Args()
	if len(remaining) < 2 || remaining[0] != "asn-name" {
		cymruUsage()
		return exitError
	}

	asn, err := strconv.ParseUint(remaining[1], 10, 32)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid ASN: %s\n", remaining[1])
		return exitError
	}

	cfg := resolveDNS.ResolverConfig{}
	if *dnsServer != "" {
		cfg.Server = *dnsServer
	}

	dns := resolveDNS.NewResolver(cfg)
	defer dns.Close()

	txtResolver := func(_ context.Context, name string) ([]string, error) {
		return dns.ResolveTXT(name)
	}

	r := cymru.New(txtResolver, nil)

	name, err := r.LookupASNName(context.Background(), uint32(asn))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if name == "" {
		fmt.Fprintf(os.Stderr, "no name found for AS%d\n", asn)
		return exitError
	}

	fmt.Println(name)

	return exitOK
}

func cymruUsage() {
	p := helpfmt.Page{
		Command: "ze resolve cymru",
		Summary: "Team Cymru ASN-to-name resolution",
		Usage:   []string{"ze resolve cymru [--dns-server <host>] asn-name <asn>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Operations", Entries: []helpfmt.HelpEntry{
				{Name: "asn-name <asn>", Desc: "Resolve ASN to organization name"},
			}},
			{Title: "Flags", Entries: []helpfmt.HelpEntry{
				{Name: "--dns-server <host>", Desc: "DNS server for TXT queries (default: system DNS)"},
			}},
		},
		Examples: []string{
			"ze resolve cymru asn-name 13335",
			"ze resolve cymru --dns-server 8.8.8.8 asn-name 65001",
		},
	}
	p.Write()
}
