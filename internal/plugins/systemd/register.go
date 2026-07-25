// Design: docs/architecture/cli/plugin-modes.md — systemd install/uninstall plugin registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package systemd

import (
	"github.com/ze-software/ze/cmd/ze/install"
	"github.com/ze-software/ze/cmd/ze/uninstall"
	"github.com/ze-software/ze/internal/core/subdispatch"
)

func init() {
	install.Register("systemd", RunInstall, subdispatch.SubMeta{Desc: "Install and enable ze as a systemd service"})
	uninstall.Register("systemd", RunUninstall, subdispatch.SubMeta{Desc: "Stop, disable, and remove the systemd service"})
}
