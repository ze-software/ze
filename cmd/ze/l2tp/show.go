// Design: docs/architecture/l2tp.md -- offline CLI entry points

package l2tp

import (
	"fmt"
	"os"
	"strings"

	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
)

// cmdShow forwards `ze l2tp show [subcmd] [args...]` to the daemon as
// the equivalent `show l2tp [subcmd] [args...]` text command. The
// response is printed verbatim (JSON from the daemon-side handlers in
// internal/component/cmd/l2tp/); pipe operators are available via
// `ze cli -c "..."` when an operator wants formatting.
func cmdShow(args []string) int {
	parts := append([]string{"show", "l2tp"}, args...)
	return forwardToDaemon(strings.Join(parts, " "))
}

// cmdTunnelTeardown forwards `ze l2tp tunnel teardown <id>` or
// `ze l2tp tunnel teardown-all` to the daemon. `args` carries the
// subcommand (`teardown` / `teardown-all`) plus any positional args
// (the tunnel id for the single-target form).
func cmdTunnelTeardown(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: ze l2tp tunnel <teardown|teardown-all> [<id>]")
		return 1
	}
	parts := append([]string{"l2tp", "tunnel"}, args...)
	return forwardToDaemon(strings.Join(parts, " "))
}

// cmdSessionTeardown is the session-scoped counterpart of
// cmdTunnelTeardown.
func cmdSessionTeardown(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: ze l2tp session <teardown|teardown-all> [<id>]")
		return 1
	}
	parts := append([]string{"l2tp", "session"}, args...)
	return forwardToDaemon(strings.Join(parts, " "))
}

// forwardToDaemon is the shared dispatch that loads SSH credentials,
// sends the text command, and prints the response. Errors distinguish
// "daemon not reachable" (exit 1, connection message) from "daemon
// returned an error payload" (exit 1, error message from payload).
func forwardToDaemon(command string) int {
	creds, err := sshclient.LoadCredentialsWithFlags("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr, "hint: is ze running? start with: ze hub")
		return 1
	}
	resp, err := sshclient.ExecCommand(creds, command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if resp != "" {
		fmt.Println(resp)
	}
	return 0
}
