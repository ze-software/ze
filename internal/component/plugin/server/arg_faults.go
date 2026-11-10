// Design: docs/architecture/api/commands.md -- every route validates
// Related: command.go -- Dispatch, which restates a list of faults before the selector guard
// Related: incomplete.go -- writeSelectorRefusal and writeSelectedPath, the selector forms used here
//
// arg_faults.go restates command.ValidateArgs's list of faults with what the
// dispatcher knows and the definitions do not: the command's path, where its
// selector is typed, and the tokens the operator typed.

package server

import (
	"errors"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// selectorMissing stands in a list of faults for the missing selector of a
// command that requires one. It reads as the selector guard's own refusal
// (writeSelectorRefusal), so the list names the line that supplies the
// selector, and it unwraps to the *command.MissingArgumentError it replaces,
// so errors.As still finds that. Immutable, safe for concurrent use.
type selectorMissing struct {
	missing *command.MissingArgumentError
	refusal string
}

// Error answers the guard's refusal text.
func (e *selectorMissing) Error() string {
	return e.refusal
}

// Unwrap answers the missing argument this line names.
func (e *selectorMissing) Unwrap() error {
	return e.missing
}

// restateArgumentFaults answers faults, the list command.ValidateArgs refused
// matchedCmd's line with, as the operator should read it.
//
// The missing selector of a command that requires one is named by the guard's
// refusal, which gives the line that supplies it. When every fault is a
// missing argument and the list omitted none, the fix line is the whole
// command to type (missingArgumentsFix); otherwise it stays the general
// correction, because a fault in a typed word has no form the dispatcher can
// write for the operator.
//
// args are the tokens after the command path, selector the selector value
// Dispatch holds ("" for none), and adopted the token of args Dispatch took as
// that selector ("" when it took none).
func restateArgumentFaults(matchedCmd *Command, faults *command.ArgumentFaults, args []string, selector, adopted string) *command.ArgumentFaults {
	selectorLine := ""
	if matchedCmd.RequiresSelector {
		var tb textbuf.Buffer
		writeSelectorRefusal(&tb, matchedCmd)
		selectorLine = tb.String()
	}

	listed := faults.Faults()
	onlyMissing := faults.Omitted() == 0
	selectorAbsent := false
	for _, fault := range listed {
		missing, ok := errors.AsType[*command.MissingArgumentError](fault)
		if !ok {
			onlyMissing = false
			continue
		}
		if missing.Leaf == selectorLeaf {
			selectorAbsent = true
		}
	}

	fix := ""
	if onlyMissing {
		fix = missingArgumentsFix(matchedCmd, listed, args, selector, adopted, selectorAbsent)
	}
	return faults.Restated(func(fault error) error {
		missing, ok := errors.AsType[*command.MissingArgumentError](fault)
		if ok && missing.Leaf == selectorLeaf && selectorLine != "" {
			return &selectorMissing{missing: missing, refusal: selectorLine}
		}
		// A fault restated as nil keeps its own line (ArgumentFaults.Restated).
		return nil
	}, fix)
}

// missingArgumentsFix writes the whole command a line needs when its only
// faults are missing arguments: the path with the selector where Dispatch
// reads it (writeSelectedPath), `<selector>` when it is one of the missing,
// then every token the operator typed, then each other missing leaf by its
// keyword (command.MissingArgumentError.Usage). Every token typed was valid,
// so the line it writes is the operator's own line completed.
func missingArgumentsFix(matchedCmd *Command, faults []error, args []string, selector, adopted string, selectorAbsent bool) string {
	var tb textbuf.Buffer
	typed := args
	switch {
	case selectorAbsent:
		writeSelectedPath(&tb, matchedCmd, "<selector>")
	case selector != "":
		// The adopted token is written where the selector is read, so it is
		// not written a second time among the typed tokens.
		writeSelectedPath(&tb, matchedCmd, selector)
		typed = withoutToken(args, adopted)
	default:
		// No selector is held or missing: the command declares none, or the
		// operator typed it by keyword among the tokens written below.
		tb.Str(matchedCmd.Name)
	}
	for _, token := range typed {
		tb.Byte(' ')
		command.WriteInvocationValue(&tb, token)
	}
	for _, fault := range faults {
		missing, ok := errors.AsType[*command.MissingArgumentError](fault)
		if ok && missing.Leaf != selectorLeaf {
			tb.Byte(' ').Str(missing.Usage())
		}
	}
	return tb.String()
}

// withoutToken answers args without the first token equal to token, and args
// itself when token is empty or not among them. args is not changed.
func withoutToken(args []string, token string) []string {
	if token == "" {
		return args
	}
	for i, arg := range args {
		if arg == token {
			out := make([]string, 0, len(args)-1)
			out = append(out, args[:i]...)
			return append(out, args[i+1:]...)
		}
	}
	return args
}
