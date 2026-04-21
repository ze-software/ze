// Design: docs/architecture/l2tp.md -- redistribute source registration
// Related: subsystem.go -- Start path calls RegisterL2TPSources
// Related: events/events.go -- typed EventBus handle for route-change

package l2tp

import (
	"log/slog"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
)

var l2tpSourcesOnce sync.Once

// RegisterL2TPSources registers the L2TP redistribute source so
// operators can write `redistribute l2tp` in config. Safe to call
// multiple times (sync.Once). Called from Subsystem.Start.
//
// The single source `l2tp` is exposed. Subscribers identified by
// PPP username and assigned IP are routes with Source=`l2tp`.
func RegisterL2TPSources() {
	l2tpSourcesOnce.Do(func() {
		err := redistribute.RegisterSource(redistribute.RouteSource{
			Name:        "l2tp",
			Protocol:    "l2tp",
			Description: "subscriber routes from L2TP tunnels",
		})
		if err != nil {
			slog.Error("BUG: failed to register l2tp redistribute source", "err", err)
		}
	})
}
