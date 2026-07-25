// Design: docs/architecture/cli/plugin-modes.md — ze systemd uninstall: remove unit + account

package systemd

import (
	"errors"
	"flag"
	"io"
	"os"

	"github.com/ze-software/ze/internal/core/helpfmt"
)

func (rt *serviceRuntime) cmdUninstall(args []string) int {
	var purge bool

	fs := newFlagSet("systemd uninstall", rt.stderr, func() { uninstallUsageTo(rt.stderr) })
	fs.BoolVar(&purge, "purge", false, "Also remove the ze user and group")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	if fs.NArg() != 0 {
		writeln(rt.stderr, "error: ze systemd uninstall takes no positional arguments")
		fs.Usage()
		return exitError
	}

	if !rt.requireSystemd() || !rt.requireRoot() {
		return exitError
	}
	if _, err := rt.ops.stat(rt.unitPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeln(rt.stderr, "service is not installed")
			if purge {
				return rt.purgeServiceAccount()
			}
			return exitOK
		}
		writef(rt.stderr, "error: checking %s: %v\n", rt.unitPath, err)
		return exitError
	}

	if err := rt.ops.run("systemctl", "stop", serviceName); err != nil {
		writef(rt.stderr, "warning: %v\n", err)
	}
	if err := rt.ops.run("systemctl", "disable", serviceName); err != nil {
		writef(rt.stderr, "error: %v\n", err)
		return exitError
	}
	if err := rt.ops.remove(rt.unitPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		writef(rt.stderr, "error: removing %s: %v\n", rt.unitPath, err)
		return exitError
	}
	if err := rt.ops.run("systemctl", "daemon-reload"); err != nil {
		writef(rt.stderr, "error: %v\n", err)
		return exitError
	}

	writeln(rt.stderr, "service uninstalled")
	if purge {
		return rt.purgeServiceAccount()
	}
	return exitOK
}

func (rt *serviceRuntime) purgeServiceAccount() int {
	code := exitOK
	userExisted := false
	if _, err := rt.ops.output("getent", "passwd", serviceUser); err == nil {
		userExisted = true
		if err := rt.deleteUser(); err != nil {
			writef(rt.stderr, "error: %v\n", err)
			writeln(rt.stderr, "error: skipping group removal (user still exists)")
			return exitError
		}
		writef(rt.stderr, "user %s removed\n", serviceUser)
	}
	if _, err := rt.ops.output("getent", "group", serviceGroup); err == nil {
		if err := rt.deleteGroup(); err != nil {
			writef(rt.stderr, "error: %v\n", err)
			code = exitError
		} else {
			writef(rt.stderr, "group %s removed\n", serviceGroup)
		}
	} else if userExisted {
		writef(rt.stderr, "group %s already removed\n", serviceGroup)
	}
	return code
}

func uninstallUsageTo(w io.Writer) {
	p := helpfmt.Page{
		Command: "ze systemd uninstall",
		Summary: "Stop, disable, and remove ze.service",
		Usage:   []string{"ze systemd uninstall [--purge]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--purge", Desc: "Also remove the ze user and group"},
			}},
		},
		Examples: []string{
			"sudo ze systemd uninstall",
			"sudo ze systemd uninstall --purge",
		},
	}
	p.WriteTo(w, false)
}
