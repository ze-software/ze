// Design: docs/architecture/cli/plugin-modes.md — systemd install/uninstall plugin registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package systemd

import (
	"os"

	"github.com/ze-software/ze/cmd/ze/install"
	"github.com/ze-software/ze/cmd/ze/uninstall"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/subdispatch"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	install.Register("systemd", RunInstall, subdispatch.SubMeta{Desc: "Install and enable ze as a systemd service"})
	uninstall.Register("systemd", RunUninstall, subdispatch.SubMeta{Desc: "Stop, disable, and remove the systemd service"})

	// The unit this plugin installs travels with it, and so does the
	// readiness check that reads the unit back (doctor.go). A refusal is a
	// programmer error in the declaration beside it.
	if err := diagnostic.RegisterDoctorCheck(serviceDoctorCheck()); err != nil {
		var tb textbuf.Buffer
		tb.Str("systemd: doctor check registration failed: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // the process is exiting on the next line
		os.Exit(1)
	}
}
