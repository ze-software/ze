// Design: docs/architecture/cli/command-namespacing.md -- the call-site gate's answer
//
// report.go holds what `le ci-dispatch check` ANSWERS, apart from what produced
// it.
//
// The payload is an object rather than a slice, because the three counts are
// the point: a run that checked 1075 emitters and one that checked none both
// report zero findings, and only the counts tell them apart. One key holds
// rows, so the row operators act on the findings.

package cidispatch

import "github.com/ze-software/ze/internal/core/textbuf"

// The two kinds a finding can be. They are the script's own spellings, and they
// are what `| match dead` selects on.
const (
	// KindDead is a command string the dispatcher answers ErrUnknownCommand
	// for. The migration deleted its key.
	KindDead = "dead"
	// KindUnverifiable is a computed command with no static prefix to check.
	// It fails as loudly as a dead one: staying silent on a string the gate
	// could not read is the same blind spot in a new place.
	KindUnverifiable = "unverifiable"
)

// Finding is one emitter that does not resolve, and it is one ROW of the
// check's answer. The keys are the script's, unchanged.
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Kind    string `json:"kind"`
	Emitter string `json:"emitter"`
	Command string `json:"command"`
	Detail  string `json:"detail,omitempty"`
}

// Report is the whole answer of one check. The keys are the script's --json
// document, unchanged.
type Report struct {
	SchemaVersion   int       `json:"schema-version"`
	CommandsKnown   int       `json:"commands-known"`
	EmittersChecked int       `json:"emitters-checked"`
	PassThrough     int       `json:"pass-through"`
	Findings        []Finding `json:"findings"`
}

// Valid reports whether every emitter this run read resolves.
func (r Report) Valid() bool { return len(r.Findings) == 0 }

// Text renders the report for a person: the three counts, then one two-line
// entry per finding, then the verdict. It ends in a newline.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("# Dispatch Command Call-Site Gate\n\n")
	tb.Str("Registered commands: ").Int(int64(r.CommandsKnown)).Byte('\n')
	tb.Str("Emitters checked:    ").Int(int64(r.EmittersChecked)).Byte('\n')
	tb.Str("Pass-through (var):  ").Int(int64(r.PassThrough)).Byte('\n')
	tb.Byte('\n')

	if len(r.Findings) == 0 {
		tb.Str("ci-dispatch-check: OK\n")
		return tb.String()
	}

	dead, unverifiable := 0, 0
	for _, finding := range r.Findings {
		if finding.Kind == KindDead {
			dead++
			continue
		}
		unverifiable++
	}
	for _, finding := range r.Findings {
		tb.Str(finding.File).Byte(':').Int(int64(finding.Line)).Str(": ").
			Str(finding.Kind).Str(": ").Str(finding.Emitter).Byte('(').Quoted(finding.Command).Byte(')').Byte('\n')
		tb.Str("    ").Str(finding.Detail).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("ci-dispatch-check: FAIL (").Int(int64(dead)).
		Str(" dead, ").Int(int64(unverifiable)).Str(" unverifiable)\n")
	return tb.String()
}
