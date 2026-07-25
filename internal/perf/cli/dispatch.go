// Design: docs/architecture/system-architecture.md -- ze-perf action-first dispatch

package cli

import "github.com/ze-software/ze/internal/core/subdispatch"

var dispatcher = subdispatch.New("perf", "BGP propagation latency benchmark tool")

func Register(name string, handler func([]string) int, meta subdispatch.SubMeta) {
	dispatcher.Register(name, handler, meta)
}

func Dispatch(args []string) int { return dispatcher.Dispatch(args) }
func Subcommands() string        { return dispatcher.Subcommands() }
