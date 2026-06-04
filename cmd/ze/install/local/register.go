// Design: docs/architecture/cli/plugin-modes.md — ze install local target registration

package local

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/install"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/subdispatch"
	impl "codeberg.org/thomas-mangin/ze/cmd/ze/local"
)

func init() {
	install.Register("local", impl.RunInstall, subdispatch.SubMeta{Desc: "Copy ze binary and create config directory"})
}
