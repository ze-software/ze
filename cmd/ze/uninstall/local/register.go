// Design: docs/architecture/cli/plugin-modes.md — ze uninstall local target registration

package local

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/subdispatch"
	impl "codeberg.org/thomas-mangin/ze/cmd/ze/local"
	"codeberg.org/thomas-mangin/ze/cmd/ze/uninstall"
)

func init() {
	uninstall.Register("local", impl.RunUninstall, subdispatch.SubMeta{Desc: "Remove ze binary and optionally config directory"})
}
