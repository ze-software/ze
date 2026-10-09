// Design: docs/architecture/api/commands.md — command argument definitions
// Related: ../argdef.go — the constructors this helper wraps

// Package commandtest holds test support for code that builds command
// argument definitions. Only test files import it.
package commandtest

import "github.com/ze-software/ze/internal/component/command"

// Must answers def, and panics when the constructor that produced it refused.
// A test's definition is literal data, so a refusal is a defect in the test.
func Must(def command.ArgDef, err error) command.ArgDef {
	if err != nil {
		panic("BUG: test argument definition refused: " + err.Error())
	}
	return def
}

// Args answers tokens judged by command.ValidateArgs against no definitions:
// the value every route hands a handler for a command whose model declares no
// arguments. A test that calls a handler directly builds its value here, so it
// goes through the one producer rather than around it. It panics when the
// judgment refuses, which against no definitions is a Ze defect.
func Args(tokens ...string) command.ValidatedArgs {
	validated, err := command.ValidateArgs(tokens, nil, nil)
	if err != nil {
		panic("BUG: tokens refused against no definitions: " + err.Error())
	}
	return validated
}
