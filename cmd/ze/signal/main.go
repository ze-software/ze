// Design: docs/architecture/behavior/signals.md — signal handling CLI
//
// Package signal provides the `ze signal` CLI command for sending OS signals
// to running Ze daemon instances via PID file lookup.
package signal

import (
	"flag"
	"fmt"
	"os"
	"syscall"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	"codeberg.org/thomas-mangin/ze/internal/core/pidfile"
)

// Exit codes for signal command.
const (
	ExitSuccess      = 0
	ExitNotRunning   = 1
	ExitNoPIDFile    = 2
	ExitPermission   = 3
	ExitSignalFailed = 4
)

// signalMap maps command names to OS signals.
var signalMap = map[string]syscall.Signal{
	"reload": syscall.SIGHUP,
	"stop":   syscall.SIGTERM,
	"quit":   syscall.SIGQUIT,
}

// Run executes the ze signal command with the given arguments.
// Returns an exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("signal", flag.ContinueOnError)
	pidFilePath := fs.String("pid-file", "", "Explicit PID file path (overrides config-derived)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze signal <command> [options] <config>

Send signals to a running Ze daemon.

Commands:
  reload    Send SIGHUP - reload configuration
  stop      Send SIGTERM - graceful shutdown
  quit      Send SIGQUIT - goroutine dump + immediate exit

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Arguments:
  <config>  Config file path (used to derive PID file location)

Examples:
  ze signal reload config.conf
  ze signal stop --pid-file /run/ze/daemon.pid config.conf
`)
	}

	if err := fs.Parse(args); err != nil {
		return ExitNotRunning
	}

	remaining := fs.Args()
	if len(remaining) < 2 {
		fmt.Fprintf(os.Stderr, "error: requires <command> and <config>\n")
		fs.Usage()
		return ExitNotRunning
	}

	command := remaining[0]
	configPath := remaining[1]

	// Resolve PID file location
	pidPath, err := resolvePIDFile(*pidFilePath, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return ExitNoPIDFile
	}

	// Read PID file info
	info, err := pidfile.ReadInfo(pidPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return ExitNoPIDFile
	}

	switch command {
	case "reload", "stop", "quit":
		return cmdSignal(command, info)
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
// Returns an exit code.
func RunStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	pidFilePath := fs.String("pid-file", "", "Explicit PID file path (overrides config-derived)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze status [options] <config>

Check if a Ze daemon is running.

Exit codes:
  0  Process is running
  1  Process is not running
  2  PID file not found

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Arguments:
  <config>  Config file path (used to derive PID file location)

Examples:
  ze status config.conf
  ze status --pid-file /run/ze/daemon.pid config.conf
`)
	}

	if err := fs.Parse(args); err != nil {
		return ExitNotRunning
	}

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <config>\n")
		fs.Usage()
		return ExitNotRunning
	}

	configPath := remaining[0]

	pidPath, err := resolvePIDFile(*pidFilePath, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return ExitNoPIDFile
	}

	info, err := pidfile.ReadInfo(pidPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return ExitNoPIDFile
	}

	return cmdStatus(info)
}

// cmdStatus checks if the process is running.
func cmdStatus(info *pidfile.Info) int {
	// Check if process is alive via kill(pid, 0)
	err := syscall.Kill(info.PID, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "process %d is not running\n", info.PID)
		return ExitNotRunning
	}
	fmt.Printf("process %d is running (config: %s, started: %s)\n",
		info.PID, info.ConfigPath, info.StartTime)
	return ExitSuccess
}

// cmdSignal sends an OS signal to the daemon.
func cmdSignal(command string, info *pidfile.Info) int {
	sig, ok := signalMap[command]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		return ExitSignalFailed
	}

	// Check process is alive first
	if err := syscall.Kill(info.PID, 0); err != nil {
		fmt.Fprintf(os.Stderr, "process %d is not running\n", info.PID)
		return ExitNotRunning
	}

	// Send the signal
	if err := syscall.Kill(info.PID, sig); err != nil {
		if os.IsPermission(err) {
			fmt.Fprintf(os.Stderr, "permission denied sending %s to pid %d\n", command, info.PID)
			return ExitPermission
		}
		fmt.Fprintf(os.Stderr, "error sending %s to pid %d: %v\n", command, info.PID, err)
		return ExitSignalFailed
	}

	fmt.Printf("sent %s to pid %d\n", command, info.PID)
	return ExitSuccess
}

// resolvePIDFile determines the PID file path from either explicit flag or config path.
func resolvePIDFile(explicit, configPath string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	return pidfile.Location(configPath)
}
