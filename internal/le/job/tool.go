// Design: docs/contributing/running-commands.md -- `le exec` takes a job slot by the program it carries
// Overview: central.go -- the central admission this reuses: the policy, the wait, the one slot
// Related: ../exec/exec.go -- holdHeavy, the only caller
//
// `le exec` replaces its own process with the program it carries, so the
// dispatcher cannot hold a slot for it: nothing would remain to release one.
// For a heavy tool (internal/le/heavytool) it asks this file instead, which
// takes the slot the way the dispatcher does and runs the tool as le's child,
// so le is still there to release the slot with the tool's code.

package job

import (
	"io"
	"os"
	"os/exec"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// Tool is one program ready to run: the resolved file, its argv with the name
// the caller typed as argv[0], and its whole environment.
type Tool struct {
	Path    string
	Argv    []string
	Environ []string
}

// HoldTool runs tool, a heavy tool `le exec` carries, as a child inside one
// job slot, when it needs one. commandName is the name its slot is held
// under (`exec go test` holds `exec-go-test`), and line the le line as typed,
// for the entry and the waiter record.
//
// held reports whether HoldTool answered for the line. When it is false the
// tool needs no slot here and nothing ran: no checkout exists, so there is
// no registry to share, or a live parent already holds the slot the tool
// runs inside. The caller then replaces its process with the tool, as it does
// for a light program, and code is meaningless. When it is true, code is the
// line's: the tool's exit code (a death by signal as gaterun.ExitCode answers
// it), 130 for a wait a signal ended, or 2 for a slot that could not be asked
// for, with its refusal written to stderr.
//
// The wait, the claim and the release are the dispatcher's own
// (admitCentral): the waiter record, the banner that names the holder, the
// unshared claim, and the parent named for every process the tool starts. The
// child inherits le's stdin, stdout and stderr, so a terminal stays a
// terminal, and every SIGINT, SIGTERM, SIGHUP and SIGQUIT sent to le is
// forwarded to it until it exits (forwardSignals). NOT safe for concurrent
// use: it names the parent in the process environment.
func HoldTool(commandName string, line []string, tool Tool) (code int, held bool) {
	point := centralPoint{root: lepath.Root, policy: NewIn, waitRoot: signalRoot, errOut: os.Stderr}
	return point.holdTool(commandName, line, tool, point.runTool)
}

// holdTool is HoldTool over the point's sources, with run starting the child,
// so a test can run none.
func (c centralPoint) holdTool(commandName string, line []string, tool Tool, run func(Tool) int) (int, bool) {
	adm, outcome, code := c.policyFor(line)
	switch outcome {
	case policyNoCheckout:
		return 0, false
	case policyRefused:
		return code, true
	case policyFound:
	case policyUnspecified:
		panic("BUG: job: the tool policy reading answered no outcome")
	default:
		panic("BUG: job: the tool policy reading answered an unknown outcome")
	}

	// A parent entry that cannot be read is not judged here: the wait below
	// reads it again and refuses the line, as it does for any central line.
	if _, inside, err := adm.insideParent(); err == nil && inside {
		return 0, false
	}
	return c.hold(adm, commandName, line, func() int {
		return run(toolInside(tool))
	}), true
}

// toolInside answers tool with the job this process holds, or runs inside,
// named as its parent: hold named it in this process's environment
// (NameParent), and tool's environment was taken before that. Every other
// spelling of ParentKey is removed, as ChildEnviron removes it.
func toolInside(tool Tool) Tool {
	var tb textbuf.Buffer
	named := tb.Str(ParentVariable).Byte('=').Str(env.Get(ParentKey)).String()
	tool.Environ = append(env.Without(tool.Environ, ParentKey), named)
	return tool
}

// runTool starts tool as this process's child with its streams, forwards the
// signals forwardSignals names to it, and answers its exit code. A tool that cannot
// start answers gaterun.CannotStart, with the reason written to the point's
// errOut.
func (c centralPoint) runTool(tool Tool) int {
	cmd := &exec.Cmd{
		Path:   tool.Path,
		Args:   tool.Argv,
		Env:    tool.Environ,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if err := cmd.Start(); err != nil {
		if c.errOut != nil {
			var tb textbuf.Buffer
			message := tb.Str("error: le exec: ").Str(tool.Path).Str(" could not be started inside its job slot: ").Err(err).
				Str("\nThe slot is released with the cannot-start code, so nothing ran.").
				Str("\nRun the program directly to see the system's own error, then reinstall the tool and run ./le setup.\n").String()
			io.WriteString(c.errOut, message) //nolint:errcheck // CLI output
		}
		return gaterun.CannotStart
	}

	stop := forwardSignals(cmd)
	err := cmd.Wait()
	stop()

	return gaterun.ExitCode(err)
}
