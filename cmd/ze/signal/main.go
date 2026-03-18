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

	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
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

// Run executes the ze signal command with the given arguments.
// Returns an exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("signal", flag.ContinueOnError)
	host := fs.String("host", "", "SSH host (default from env or 127.0.0.1)")
	port := fs.String("port", "", "SSH port (default from env or 2222)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze signal <command> [options]

Send commands to a running Ze daemon via SSH.

Commands:
  reload    Reload configuration
  stop      Graceful shutdown
  quit      Goroutine dump + immediate exit

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Environment:
  ze_ssh_host / ze.ssh.host   Override connection host
  ze_ssh_port / ze.ssh.port   Override connection port

Examples:
  ze signal reload
  ze signal stop
  ze signal stop --host 10.0.0.1 --port 2222
`)
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

	command := remaining[0]

	// Resolve SSH target
	h := resolveHost(*host)
	p := resolvePort(*port)
	addr := net.JoinHostPort(h, p)

	switch command {
	case "reload", "stop", "quit":
		return cmdSSHExec(addr, command)
	default:
		fmt.Fprintf(os.Stderr, "unknown signal command: %s\n", command)
		if s := suggest.Command(command, []string{"reload", "stop", "quit"}); s != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
		}
		fs.Usage()
		return ExitNotRunning
	}
}

// RunStatus executes the ze status command with the given arguments.
// Checks daemon liveness by dialing the SSH port.
// Returns an exit code.
func RunStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	host := fs.String("host", "", "SSH host (default from env or 127.0.0.1)")
	port := fs.String("port", "", "SSH port (default from env or 2222)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze status [options]

Check if a Ze daemon is running by dialing the SSH port.

Exit codes:
  0  Daemon is running (SSH port reachable)
  1  Daemon is not running

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Environment:
  ze_ssh_host / ze.ssh.host   Override connection host
  ze_ssh_port / ze.ssh.port   Override connection port

Examples:
  ze status
  ze status --host 10.0.0.1 --port 2222
`)
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
func resolveHost(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	for _, key := range []string{"ze.ssh.host", "ze_ssh_host"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return defaultHost
}

// resolvePort returns the SSH port from flag, env var, or default.
func resolvePort(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	for _, key := range []string{"ze.ssh.port", "ze_ssh_port"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return defaultPort
}
