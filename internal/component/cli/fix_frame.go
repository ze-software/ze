// Design: docs/architecture/cli/error-surface.md -- The operator error
// Overview: model.go -- operatorPosition, the position a fix is framed for
// Related: model_commands_commit.go -- CommitRefusal and the issue lists frame their fixes here
// Related: validator.go -- ConfigValidationError carries the typed fix

package cli

import (
	"errors"
	"slices"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// FixCommand is an editor command a refusal proposes as its fix: Verb (set,
// delete, rename, copy, insert), At the position its Args are relative to
// (nil for a path from the root) and Args. It holds no position of its own
// to print: FrameFix writes it for the position the operator types it at.
type FixCommand struct {
	Verb string
	At   []string
	Args []string
	// placeholders names Args that are correction syntax, not literal data.
	// It is set by the producer, never guessed from the token's punctuation.
	placeholders []int
}

func (fix FixCommand) withPlaceholders(indexes ...int) FixCommand {
	fix.placeholders = slices.Concat(fix.placeholders, indexes)
	return fix
}

// FrameFix writes fix as the exact command the operator types at operator,
// their position: `<verb> <args>` when operator is At (the command typed
// where the operator is keeps its relative form), `<verb> <At> <args>` when
// operator is the root, and `top <verb> <At> <args>` otherwise, which is exact
// from any position. Every editor fix line is written here, so every surface
// frames a fix by the same rule.
func FrameFix(fix FixCommand, operator []string) string {
	var tb textbuf.Buffer
	if slices.Equal(fix.At, operator) {
		tb.Str(fix.Verb)
		appendWords(&tb, fix.Args, fix.placeholders)
		return tb.String()
	}
	if len(operator) > 0 {
		tb.Str(cmdTop).Byte(' ')
	}
	tb.Str(fix.Verb)
	appendWords(&tb, fix.At, nil)
	appendWords(&tb, fix.Args, fix.placeholders)
	return tb.String()
}

// appendWords quotes each concrete token with the config codec, while explicit
// placeholder arguments keep the replacement syntax the operator fills in.
func appendWords(tb *textbuf.Buffer, words []string, placeholders []int) {
	for i, word := range words {
		tb.Byte(' ')
		if slices.Contains(placeholders, i) {
			tb.Str(word)
		} else {
			tb.Str(configtext.QuoteCommandWord(word))
		}
	}
}

// correctionFix replaces exactly one argument with the schema's correction,
// retaining its distinction between a concrete value and a placeholder.
func correctionFix(verb string, at, args []string, index int, refusal *configyang.ValidationError) FixCommand {
	corrected := slices.Clone(args)
	corrected[index] = refusal.Correction()
	fix := FixCommand{Verb: verb, At: at, Args: corrected}
	if refusal.CorrectionIsPlaceholder() {
		fix = fix.withPlaceholders(index)
	}
	return fix
}

// valueRefusal answers the operator refusal err holds: a value refusal the
// schema built (enum, union, range, length, pattern), whose fix a frame may
// replace. Any other error answers false and keeps its own text.
func valueRefusal(err error) (*configyang.ValidationError, bool) {
	var refusal *configyang.ValidationError
	if !errors.As(err, &refusal) {
		return nil, false
	}
	holds := refusal.HoldsOperatorError()
	return refusal, holds
}

// FramedSetRefusal answers err with its fix replaced by the `set` command the
// operator typed, args being the words after `set` relative to at and the
// last one the refused value, corrected: the suggestion when one value is
// close, else the alternation or the placeholder. An error that holds no
// value refusal is answered as it is.
func FramedSetRefusal(err error, at, args, operator []string) error {
	refusal, ok := valueRefusal(err)
	if !ok {
		return err
	}
	fix := correctionFix(cmdSet, at, args, len(args)-1, refusal)
	return refusal.WithFix(FrameFix(fix, operator))
}
