// Design: docs/architecture/cli/plugin-modes.md — provision install plugin registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package provision

import (
	"github.com/ze-software/ze/cmd/ze/install"
	"github.com/ze-software/ze/internal/core/subdispatch"
)

func init() {
	install.Register("remote", Run, subdispatch.SubMeta{Desc: "Start DHCP+PXE+TFTP provisioning servers for remote device installation"})
}
