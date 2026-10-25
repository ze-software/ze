// Design: docs/contributing/running-commands.md -- installing le's completion
// Related: register.go -- the area's registration as a shell callback
//
// Package completion is `le completion`: the shell scripts that complete le's
// command lines, and the callback they run on each keystroke. The scripts come
// from the generator ze's own completion shares (internal/plugins/completion/script),
// and the candidates from the tree every other renderer reads (leroot.Complete).
package completion

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/plugins/completion/script"
)

// name is the word this command is typed as.
const name = "completion"

// shortHelp is the area's one line, which the help line and the listing read.
const shortHelp = "shell completion for le: the bash, zsh and fish scripts, and the callback they run"

// verbWords is the callback the scripts run on every keystroke.
const verbWords = "words"

// program describes le to the shared generator: the two spellings a reader
// types, and the binary beside the launcher that answers the callback, so a
// keystroke never runs the launcher and never builds.
var program = script.Program{Name: "le", Spellings: []string{"le", "./le"}, Binary: "bin/le"}

// table declares the area: one action per shell, and the callback.
func table() leaction.Area {
	actions := make([]leaction.Action, 0, len(script.Shells())+1)
	for _, shell := range script.Shells() {
		actions = append(actions, leaction.Action{
			Verb:   shell,
			Why:    textbuf.New().Str("print the ").Str(shell).Str(" script that completes le and ./le; its first lines say how to install it").String(),
			Answer: func() (any, int) { return scriptOf(shell) },
		})
	}
	actions = append(actions, leaction.Action{
		Verb: verbWords,
		Why: "print the candidates for the last word of an le command line, one `candidate<TAB>summary` record per line. " +
			"The scripts call it; every word after the verb is the line as typed, the last one the word being completed",
		AnswerWords: answerWords,
	})
	return leaction.New(name, actions...)
}

// scriptOf answers one shell's script, rendered as itself.
func scriptOf(shell string) (any, int) {
	text, err := script.Generate(shell, program)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Str(shell).Str(": ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
		return nil, 1
	}
	return scriptText(text), 0
}

// scriptText is a completion script, printed as it is.
type scriptText string

// Text renders the script.
func (s scriptText) Text() string { return string(s) }

var _ leroot.Prose = scriptText("")

// answerWords answers the candidates for words. A word le does not know is
// refused on stderr with exit 1 and nothing on stdout, so the shell, which
// discards stderr, offers nothing, and a person running the callback by hand
// reads which word was wrong.
func answerWords(words []string) (any, int) {
	candidates, err := leroot.Complete(words)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: completion words: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
		return nil, 1
	}
	return records(candidates), 0
}

// records is the callback's answer: one record per candidate, in the format
// every Ze completion script reads (script.WriteRecord).
type records []leaction.Candidate

// Text renders one record per candidate. A buffer cannot fail a write.
func (r records) Text() string {
	var tb textbuf.Buffer
	for _, candidate := range r {
		script.WriteRecord(&tb, candidate.Value, candidate.Summary) //nolint:errcheck // a textbuf.Buffer write never fails
	}
	return tb.String()
}

var _ leroot.Prose = records(nil)
