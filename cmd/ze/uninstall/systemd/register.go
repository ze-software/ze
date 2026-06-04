// Design: docs/architecture/cli/plugin-modes.md — ze uninstall systemd target registration

package systemd

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/subdispatch"
	impl "codeberg.org/thomas-mangin/ze/cmd/ze/systemd"
	"codeberg.org/thomas-mangin/ze/cmd/ze/uninstall"
)

func init() {
	uninstall.Register("systemd", impl.RunUninstall, subdispatch.SubMeta{Desc: "Stop, disable, and remove the systemd service"})
}
