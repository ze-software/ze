// Design: docs/architecture/diagnostics/debug-filtering.md -- BGP debug flag registration
// Related: register.go -- config YANG registration in same package

package yang

import (
	debugyang "github.com/ze-software/ze/internal/component/debug/yang"
)

func init() {
	debugyang.RegisterModule(debugyang.Module{
		Name:   "ze-bgp-debug",
		Prefix: "bgp",
		Flags: []string{
			"open", "update", "keepalive", "notification", "refresh",
			"route", "policy", "fsm", "timer", "socket", "config",
			"graceful-restart", "bfd", "capability",
		},
		Scopes: []string{"neighbor", "group", "direction"},
	})
}
