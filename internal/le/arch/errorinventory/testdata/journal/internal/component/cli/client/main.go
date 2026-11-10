// Fixture: the local CLI client's daemon hint, before and after its fix
// (plan/journal/error-names-no-fix-command.md, 2026-10-15).
package client

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/opererr"
)

// runBefore is the pre-fix form: a question, not a fix.
func runBefore() int {
	fmt.Fprintln(os.Stderr, "hint: is the daemon running?") // shape:cli-before
	return 1
}

// runAfter is the fixed form: the operator error naming the endpoint and the
// command that checks or starts the daemon.
func runAfter(host, port string, err error) int {
	fmt.Fprintln(os.Stderr, "error:", daemonUnreachable(host, port, err)) // shape:cli-after
	return 1
}

// daemonUnreachable is the fixed refusal.
func daemonUnreachable(host, port string, err error) error {
	return opererr.Wrap(err,
		"ze cli cannot reach the daemon at "+host+":"+port,
		"ze cli sends every daemon command over SSH to that address, and the connection failed",
		"ze status (and ze start <config> when no daemon runs)")
}
