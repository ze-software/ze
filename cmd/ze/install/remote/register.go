// Design: docs/architecture/cli/plugin-modes.md — ze install remote target registration

package remote

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/install"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/subdispatch"
	impl "codeberg.org/thomas-mangin/ze/cmd/ze/provision"
)

func init() {
	install.Register("remote", impl.Run, subdispatch.SubMeta{Desc: "Start DHCP+PXE+TFTP provisioning servers for remote device installation"})
}
