// Design: docs/guide/l2tp.md -- offline CLI entry points

package cli

import (
	"fmt"
	"os"

	sshclient "codeberg.org/thomas-mangin/ze/internal/core/ssh/client"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// cmdShow forwards `ze l2tp show [subcmd] [args...]` to the daemon as
// the equivalent `show l2tp [subcmd] [args...]` text command. The
// response is printed verbatim (JSON from the daemon-side handlers in
// internal/component/l2tp/cmd/); pipe operators are available via
// `ze cli -c "..."` when an operator wants formatting.
func cmdShow(args []string) int {
	parts := append([]string{"show", "l2tp"}, args...)
	return forwardToDaemon(textbuf.Join(parts, " "))
}

// cmdTunnelTeardown forwards `ze l2tp tunnel id <id>` or `ze l2tp tunnel all`
// to the daemon as the operational `clear l2tp tunnel …` command (clear already
// means tear down). `args` carries the subcommand (`id <id>` / `all`).
func cmdTunnelTeardown(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: ze l2tp tunnel {id <id> | all}")
		return 1
	}
	parts := append([]string{"clear", "l2tp", "tunnel"}, args...)
	return forwardToDaemon(textbuf.Join(parts, " "))
}

// cmdSessionTeardown is the session-scoped counterpart of
// cmdTunnelTeardown.
func cmdSessionTeardown(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: ze l2tp session {id <id> | all}")
		return 1
	}
	parts := append([]string{"clear", "l2tp", "session"}, args...)
	return forwardToDaemon(textbuf.Join(parts, " "))
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
