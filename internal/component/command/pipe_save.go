// Design: docs/architecture/api/commands.md — CLI pipe operators
// Detail: pipe_catalog.go — the operator contract this implements
// Related: plan/spec-cli-pipe-operator-coverage.md — AC-7
//
// pipe_save.go implements `| save <path>`, the one global operation the owner
// named that did not exist. Nothing in the CLI wrote an answer to a file: a
// caller redirected the process's stdout from the shell, which works for
// `ze cli -c` and for an SSH exec channel, and is unavailable inside an
// interactive session, where the answer is drawn to a terminal and never
// reaches a pipe. That session is exactly where an operator needs it.
//
// WHERE IT IS ALLOWED, and why the default is refusal.
//
// The daemon expands the pipe chain on behalf of whoever connected, for the SSH
// exec channel and for every web surface. A `| save` honored there would write
// on the DAEMON's filesystem with the DAEMON's privileges, at a path the remote
// caller chose. That is a write primitive handed to anyone who can reach the
// CLI, so it is refused on those surfaces by name.
//
// It is honored where the process expanding the chain is the one the operator
// started: the interactive client, the TUI monitors, and `ze pipe`. The file is
// then written as that operator, by their own process, and the operating
// system's permissions are the whole answer to what they may write. Nothing is
// lost on the refused surfaces, because a shell redirect already works there.

package command

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// saveFileMode is the mode a saved answer is created with. An answer can carry
// peer addresses, keys and topology, so it is readable by its owner alone
// rather than by every account on the box.
const saveFileMode = 0o600

// savePathsInChain answers the paths a chain asks the answer to be written to.
func savePathsInChain(ops []pipeOp) []string {
	var paths []string
	for _, op := range ops {
		if op.kind == pipeSave {
			paths = append(paths, op.arg)
		}
	}
	return paths
}

// validateSaveOps refuses a `| save` that names no path, and refuses one
// entirely when the chain is being expanded for a remote caller.
func validateSaveOps(ops []pipeOp, allowed bool) string {
	for _, op := range ops {
		if op.kind != pipeSave {
			continue
		}
		if op.arg == "" {
			return "save requires a path to write to"
		}
		if !allowed {
			var tb textbuf.Buffer
			return tb.Str("save is refused here: this chain is expanded by the daemon, ").
				Str("so the file would be written on the daemon's filesystem with the ").
				Str("daemon's privileges. Redirect the output from your shell instead").String()
		}
	}
	return ""
}

// applySaves writes the finished answer to every path the chain named.
//
// It runs after the whole chain, wherever `| save` sat in it, because what an
// operator means by saving is the answer they are looking at: the formatted
// one. The configured default format is appended to the END of a chain that
// names none, so a `| save` applied in place would write the dispatcher's JSON
// and the terminal would show something else.
//
// The write is atomic: a temporary file in the same directory, then a rename.
// A failure therefore leaves the destination as it was rather than truncated,
// which matters because the usual failure is a path the operator cannot write
// and the usual destination is a file they still want.
func applySaves(answer string, paths []string) string {
	for _, path := range paths {
		if msg := saveAnswer(answer, path); msg != "" {
			return msg
		}
	}
	return ""
}

func saveAnswer(answer, path string) string {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ze-save-")
	if err != nil {
		return saveError(path, err)
	}
	tmpName := tmp.Name()

	// Every step's error is kept rather than discarded: a Close that fails is
	// how a full disk reports itself, and dropping it would report a saved
	// answer that is short.
	_, writeErr := tmp.WriteString(answer)
	chmodErr := tmp.Chmod(saveFileMode)
	closeErr := tmp.Close()
	if err = firstError(writeErr, chmodErr, closeErr); err != nil {
		return discardTemp(path, tmpName, err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return discardTemp(path, tmpName, err)
	}
	return ""
}

// firstError answers the first of the errors that is not nil.
func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// discardTemp removes the half-written file and answers why the save failed.
//
// A failure to remove it is reported too. Nothing can act on it automatically,
// but a file the operator did not ask for is now on their disk and the message
// is the only thing that will ever tell them.
func discardTemp(path, tmpName string, cause error) string {
	msg := saveError(path, cause)
	if err := os.Remove(tmpName); err != nil && !os.IsNotExist(err) {
		var tb textbuf.Buffer
		return tb.Str(msg).Str(" (and ").Str(tmpName).
			Str(" could not be removed: ").Str(err.Error()).Str(")").String()
	}
	return msg
}

// saveError says which path could not be written and why, because the reader's
// next action depends on which it was: a missing directory, a permission, or a
// full disk.
func saveError(path string, err error) string {
	var tb textbuf.Buffer
	return tb.Str("save could not write ").Str(path).Str(": ").Str(err.Error()).String()
}
