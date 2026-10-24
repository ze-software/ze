// Design: docs/architecture/plugin/feature-gates.md -- what the coupling check answers
//
// report.go holds what the check ANSWERS, apart from what produced it. The
// answer carries one row set, the couplings, so `| json` feeds a script and
// `| count` says how many the run found.

package coupling

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CheckReport is what `le plugin coupling check` answers.
type CheckReport struct {
	// Couplings is every undeclared cross-gate coupling, sorted by consumer.
	Couplings []Coupling `json:"couplings"`
	// Undeclarable are cross-gate couplings whose producer is code outside
	// every plugin's directory, a subsystem such as L2TP: no edge can name it,
	// so they are listed and do not fail the check.
	Undeclarable []Coupling `json:"undeclarable,omitempty"`
	// Unjudged are the sites of uses taken by code outside every plugin's
	// directory: no plugin can declare an edge for them, so they are listed
	// rather than judged.
	Unjudged []string `json:"unjudged,omitempty"`
	// Plugins is how many registrations the run read.
	Plugins int `json:"plugins"`
	// Files is how many Go files the run parsed.
	Files int `json:"files"`
}

// Text renders the verdict: each coupling with what is wrong, why it matters
// and the edit that fixes it. It ends in a newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer

	for i := range r.Undeclarable {
		tb.Str("note: ")
		writeCoupling(&tb, &r.Undeclarable[i])
	}
	for i := range r.Couplings {
		writeCoupling(&tb, &r.Couplings[i])
	}
	if len(r.Couplings) > 0 {
		tb.Str("Fix: in the consumer's register.go, add the provider to OptionalDependencies with a ConfigNeeds entry\n").
			Str("  when a config setting makes the consumer depend on it (a build without it then refuses that setting),\n").
			Str("  or to StartAfter when the consumer only listens and works without it.\n")
		return tb.String()
	}

	tb.Str("no undeclared cross-gate coupling (").Int(int64(r.Plugins)).Str(" plugins, ").
		Int(int64(r.Files)).Str(" files")
	if len(r.Unjudged) > 0 {
		tb.Str(", ").Int(int64(len(r.Unjudged))).Str(" uses outside every plugin not judged")
	}
	return tb.Str(")\n").String()
}

// writeCoupling renders one coupling as two lines: the problem, then why.
func writeCoupling(tb *textbuf.Buffer, c *Coupling) {
	tb.Str(c.Site).Str(": plugin ").Str(c.Consumer).Str(" takes ").Str(c.Item).
		Str(" (").Str(c.Kind).Str("), which only ")
	if len(c.Providers) == 0 {
		tb.Str("code outside every plugin (no edge can name it)")
	} else {
		tb.Str(strings.Join(c.Providers, " or "))
	}
	tb.Str(" produces, and ").Str(c.Consumer).Str(" declares no edge to it\n")
	tb.Str("  a build without ").Str(strings.Join(c.ProviderTags, " ")).Str(" still runs ").Str(c.Consumer).
		Str(", which then receives nothing and says nothing\n")
}
