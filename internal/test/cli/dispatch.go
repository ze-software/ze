// Design: docs/architecture/testing/ci-format.md -- ze-test action-first dispatch

package cli

import "codeberg.org/thomas-mangin/ze/internal/core/subdispatch"

var dispatcher = subdispatch.New("test", "Functional test runners, mock servers, and tools")

func Register(name string, handler func([]string) int, meta subdispatch.SubMeta) {
	dispatcher.Register(name, handler, meta)
}

func Dispatch(args []string) int { return dispatcher.Dispatch(args) }
func Subcommands() string        { return dispatcher.Subcommands() }
