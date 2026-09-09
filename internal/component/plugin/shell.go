// Design: docs/architecture/api/process-protocol.md -- how an external plugin is started
// Related: declarations.go -- runQuery, the declaration query that forks this shell
// Related: process/process.go -- (*Process).startExternal, the live start that forks it
// Related: doctor/check_shell.go -- the doctor check that reports the shell absent
//
// An external plugin is started by giving the operator's `run` string to a
// shell, so the shell is a runtime dependency of every external plugin. A
// gokrazy appliance image carries no shell utilities, so the dependency is
// absent there and no external plugin starts. This file holds the shell and
// the probe once, because the two call sites that fork it and the doctor check
// that reports it must name the same file.

package plugin

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Shell is the interpreter an external plugin's `run` string is given to. One
// constant serves the live start and the declaration query, because a plugin
// the daemon can start must be startable by the query.
const Shell = "/bin/sh"

// ShellAvailable answers nil when this host carries a Shell a fork can execute,
// and an error naming the shell when it does not. A caller that reports the
// error tells the operator which dependency is missing, rather than which
// plugin failed.
func ShellAvailable() error {
	return shellAvailable(Shell)
}

// shellAvailable takes the shell as a parameter so a test states which file the
// probe looks for. No test can take /bin/sh away from the host it runs on.
//
// The probe asks what the fork asks: a file at that path, and permission for
// THIS process to execute it. A path that exists and cannot be executed fails
// every plugin start while a probe reading existence alone reports the host
// healthy, which sends the operator to the plugin rather than to the shell
// (ai/rules/principles.md). A directory is the common shape of that, and
// os.Stat succeeds on one, so the kind is asked before the permission:
// unix.Faccessat answers yes for a directory, which every process may search.
//
// The permission question is put to the kernel rather than read off the mode
// bits, because a mode carries three answers and only one of them is ze's. A
// shell with mode 0700 owned by root has an execute bit set and cannot be
// executed by an unprivileged ze, so a probe reading the bits passes a host
// where every plugin start fails. AT_EACCESS asks for the EFFECTIVE user, the
// one execve tests, rather than the real user access(2) tests by default.
func shellAvailable(shell string) error {
	info, err := os.Stat(shell)
	if err != nil {
		return fmt.Errorf("the shell %s that starts an external plugin is absent: %w", shell, err)
	}
	if info.IsDir() {
		return fmt.Errorf("the shell %s that starts an external plugin is a directory", shell)
	}
	if err := unix.Faccessat(unix.AT_FDCWD, shell, unix.X_OK, unix.AT_EACCESS); err != nil {
		return fmt.Errorf("the shell %s that starts an external plugin cannot be executed by uid %d: its mode is %s: %w",
			shell, os.Geteuid(), info.Mode().Perm(), err)
	}
	return nil
}
