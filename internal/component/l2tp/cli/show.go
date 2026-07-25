// Design: docs/guide/l2tp.md -- offline CLI entry points

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// forward is the seam the daemon-forwarding verbs dispatch through. It is a
// variable so tests can observe the command string and user a verb resolves to
// without an SSH daemon; production always runs forwardToDaemon.
var forward = forwardToDaemon

// clientFlags parses the flags shared by the three daemon-forwarding verbs and
// returns the positional tokens that make up the daemon command.
//
// The verbs pass their remaining tokens to the daemon verbatim, so the FlagSet
// owns only --user/-u; everything else is the daemon's grammar.
func clientFlags(verb, usageLine string, args []string) (rest []string, user string, code int) {
	var tb textbuf.Buffer
	fs := flag.NewFlagSet(tb.Str("ze l2tp ").Str(verb).String(), flag.ContinueOnError)
	name := fs.String("user", "", "SSH login username (overrides zefs super-admin)")
	fs.StringVar(name, "u", "", "Short alias for --user")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, usageLine)
		fmt.Fprintln(os.Stderr, "  flags must precede the subcommand")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, "", 0
		}
		return nil, "", 1
	}
	rest = fs.Args()

	// Fail closed on a flag that survived into the positional tail. Go's flag
	// package stops parsing at the first non-flag token, so `show tunnels
	// --user alice` leaves `--user alice` here rather than binding the flag.
	//
	// The daemon rejects flag-shaped tokens too (Dispatch, internal/component/
	// plugin/server/command.go:599), so this is no longer the only line of
	// defense -- but it is the better one: it names the mistake before a round
	// trip, and it holds even against an older daemon. Both matter because
	// forwarding used to SUCCEED: matchCommandTokens (command.go:428) returns
	// unmatched trailing tokens as args and reports a match, and the l2tp CLI
	// containers carry no leaves, so the validator guarded at command.go:609
	// never ran and `show l2tp --user alice tunnels` printed the summary for
	// the DEFAULT user with exit 0.
	for _, a := range rest {
		if strings.HasPrefix(a, "-") && a != "-" {
			tb.Reset()
			fmt.Fprintln(os.Stderr, tb.Str("error: ").Quoted(a).Str(" must come before the subcommand").String())
			fs.Usage()
			return nil, "", 1
		}
	}
	return rest, *name, 0
}

// cmdShow forwards `ze l2tp show [subcmd] [args...]` to the daemon as
// the equivalent `show l2tp [subcmd] [args...]` text command. The
// response is printed verbatim (JSON from the daemon-side handlers in
// internal/component/l2tp/cmd/); pipe operators are available via
// `ze cli -c "..."` when an operator wants formatting.
func cmdShow(args []string) int {
	rest, user, code := clientFlags("show", "usage: ze l2tp show [--user <name>] [tunnels|tunnel <id>|sessions|statistics|...]", args)
	if code != 0 {
		return code
	}
	parts := append([]string{"show", "l2tp"}, rest...)
	return forward(textbuf.Join(parts, " "), user)
}

// cmdTunnelTeardown forwards `ze l2tp tunnel id <id>` or `ze l2tp tunnel all`
// to the daemon as the operational `clear l2tp tunnel …` command (clear already
// means tear down). `args` carries the subcommand (`id <id>` / `all`).
func cmdTunnelTeardown(args []string) int {
	rest, user, code := clientFlags("tunnel", "usage: ze l2tp tunnel [--user <name>] {id <id> | all}", args)
	if code != 0 {
		return code
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "error: ze l2tp tunnel {id <id> | all}")
		return 1
	}
	parts := append([]string{"clear", "l2tp", "tunnel"}, rest...)
	return forward(textbuf.Join(parts, " "), user)
}

// cmdSessionTeardown is the session-scoped counterpart of
// cmdTunnelTeardown.
func cmdSessionTeardown(args []string) int {
	rest, user, code := clientFlags("session", "usage: ze l2tp session [--user <name>] {id <id> | all}", args)
	if code != 0 {
		return code
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "error: ze l2tp session {id <id> | all}")
		return 1
	}
	parts := append([]string{"clear", "l2tp", "session"}, rest...)
	return forward(textbuf.Join(parts, " "), user)
}

// forwardToDaemon is the shared dispatch that loads SSH credentials,
// sends the text command, and prints the response. Errors distinguish
// "daemon not reachable" (exit 1, connection message) from "daemon
// returned an error payload" (exit 1, error message from payload).
func forwardToDaemon(command, user string) int {
	creds, err := sshclient.LoadCredentialsWithFlags(user)
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
