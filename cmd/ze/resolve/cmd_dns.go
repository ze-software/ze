// Design: docs/architecture/resolve.md -- DNS resolution CLI
package resolve

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	resolveDNS "codeberg.org/thomas-mangin/ze/internal/component/resolve/dns"
)

var dnsOperations = []string{"a", "aaaa", "txt", "ptr"}

func cmdDNS(args []string) int {
	if len(args) > 0 && isHelp(args[0]) {
		dnsUsage()
		return exitOK
	}

	fs := flag.NewFlagSet("ze resolve dns", flag.ContinueOnError)
	dnsServer := fs.String("server", "", "DNS server (default: system DNS)")
	fs.Usage = func() { dnsUsage() }

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	remaining := fs.Args()
	if len(remaining) < 2 {
		dnsUsage()
		return exitError
	}

	op := remaining[0]
	name := remaining[1]

	cfg := resolveDNS.ResolverConfig{}
	if *dnsServer != "" {
		cfg.Server = *dnsServer
	}

	resolver := resolveDNS.NewResolver(cfg)
	defer resolver.Close()

	var records []string
	var err error

	switch op {
	case "a":
		records, err = resolver.ResolveA(name)
	case "aaaa":
		records, err = resolver.ResolveAAAA(name)
	case "txt":
		records, err = resolver.ResolveTXT(name)
	case "ptr":
		records, err = resolver.ResolvePTR(name)
	default:
		fmt.Fprintf(os.Stderr, "unknown dns operation: %s\n", op)
		fmt.Fprintf(os.Stderr, "valid operations: %s\n", strings.Join(dnsOperations, ", "))
		return exitError
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	for _, r := range records {
		fmt.Println(r)
	}

	return exitOK
}

func dnsUsage() {
	p := helpfmt.Page{
		Command: "ze resolve dns",
		Summary: "DNS record queries",
		Usage:   []string{"ze resolve dns [--server <host>] <operation> <hostname|address>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Operations", Entries: []helpfmt.HelpEntry{
				{Name: "a <hostname>", Desc: "IPv4 address records"},
				{Name: "aaaa <hostname>", Desc: "IPv6 address records"},
				{Name: "txt <hostname>", Desc: "TXT records"},
				{Name: "ptr <address>", Desc: "Reverse DNS (PTR) records"},
			}},
			{Title: "Flags", Entries: []helpfmt.HelpEntry{
				{Name: "--server <host>", Desc: "DNS server (default: system DNS)"},
			}},
		},
		Examples: []string{
			"ze resolve dns a example.com",
			"ze resolve dns aaaa example.com",
			"ze resolve dns txt example.com",
			"ze resolve dns ptr 8.8.8.8",
			"ze resolve dns --server 8.8.8.8 a example.com",
		},
	}
	p.Write()
}
