// Design: docs/architecture/cli/plugin-modes.md — provision install plugin registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package provision

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/install"
	"codeberg.org/thomas-mangin/ze/internal/core/subdispatch"
)

func init() {
	install.Register("remote", Run, subdispatch.SubMeta{Desc: "Start DHCP+PXE+TFTP provisioning servers for remote device installation"})
}
