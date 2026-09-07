// Design: docs/architecture/testing/ci-format.md -- the .ci directive vocabulary
// Overview: record_parse.go -- the switches these lists gate
// Related: record_parse_keys.go -- the same refusal, one level down, for a directive's keys

package runner

import (
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The directive type tokens a switch case and a vocabulary list both name.
// Spelling one of them twice is what goconst reports, and the constant is the
// better answer here for the same reason the lists exist: the case label and
// the list entry are then the SAME declaration. directiveTypeBGP
// (record_parse.go) is the first of them.
const (
	directiveTypeFile   = "file"
	directiveTypeOpen   = "open"
	directiveTypeStderr = "stderr"
	directiveTypeStdout = "stdout"
	directiveTypeSyslog = "syslog"
)

// The vocabulary of the .ci directives record_parse.go reads, declared once per
// switch. Each list GATES its parser: a word the list does not carry is refused
// before the switch runs, and the refusal prints the same slice the gate read.
// So the list is what the runner accepts rather than a description of it, and
// neither drift direction is silent (ai/rules/principles.md). Add a case
// without adding its word here and the directive stops parsing, loudly, in
// every .ci that writes it. Add the word without a case and the parser answers
// errDirectiveUnlisted.
var (
	// recordActions is the action= word, the part before the first ':'.
	// expect=output:, expect=stream: and expect=command-error: never reach the
	// switch: parseLine intercepts them ahead of the ':' split because their
	// contains= needle may itself hold a ':'. They are expect types, so they are
	// named in recordExpectTypes rather than here.
	recordActions = []string{
		"action", "await", "cmd", engineActionCommand, "expect", "http", "option", "reject", engineActionStream,
	}

	recordOptionTypes = []string{
		"asn", "bind", "env", "exclusive", directiveTypeFile, "needs-linux", "needs-path", "netns-link",
		directiveTypeOpen, "skip-env", "skip-os", "tcp_connections", "timeout", "update",
	}

	// recordExpectTypes carries the three intercepted spellings as well, because
	// an author reading this refusal is asking what they may write, and those
	// three are writable. parseExpect gates on recordExpectTypesParsed, the arms
	// it holds itself.
	recordExpectTypes = slices.Concat(recordExpectTypesParsed, recordExpectTypesRaw)

	// recordExpectTypesParsed are the expect types parseExpect reads from a
	// key=value map.
	recordExpectTypesParsed = []string{
		directiveTypeBGP, "event", "exit", directiveTypeFile, "json",
		directiveTypeStderr, directiveTypeStdout, directiveTypeSyslog,
	}

	// recordExpectTypesRaw are the expect types parseLine reads whole, before
	// the ':' split, so their value keeps every colon the author wrote.
	recordExpectTypesRaw = []string{"command-error", "output", engineActionStream}

	recordRejectTypes = []string{directiveTypeBGP, directiveTypeStderr, directiveTypeStdout, directiveTypeSyslog}

	recordActionTypes = []string{"notification", "rewrite", "send", "sighup", "sigterm"}

	recordCmdTypes = []string{"api", modeBackground, modeForeground, modeStop}
)

// unknownDirective refuses a directive word no arm of its switch reads, and
// names what the runner accepts in its place.
//
// The three facts an author needs are the word they wrote, the accepted set,
// and the line, and the line is added by parseAndAdd, which is the only caller
// that knows it. A refusal that names only the word sends its reader into the
// parser to find the vocabulary, which is where the .ci format was learned by
// reading source for as long as this message existed.
func unknownDirective(kind, name string, accepted []string) error {
	sorted := slices.Clone(accepted)
	slices.Sort(sorted)

	var b textbuf.Buffer
	b.Str("unknown ").Str(kind).Byte(' ').Quoted(name).Str(" (accepts ").Join(sorted, ", ").Byte(')')
	return errors.New(b.String())
}

// errDirectiveUnlisted is the arm no author can reach: the vocabulary above
// carries the word and no case reads it, which is an edit that added half a
// directive. It fails the file rather than falling through, because a directive
// nobody parses is a directive that asserts nothing.
func errDirectiveUnlisted(kind, name string) error {
	var b textbuf.Buffer
	b.Str(kind).Byte(' ').Quoted(name).Str(" is listed as accepted and no parser reads it (BUG in the runner's vocabulary)")
	return errors.New(b.String())
}
