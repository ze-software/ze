// Design: docs/contributing/patching-the-standard-library.md -- the command that prints the patched GOROOT
//
// actions.go carries the table of `le go stdpatch`. The generator itself is
// internal/le/go/stdpatch, which imports only the standard library; this
// package is the le surface over it, so the generator stays buildable before
// le exists.

package stdpatchcmd

import (
	"github.com/ze-software/ze/internal/le/go/stdpatch"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "go stdpatch"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "goroot", Why: "print the patched GOROOT for this checkout's pinned toolchain, building it on first use, for plain go commands, editors and the routes that run before le exists",
		Answer: runGoroot},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le go stdpatch` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runGoroot prints the tree, or the generator's refusal with exit 1. It
// answers what le's own activation answered, so the path printed is the one
// every child of this le compiles with.
func runGoroot() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	tree, err := stdpatch.Activate(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return tree, 0
}
