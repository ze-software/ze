// Design: docs/architecture/behavior/signals.md — signal handling CLI
//
// Package signal provides the `ze signal` and `ze status` CLI commands.
// Commands are sent to the daemon via SSH. Status is checked by dialing
// the SSH port (TCP connect).
package signal

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// Exit codes for signal command.
const (
	ExitSuccess       = 0
	ExitNotRunning    = 1
	ExitNoCredentials = 2
	ExitSignalFailed  = 4
)

// Default SSH connection target.
const (
	defaultHost = "127.0.0.1"
	defaultPort = "2222"
	dialTimeout = 2 * time.Second
)

// Command describes a signal subcommand.
type Command struct {
	Name        string // CLI name (e.g., "reload")
	ExecCommand string // SSH exec command sent to daemon (e.g., "daemon reload")
	Description string // One-line help text
	Signal      string // Equivalent OS signal (for documentation; empty if none)
}

// commands is the registered list of signal subcommands.
// Help text, dispatch, suggestions, and shell completions are all derived from this list.
var commands = []Command{
	{"reload", "daemon reload", "Reload configuration", "SIGHUP"},
	{"stop", "stop", "Graceful shutdown (no GR marker)", "SIGTERM"},
	{"restart", "restart", "Graceful restart (writes GR marker, then shuts down)", ""},
	{"status", "daemon status", "Dump daemon status", "SIGUSR1"},
	{"quit", "daemon quit", "Goroutine dump + immediate exit", "SIGQUIT"},
}

// Commands returns a copy of the registered signal subcommands.
func Commands() []Command {
	out := make([]Command, len(commands))
	copy(out, commands)
	return out
}

// Names returns the CLI names of all registered commands.
func Names() []string {
	names := make([]string, len(commands))
	for i, c := range commands {
		names[i] = c.Name
	}
	return names
}

// lookup returns the Command for the given CLI name, or nil.
func lookup(name string) *Command {
	for i := range commands {
		if commands[i].Name == name {
			return &commands[i]
		}
	}
	return nil
}

// Run executes the ze signal command with the given arguments.
// Returns an exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("signal", flag.ContinueOnError)
	host := fs.String("host", "", "SSH host (default from env or 127.0.0.1)")
	port := fs.String("port", "", "SSH port (default from env or 2222)")

	fs.Usage = func() {
		cmdEntries := make([]helpfmt.HelpEntry, len(commands))
		for i, cmd := range commands {
			cmdEntries[i] = helpfmt.HelpEntry{Name: cmd.Name, Desc: cmd.Description}
		}
		p := helpfmt.Page{
			Command: "ze signal",
			Summary: "Send commands to a running Ze daemon via SSH",
			Usage:   []string{"ze signal <command> [options]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Commands", Entries: cmdEntries},
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--host", Desc: "SSH host (default from env or 127.0.0.1)"},
					{Name: "--port", Desc: "SSH port (default from env or 2222)"},
				}},
				{Title: "Environment", Entries: []helpfmt.HelpEntry{
					{Name: "ze_ssh_host / ze.ssh.host", Desc: "Override connection host"},
					{Name: "ze_ssh_port / ze.ssh.port", Desc: "Override connection port"},
				}},
			},
			Examples: []string{
				"ze signal reload",
				"ze signal stop",
				"ze signal stop --host 10.0.0.1 --port 2222",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		return ExitNotRunning
	}

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <command>\n")
		fs.Usage()
		return ExitNotRunning
	}

	name := remaining[0]

	// Resolve SSH target
	h := resolveHost(*host)
	p := resolvePort(*port)
	addr := net.JoinHostPort(h, p)

	cmd := lookup(name)
	if cmd == nil {
		fmt.Fprintf(os.Stderr, "unknown signal command: %s\n", name)
		if s := suggest.Command(name, Names()); s != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
		}
		fs.Usage()
		return ExitNotRunning
	}
	return cmdSSHExec(addr, cmd.ExecCommand)
}

// RunStatus executes the ze status command with the given arguments.
// Checks daemon liveness by dialing the SSH port.
// Returns an exit code.
func RunStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	host := fs.String("host", "", "SSH host (default from env or 127.0.0.1)")
	port := fs.String("port", "", "SSH port (default from env or 2222)")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze status",
			Summary: "Check if a Ze daemon is running by dialing the SSH port",
			Usage:   []string{"ze status [options]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Exit codes", Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: "Daemon is running (SSH port reachable)"},
					{Name: "1", Desc: "Daemon is not running"},
				}},
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--host", Desc: "SSH host (default from env or 127.0.0.1)"},
					{Name: "--port", Desc: "SSH port (default from env or 2222)"},
				}},
				{Title: "Environment", Entries: []helpfmt.HelpEntry{
					{Name: "ze_ssh_host / ze.ssh.host", Desc: "Override connection host"},
					{Name: "ze_ssh_port / ze.ssh.port", Desc: "Override connection port"},
				}},
			},
			Examples: []string{
				"ze status",
				"ze status --host 10.0.0.1 --port 2222",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		return ExitNotRunning
	}

	// Resolve SSH target
	h := resolveHost(*host)
	p := resolvePort(*port)
	addr := net.JoinHostPort(h, p)

	// Dial SSH port to check liveness
	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon is not running (%s)\n", addr)
		return ExitNotRunning
	}
	conn.Close() //nolint:errcheck // probe connection

	fmt.Printf("daemon is running (%s)\n", addr)
	return ExitSuccess
}

// cmdSSHExec sends a command to the daemon via SSH exec.
func cmdSSHExec(addr, command string) int {
	creds, err := sshclient.LoadCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: run 'ze init' to set up credentials\n")
		return ExitNoCredentials
	}

	// Override credentials' host/port with flag-specified addr
	creds.Host, creds.Port, _ = net.SplitHostPort(addr)

	result, err := sshclient.ExecCommand(creds, command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return ExitSignalFailed
	}
	if result != "" {
		fmt.Println(result)
	}
	return ExitSuccess
}

// resolveHost returns the SSH host from flag, env var, or default.
// env.Get normalizes dot/underscore and case automatically, so a single
// call handles ze.ssh.host, ze_ssh_host, ZE_SSH_HOST, etc.
func resolveHost(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := env.Get("ze.ssh.host"); v != "" {
		return v
	}
	return defaultHost
}

// resolvePort returns the SSH port from flag, env var, or default.
// env.Get normalizes dot/underscore and case automatically, so a single
// call handles ze.ssh.port, ze_ssh_port, ZE_SSH_PORT, etc.
func resolvePort(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := env.Get("ze.ssh.port"); v != "" {
		return v
	}
	return defaultPort
}
