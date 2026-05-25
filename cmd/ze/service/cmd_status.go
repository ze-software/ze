// Design: (none — offline CLI command for systemd service management)

package service

import (
	"errors"
	"flag"
	"io"
	"os/exec"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
)

func (rt *serviceRuntime) cmdStatus(args []string) int {
	fs := newFlagSet("service status", rt.stderr, func() { statusUsageTo(rt.stderr) })
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	if fs.NArg() != 0 {
		writeln(rt.stderr, "error: ze service status takes no positional arguments")
		fs.Usage()
		return exitError
	}

	if !rt.requireSystemd() {
		return exitError
	}
	err := rt.ops.run("systemctl", "status", serviceName)
	if err == nil {
		return exitOK
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	io.WriteString(rt.stderr, "error: "+err.Error()+"\n") //nolint:errcheck // CLI stderr
	return exitError
}

func statusUsageTo(w io.Writer) {
	p := helpfmt.Page{
		Command: "ze service status",
		Summary: "Show systemctl status for ze.service",
		Usage:   []string{"ze service status"},
		Examples: []string{
			"ze service status",
		},
	}
	p.WriteTo(w, false)
}
