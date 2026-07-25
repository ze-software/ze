// Design: docs/architecture/system-architecture.md -- ze-analyze action-first dispatch

package analyze

import "github.com/ze-software/ze/internal/core/subdispatch"

var dispatcher = subdispatch.New("analyze", "BGP MRT analysis tools")

func Register(name string, handler func([]string) int, meta subdispatch.SubMeta) {
	dispatcher.Register(name, handler, meta)
}

func Dispatch(args []string) int { return dispatcher.Dispatch(args) }
func Subcommands() string        { return dispatcher.Subcommands() }
