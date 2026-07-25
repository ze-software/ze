// Design: docs/architecture/cli/plugin-modes.md — ze uninstall: action-first dispatch

package uninstall

import "github.com/ze-software/ze/internal/core/subdispatch"

var dispatcher = subdispatch.New("uninstall", "Remove ze binary or systemd service")

func Register(name string, handler func([]string) int, meta subdispatch.SubMeta) {
	dispatcher.Register(name, handler, meta)
}

func Dispatch(args []string) int { return dispatcher.Dispatch(args) }
func Subcommands() string        { return dispatcher.Subcommands() }
