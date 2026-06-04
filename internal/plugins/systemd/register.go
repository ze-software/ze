// Design: docs/architecture/cli/plugin-modes.md — systemd install/uninstall plugin registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package systemd

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/install"
	"codeberg.org/thomas-mangin/ze/cmd/ze/uninstall"
	"codeberg.org/thomas-mangin/ze/internal/core/subdispatch"
)

func init() {
	install.Register("systemd", RunInstall, subdispatch.SubMeta{Desc: "Install and enable ze as a systemd service"})
	uninstall.Register("systemd", RunUninstall, subdispatch.SubMeta{Desc: "Stop, disable, and remove the systemd service"})
}
