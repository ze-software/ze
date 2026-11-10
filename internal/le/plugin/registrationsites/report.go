// Design: docs/architecture/plugin/plugin-system.md -- registration closes when main starts
// Related: registrationsites.go -- the judgement this file renders
//
// report.go holds what `le plugin registration-sites check` ANSWERS, apart
// from what produced it. The answer renders itself (Text), because a person
// reading a refusal wants the file, the line and the repair under it.

package pluginregistrationsites

import "github.com/ze-software/ze/internal/core/textbuf"

// Site is one reference to a registering function of the plugin registry.
type Site struct {
	// Package is the directory of the package holding the reference.
	Package string `json:"package"`
	// File and Line are where the reference is written.
	File string `json:"file"`
	Line int    `json:"line"`
	// Function is the registering function, qualified by the registry package.
	Function string `json:"function"`
	// Within names the declaration holding the reference: init, a function's
	// name, a method, or the package initializer.
	Within string `json:"within"`
}

// Refusal is one site that breaks the registration rule.
type Refusal struct {
	Site
	// Rule is the part of the rule the site breaks, and why that matters.
	Rule string `json:"rule"`
	// Fix is the repair and the command that prints the whole rule.
	Fix string `json:"fix"`
}

// Report is the whole answer of one check over one committed revision.
type Report struct {
	Revision string `json:"revision"`
	// Packages counts the Ze-module packages read; vendor/ is never read.
	Packages int `json:"packages"`
	// Registering lists the registry functions judged as registering: the
	// exported ones that reach the seal guard.
	Registering []string  `json:"registering"`
	Accepted    []Site    `json:"accepted"`
	Refused     []Refusal `json:"refused"`
}

// Text renders the report for a person. A pass names how much it read and
// every accepted site; a refusal names each wrong site, its rule and the
// repair. It ends in a newline.
func (r Report) Text() string {
	var tb textbuf.Buffer
	if len(r.Refused) != 0 {
		tb.Str("plugin-registration-sites: ").Int(int64(len(r.Refused))).
			Str(" registering site(s) at ").Str(r.Revision).Str(" break the registration rule:\n")
		for _, refusal := range r.Refused {
			writeSite(&tb, refusal.Site)
			tb.Str("    ").Str(refusal.Rule).Byte('\n')
			tb.Str("    fix: ").Str(refusal.Fix).Byte('\n')
		}
		return tb.String()
	}
	tb.Str("plugin-registration-sites: OK at ").Str(r.Revision).Str(": ").
		Int(int64(r.Packages)).Str(" packages read, ").
		Int(int64(len(r.Accepted))).Str(" registering site(s) accepted\n")
	for _, site := range r.Accepted {
		writeSite(&tb, site)
	}
	return tb.String()
}

func writeSite(tb *textbuf.Buffer, site Site) {
	tb.Str("  ").Str(site.File).Byte(':').Int(int64(site.Line)).
		Str("  ").Str(site.Function).Str(" in ").Str(site.Within).Byte('\n')
}
