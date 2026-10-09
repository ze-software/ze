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
