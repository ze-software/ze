// Package signal provides the `ze signal` CLI command for sending OS signals
// to running Ze daemon instances via PID file lookup.
package signal

import (
	"flag"
	"fmt"
	"os"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/pidfile"
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
  status    Check if process is running (exit 0 = running, 1 = not)

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Arguments:
  <config>  Config file path (used to derive PID file location)

Examples:
  ze signal status config.conf
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
	case "status":
		return cmdStatus(info)
	case "reload", "stop", "quit":
		return cmdSignal(command, info)
	default:
		fmt.Fprintf(os.Stderr, "unknown signal command: %s\n", command)
		fs.Usage()
		return ExitNotRunning
	}
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
