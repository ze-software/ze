// Design: docs/architecture/cli/plugin-modes.md — local install/uninstall plugin registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package local

import (
	"github.com/ze-software/ze/cmd/ze/install"
	"github.com/ze-software/ze/cmd/ze/uninstall"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/subdispatch"
)

func init() {
	install.RegisterDefault(RunNativeInstall, subdispatch.SubMeta{Desc: "Install and initialize this Linux host", Options: nativeInstallOptions})
	uninstall.RegisterDefault(RunNativeUninstall, subdispatch.SubMeta{Desc: "Remove this installation and preserve state", Options: nativeUninstallOptions})
	install.Register("local", RunInstall, subdispatch.SubMeta{Desc: "Copy ze binary and create config directory"})
	uninstall.Register("local", RunUninstall, subdispatch.SubMeta{Desc: "Remove ze binary and optionally config directory"})

	// Flag inventory for shell completion (registration over hardcoding).
	// Mirrors the flag.FlagSet declarations in cmd_install.go.
	registry.RegisterCommandFlags("install local", []registry.FlagSpec{
		{Name: "--prefix", Description: "installation prefix (default: interactive selection)", ValueHint: registry.FlagValueFile},
		{Name: flagDryRun, Description: "print what would be done without making changes", ValueHint: registry.FlagValueNone},
	})
	registry.RegisterCommandFlags("install", []registry.FlagSpec{
		{Name: "--prefix", Description: "installation prefix (default /usr/local)", ValueHint: registry.FlagValueFile},
		{Name: "--no-service", Description: "install for foreground operation", ValueHint: registry.FlagValueNone},
		{Name: flagDryRun, Description: "show paths without changes", ValueHint: registry.FlagValueNone},
	})
	registry.RegisterCommandFlags("uninstall", []registry.FlagSpec{
		{Name: "--prefix", Description: "installation prefix (default /usr/local)", ValueHint: registry.FlagValueFile},
		{Name: "--no-service", Description: "remove a foreground installation", ValueHint: registry.FlagValueNone},
		{Name: "--yes", Description: "confirm removal", ValueHint: registry.FlagValueNone},
		{Name: "--purge", Description: "also remove selected state", ValueHint: registry.FlagValueNone},
		{Name: flagDryRun, Description: "show paths without changes", ValueHint: registry.FlagValueNone},
	})
}
