// Design: docs/architecture/cli/plugin-modes.md — ze install systemd target registration

package systemd

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/install"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/subdispatch"
	impl "codeberg.org/thomas-mangin/ze/cmd/ze/systemd"
)

func init() {
	install.Register("systemd", impl.RunInstall, subdispatch.SubMeta{Desc: "Install and enable ze as a systemd service"})
}
