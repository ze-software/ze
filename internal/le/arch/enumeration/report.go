// Design: docs/architecture/core-design.md -- what an le area answers
//
// Overview: enumeration.go -- the walk that produces these rows
//
// report.go holds what `le arch enumeration` ANSWERS, apart from what produced it.
//
// The answer IS its rows, so the payload is a slice rather than a struct
// wrapping one: `| json` renders the array and `| count` says how many. The
// slice also renders itself, because a list of copies with the remedy under it
// is what a person reads here.

package archenumeration

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The four kinds of row this gate answers. A reader filters on the kind, so
// each one is a word rather than a sentence.
const (
	// KindLiteral is a syntactic unit that writes a registry's keys out again.
	KindLiteral = "literal"
	// KindMarker is an exemption marker that excuses nothing: it states no
	// reason, or it suppresses no unit.
	KindMarker = "marker"
	// KindDoctorCheck is a doctor check function a hand-written call reaches
	// and no registration does.
	KindDoctorCheck = "doctor-check"
	// KindGated is a closed-corpus row whose agreement with the model a named
	// test proves (the `gated by TestX` marker). It is listed, counted apart,
	// and never blocks: the backlog stays visible while check stops on it
	// (owner decision, 2026-09-14).
	KindGated = "gated"
)

// Finding is one row of the gate's answer.
//
// Corpus names the registry a literal restates, which is the fact the reader
// acts on: it says where the one declaration of that set lives. The two
// structural kinds carry no corpus, because neither is about a key set.
type Finding struct {
	Kind   string   `json:"kind"`
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Symbol string   `json:"symbol,omitempty"`
	Corpus string   `json:"corpus,omitempty"`
	Keys   []string `json:"keys,omitempty"`
	// Strings is how many distinct string constants the unit holds in all, of
	// which Keys are the registry's. The two together are what a reader judges
	// a row by: four of four is a transcription, two of forty-one is a literal
	// about something else that happens to hold two of these words.
	Strings int    `json:"strings,omitempty"`
	Detail  string `json:"detail"`
}

// CheckReport is what an action answers: the rows, and WHICH code the run
// judged to produce them.
//
// The scope is published rather than implied, because the two live answers are
// told apart by nothing else. An empty row set from the change set and an empty
// row set from a run that judged nothing look the same, and the second is the
// defect this gate is named after.
type CheckReport struct {
	Scope    string   `json:"scope"`
	Findings Findings `json:"findings"`
}

// Text renders the scope line first, then the rows. The scope comes first
// because it decides how much the rows are worth.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("enumeration scope: ").Str(r.Scope).Byte('\n')
	return tb.Str(r.Findings.Text()).String()
}

// exitCode is what check answers the shell for this report: 1 on a finding,
// 0 on none. A gated row is not a finding, so a report holding only gated rows
// exits 0 while still listing them.
func (r CheckReport) exitCode() int {
	if findings, _ := r.Findings.split(); len(findings) > 0 {
		return 1
	}
	return 0
}

// Findings is the whole answer of one run.
type Findings []Finding

// sort orders the rows by file, then line, then corpus, so two runs over one
// tree answer the same list in the same order.
func (f Findings) sort() {
	slices.SortFunc(f, func(a, b Finding) int {
		if a.File != b.File {
			return strings.Compare(a.File, b.File)
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.Corpus, b.Corpus)
	})
}

// split answers the rows apart: the findings, which check blocks on, and the
// gated rows, which it does not.
func (f Findings) split() (findings, gated Findings) {
	for _, finding := range f {
		if finding.Kind == KindGated {
			gated = append(gated, finding)
			continue
		}
		findings = append(findings, finding)
	}
	return findings, gated
}

// Count is one tally row: the findings and the gated rows of one corpus.
type Count struct {
	Findings int
	Gated    int
}

// Tally counts the rows by corpus, and by kind for the rows that name no
// corpus, with the gated rows of a corpus counted apart from its findings. It
// is what `le arch enumeration report` is read for: which registry the tree copies
// most, and how much of a closed corpus is proved by a test.
func (f Findings) Tally() map[string]Count {
	tally := map[string]Count{}
	for _, finding := range f {
		name := finding.Corpus
		if name == "" {
			name = finding.Kind
		}
		count := tally[name]
		if finding.Kind == KindGated {
			count.Gated++
		} else {
			count.Findings++
		}
		tally[name] = count
	}
	return tally
}

// Text renders the rows for a person: the findings, then the gated rows, then
// the tally and the remedy. A run that found nothing says so, and a run whose
// every row is gated says OK and still lists them. It ends in a newline.
func (f Findings) Text() string {
	var tb textbuf.Buffer
	findings, gated := f.split()
	if len(findings) == 0 {
		tb.Str("enumeration: OK\n")
	} else {
		tb.Str("enumeration: ").Int(int64(len(findings))).Str(" finding(s):\n")
	}
	for _, finding := range findings {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).Str(": ")
		if finding.Symbol != "" {
			tb.Str(finding.Symbol).Str(": ")
		}
		tb.Str(finding.Detail)
		if len(finding.Keys) > 0 {
			tb.Str(" (").Int(int64(len(finding.Keys)))
			if finding.Strings > 0 {
				tb.Str(" of ").Int(int64(finding.Strings))
			}
			tb.Str(" strings: ").Str(strings.Join(finding.Keys, ", ")).Byte(')')
		}
		tb.Byte('\n')
	}
	if len(f) == 0 {
		return tb.String()
	}

	// The gated rows come after the findings and carry no key list: the test
	// each one names is what a reader opens, and the row is not work to do.
	if len(gated) > 0 {
		tb.Byte('\n')
	}
	for _, row := range gated {
		tb.Str("gated: ").Str(row.File).Byte(':').Int(int64(row.Line)).Str(": ").
			Str(row.Symbol).Str(": ").Str(row.Detail).Byte('\n')
	}

	tb.Byte('\n').Str("by corpus:\n")
	tally := f.Tally()
	names := make([]string, 0, len(tally))
	for name := range tally {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		count := tally[name]
		tb.Str("  ").Str(name).Str(": ").Int(int64(count.Findings))
		if count.Gated > 0 {
			tb.Str(" findings, ").Int(int64(count.Gated)).Str(" gated")
		}
		tb.Byte('\n')
	}

	tb.Byte('\n')
	tb.Str("Each row holds a second declaration of a set that is already declared elsewhere,\n")
	tb.Str("and two declarations of one fact drift. Where the row says a literal RESTATES a\n")
	tb.Str("registry, the registry is the declaration: derive the set from it. Where the row\n")
	tb.Str("says the two MUST AGREE, both sides are declarations and which one should derive\n")
	tb.Str("from the other is a design decision this gate cannot make for you.\n")
	tb.Str("This checkout is shared, so a row in a file you did not touch is somebody\n")
	tb.Str("else's: it is a real finding and it belongs to whoever wrote that file\n")
	tb.Str("(ai/rules/principles.md, several sessions work this checkout at once).\n")
	tb.Str("Where the list states what is ALLOWED rather than what EXISTS, it is policy:\n")
	tb.Str("mark it `enumeration: exempt (why this list is policy)` and the marker itself is\n")
	tb.Str("accounted for, so it goes red when it stops suppressing anything.\n")
	tb.Str("A GATED row is a MUST AGREE row whose Go side carries a fact the model does not\n")
	tb.Str("(a wire value, an IANA number, a kernel name, a handler), so the Go table is the\n")
	tb.Str("declaration and the model is the copy: an agreement test in the owning package\n")
	tb.Str("loads the module, reads the enumeration at the leaf the row names and compares\n")
	tb.Str("both ways. Mark the table `enumeration: gated by TestX`, naming that test, and\n")
	tb.Str("the row stays listed here while check stops blocking on it. A bare TestX must be\n")
	tb.Str("declared in the table's own package; a test in another package is spelled\n")
	tb.Str("`enumeration: gated by <dir>:TestX`, with the directory relative to the checkout.\n")
	tb.Str("The gate verifies only that the test is declared in that package,\n")
	tb.Str("not that it reads the leaf: a gated row is proved by its test, never by the gate.\n")
	tb.Str("The marker is legitimate only where the Go side carries such a fact; a plain\n")
	tb.Str("copy of the model derives from the model, and a gated marker on a registry copy\n")
	tb.Str("is itself a finding.\n")
	return tb.String()
}
