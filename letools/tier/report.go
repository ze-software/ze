// Design: ai/rules/architecture.md -- what the tier gate answers
//
// Overview: tier.go -- the import audit behind these answers
//
// report.go contains `le tier` ANSWERS, separate from their producers.
//
// It preserves the script's two output streams. Each check writes either a
// stdout verdict (Page) or a stderr failure (Diagnosis). One run can contain
// both when different checks pass and fail. A merged capture would order pages
// by stream flush timing, which prevents comparison. Separate streams prove
// the migration.
package tier

import "github.com/ze-software/ze/internal/core/textbuf"

// Row is one subsystem's audit result: who imports it from outside, and what
// that makes it.
type Row struct {
	// Name is the subsystem directory, without its area.
	Name string `json:"name"`
	// External are the importers that are neither a composition root nor a
	// test. One of them is what makes a subsystem shared code rather than a
	// plugin candidate.
	External []string `json:"external"`
	// Registration are the composition-root blank imports that WIRE it.
	Registration []string `json:"registration"`
	// Tests are the test files that reach it.
	Tests []string `json:"tests"`
	// IsCandidate says nothing outside its subtree calls into it.
	IsCandidate bool `json:"is-candidate"`
	// IsRegistered says a composition root wires it, which is the
	// authoritative "the build treats this as a plugin or a command" signal.
	IsRegistered bool `json:"is-registered"`
	// IsEngine says its non-test code constructs a config-driven engine.
	IsEngine bool `json:"is-engine"`
	// CoreCandidate says it is a leaf nothing depends on, that no composition
	// root wires, and that is not an engine.
	CoreCandidate bool `json:"core-candidate"`
}

// AuditReport answers `le tier report` with the module and each area's rows.
//
// The script's --json keys used underscores. `le` requires kebab-case
// (ai/rules/cli.md). No gate, target, or hook reads this document, so keys change
// with the command. Human-readable output stays unchanged.
type AuditReport struct {
	Module string           `json:"module"`
	Areas  map[string][]Row `json:"areas"`
	// Order contains areas in request order. JSON object keys do not preserve
	// that order, but the page requires it.
	Order []string `json:"order"`
}

// Text renders one script-style block for each area. It includes registered
// plugins, core candidates, and shared libraries with their importers.
func (r AuditReport) Text() string {
	var tb textbuf.Buffer
	for _, area := range r.Order {
		var plugins, candidates, shared []Row
		for _, row := range r.Areas[area] {
			switch {
			case row.IsRegistered:
				plugins = append(plugins, row)
			case row.CoreCandidate:
				candidates = append(candidates, row)
			default:
				shared = append(shared, row)
			}
		}
		sortByExternalCount(shared)

		tb.Byte('\n').Repeat("=", 78).Byte('\n')
		tb.Str("AREA: ").Str(area).Byte('\n')
		tb.Repeat("=", 78).Byte('\n')

		tb.Str("\n-- REGISTERED PLUGINS (wired by the generator / all.go): ").
			Int(int64(len(plugins))).Str(" --\n")
		for _, row := range plugins {
			engine := "  -   "
			if row.IsEngine {
				engine = "engine"
			}
			tb.Str("  ").PadRight(row.Name, 24).Byte(' ').Str(engine).Str("  external=").
				Int(int64(len(row.External))).Byte('\n')
		}

		tb.Str("\n-- CORE CANDIDATES (0 external, not registered, not an engine): ").
			Int(int64(len(candidates))).Str(" --\n")
		for _, row := range candidates {
			tb.Str("  ").PadRight(row.Name, 24).Str(" registration=").Int(int64(len(row.Registration))).
				Str(" tests=").Int(int64(len(row.Tests))).Byte('\n')
		}

		tb.Str("\n-- SHARED LIBRARIES (external importers, not a registered plugin): ").
			Int(int64(len(shared))).Str(" --\n")
		for _, row := range shared {
			tb.Str("  ").PadRight(row.Name, 24).Str(" external=").Int(int64(len(row.External))).Byte('\n')
			for i, importer := range row.External {
				if i == 6 {
					break
				}
				tb.Str("        <- ").Str(importer).Byte('\n')
			}
			if len(row.External) > 6 {
				tb.Str("        ... and ").Int(int64(len(row.External) - 6)).Str(" more\n")
			}
		}
	}
	return tb.String()
}

// sortByExternalCount orders the shared libraries by how many external
// importers they have, most first, keeping the name order among equals. That is
// Python's stable sort on the negated count.
func sortByExternalCount(rows []Row) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && len(rows[j].External) > len(rows[j-1].External); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// GateResult is what one of the five checks said.
type GateResult struct {
	// Name is the check, so a caller of `| json` can tell which one failed.
	Name string `json:"name"`
	// Page is what the check wrote to stdout, which is its verdict.
	Page string `json:"page,omitempty"`
	// Diagnosis is what it wrote to stderr, which is its failure.
	Diagnosis string `json:"diagnosis,omitempty"`
	// Code is the check's own exit code: 0 or 2.
	Code int `json:"code"`
}

// CheckReport is the whole answer of `le tier check`.
type CheckReport struct {
	// Gates are the five checks, in the order they ran.
	Gates []GateResult `json:"gates"`
	// Failed is the FIRST non-zero code, which is the one a caller can act on:
	// a later check's failure says nothing about the first one's.
	Failed int `json:"failed"`
}

// Text renders the stdout half: every check's verdict, in order.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer
	for _, gate := range r.Gates {
		tb.Str(gate.Page)
	}
	return tb.String()
}

// Diagnosis renders the stderr half: every check's failure, in order.
func (r CheckReport) Diagnosis() string {
	var tb textbuf.Buffer
	for _, gate := range r.Gates {
		tb.Str(gate.Diagnosis)
	}
	return tb.String()
}

// BaselineReport is what `le tier write-baseline` answers.
type BaselineReport struct {
	// File is the baseline that was written.
	File string `json:"file"`
	// Engines is how many misplaced engines it now names.
	Engines int `json:"engines"`
}

// Text renders the line that the script prints after it writes the baseline.
func (r BaselineReport) Text() string {
	var tb textbuf.Buffer
	return tb.Str("wrote ").Str(r.File).Str(" with ").Int(int64(r.Engines)).Str(" engine(s)\n").String()
}
