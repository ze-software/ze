// Design: docs/architecture/cli/plugin-modes.md — ze install: action-first dispatch

package install

import "github.com/ze-software/ze/internal/core/subdispatch"

var dispatcher = subdispatch.New("install", "Install ze binary, systemd service, or provision remote devices")

func Register(name string, handler func([]string) int, meta subdispatch.SubMeta) {
	dispatcher.Register(name, handler, meta)
}

func Dispatch(args []string) int { return dispatcher.Dispatch(args) }
func Subcommands() string        { return dispatcher.Subcommands() }
