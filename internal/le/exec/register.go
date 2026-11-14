// Design: docs/guide/developer-setup.md -- le exec: the launcher the tool shims run
// Related: exec.go -- the environment, the GOROOT rule and the refusals

package leexec

import (
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/heavytool"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// name is the le command this package registers.
const name = "exec"

func init() {
	leroot.Register(name, leroot.GroupWorkflow, answer, registry.Meta{
		Description: "Run a program with its caches under the per-user cache directory, and with Ze's patched GOROOT active when it runs inside a checkout.",
		ShortHelp:   "run a program with Ze's caches and GOROOT",
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the program path is the program's own command line, so
	// a trailing help word reaches the program and it prints its own help.
	leroot.RegisterForwarding(name, "the program its first word names")
	// The area runs the program its first word names and prints that
	// program's output, so it carries a command and holds no verdict of its
	// own (leroot.CarrierLine).
	leroot.RegisterCarrier(name)
	// A line is heavy by the program it carries, and only then, so a tool
	// shim's gopls and `le exec go version` are light. A heavy line takes its
	// own slot (holdHeavy, exec.go), because the dispatcher's slot would be
	// released when the body returns, and this body replaces le with the
	// program for every light program.
	leroot.RegisterCarrierJudge(name, carriesHeavy)
}

// carriesHeavy reports whether carried, the words after `le exec`, start
// heavy work: a heavy tool by the one reading and definition the Bash hook
// applies to a raw command (heavytool.Started, which reads through the
// assignments, the launchers and the Go toolchain's flag in front of the
// verb), or an le line that is heavy itself, read through the same words in
// front of it (heavytool.Program, heavytool.LeWords, leroot.HeavyLine). A
// carried le line is judged by its own declaration because that le admits
// itself; exec replaces its process with it and holds nothing (holdHeavy).
// A line handed to a shell with `-c` is heavy whatever it runs
// (heavytool.StartedUnread): reading it needs a shell parser, which the
// shared reading does not hold, and holdHeavy takes a slot by the same
// answer, so the hook and the slot agree.
func carriesHeavy(carried []string) bool {
	if _, heavy := heavytool.StartedUnread(carried); heavy {
		return true
	}
	words, typed := heavytool.LeWords(heavytool.Program(carried))
	if !typed {
		return false
	}
	return leroot.HeavyLine(words)
}

// answer is `le exec <path> [args...]`. For a light program, on success, it
// never returns: le becomes the program. For a heavy tool that takes a slot
// it answers the tool's exit code (holdHeavy). It answers 0 for a help
// request, 2 for a usage error, and 1 for a program it refuses to run.
func answer(args []string) (any, int) {
	return nil, run(args, os.Stdout, os.Stderr)
}
