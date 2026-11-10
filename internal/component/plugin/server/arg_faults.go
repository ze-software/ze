// Design: docs/architecture/api/commands.md -- every route validates
// Related: command.go -- Dispatch, which restates a list of faults before the selector guard
// Related: incomplete.go -- writeSelectorRefusal and writeSelectedPath, the selector forms used here
//
// arg_faults.go restates command.ValidateArgs's list of faults with what the
// dispatcher knows and the definitions do not: where the command's selector is
// typed, and the line that supplies a missing one. The fix line itself is
// command.ArgumentFaults.RestatedForLine's, the same on every route.

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
// refusal, which gives the line that supplies it. The fix line is chosen by
// command.ArgumentFaults.RestatedForLine, the one function every route calls,
// so a list of only missing arguments reads the same here as on every model
// route: the whole command to type. What the dispatcher adds is the head of
// that line, the path with the selector where Dispatch reads it
// (writeSelectedPath), `<selector>` when it is one of the missing; the adopted
// token is written there, so it is not written a second time among the typed
// tokens.
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

	selectorAbsent := false
	for _, fault := range faults.Faults() {
		missing, ok := errors.AsType[*command.MissingArgumentError](fault)
		if ok && missing.Leaf == selectorLeaf {
			selectorAbsent = true
		}
	}

	var head textbuf.Buffer
	placedToken, placedLeaf := "", ""
	switch {
	case selectorAbsent:
		writeSelectedPath(&head, matchedCmd, "<selector>")
		placedLeaf = selectorLeaf
	case selector != "":
		writeSelectedPath(&head, matchedCmd, selector)
		placedToken = adopted
	default:
		// No selector is held or missing: the command declares none, or the
		// operator typed it by keyword among the tokens written after the path.
		head.Str(matchedCmd.Name)
	}
	return faults.RestatedForLine(func(fault error) error {
		missing, ok := errors.AsType[*command.MissingArgumentError](fault)
		if ok && missing.Leaf == selectorLeaf && selectorLine != "" {
			return &selectorMissing{missing: missing, refusal: selectorLine}
		}
		// A fault restated as nil keeps its own line (ArgumentFaults.Restated).
		return nil
	}, head.String(), args, placedToken, placedLeaf)
}
