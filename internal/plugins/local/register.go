// Design: docs/architecture/cli/plugin-modes.md — local install/uninstall plugin registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package local

import (
	"github.com/ze-software/ze/cmd/ze/install"
	"github.com/ze-software/ze/cmd/ze/uninstall"
	"github.com/ze-software/ze/internal/core/subdispatch"
)

func init() {
	install.Register("local", RunInstall, subdispatch.SubMeta{Desc: "Copy ze binary and create config directory"})
	uninstall.Register("local", RunUninstall, subdispatch.SubMeta{Desc: "Remove ze binary and optionally config directory"})
}
