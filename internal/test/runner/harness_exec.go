// Design: docs/architecture/testing/runner-architecture.md -- how a .ci exec head resolves
// Related: runner_exec.go -- the run steps that exec a head
// Related: parsing.go -- the parse steps that exec a head
// Related: runner.go -- setupBinShims, which puts the same heads on a child's PATH

package runner

import (
	"errors"
	"os"
	"strings"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// The exec heads the runner answers with its own executable. Every harness
// command is `le test <name>` (spec-le-subject-first-command-tree, D-8),
// and the runner runs inside that `le`, so an `le` head runs the runner's own
// file rather than whatever `le` a PATH lookup finds. Any other head, the
// retired standalone harness names among them, is no head of the runner's: it
// reaches a PATH lookup, which finds no such program and fails the step.
const (
	binNameLE = "le"

	leTestWord     = "test"
	leTestPeerWord = "peer"

	// binNamePeer is the role name of a step that starts the harness peer,
	// whichever head spelled it. The peer's stdin route, port and success
	// barrier read this role.
	binNamePeer = "le test peer"
)

// launchesPeer answers whether the words of an exec value start the harness
// peer: `le test peer ...`. It reads the command words only, so a helper
// whose arguments mention the peer does not match.
func launchesPeer(fields []string) bool {
	if len(fields) < 3 {
		return false
	}
	return fields[0] == binNameLE && fields[1] == leTestWord && fields[2] == leTestPeerWord
}

// leHarnessArea answers whether the arguments of an `le` head name a harness
// command, `test <name>`. A harness command runs in the test's work directory,
// as the harness binary did, and `le test fixture` refuses the checkout root.
//
// The answer is derived from registration: under `test`, the harness commands
// are exactly the forwarding ones (internal/le/test/harnesstool), so a new
// harness command needs no edit here. In a process where no harness command is
// registered, every `le` head keeps the repository root.
func leHarnessArea(args []string) bool {
	if len(args) < 2 {
		return false
	}
	if args[0] != leTestWord {
		return false
	}
	return leroot.Forwards(leTestWord + " " + args[1])
}

// ownExecutable answers the file of the running process, which is the `le`
// that runs this suite. It never falls back to a PATH lookup: a caller that
// cannot name its own file refuses the exec line with this error.
func ownExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", errors.New("the runner does not know its own executable: " + err.Error())
	}
	return path, nil
}

// leBuildNameVariable is the variable the root launcher's --name option sets
// (cmd/ze/le_build_name.go). A child `le` reached through the `le` shim is not
// the named build file, so refuseWrongBuildName would refuse every one of them:
// no child environment carries it.
const leBuildNameVariable = "ZE_LE_BUILD_NAME"

// droppedFromChild answers whether an inherited environment entry name is one
// no test child receives. env.Get matches case-insensitively and reads a dot as
// an underscore, so every spelling of the key is dropped.
func droppedFromChild(name string) bool {
	return strings.EqualFold(strings.ReplaceAll(name, ".", "_"), leBuildNameVariable)
}
