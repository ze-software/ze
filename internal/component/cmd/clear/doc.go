// Design: docs/architecture/api/commands.md -- clear verb (generic verb shell)
//
// Package clear is the generic top-level "clear" CLI verb for resetting
// runtime/operational state without changing configuration. It owns only the
// generic clear verb tree schema; every owner-specific clear handler lives in
// its owning component's cmd/ package:
//
//   - ze-clear:dns-cache         -> internal/component/resolve/cmd
//   - ze-clear:vpn-ipsec-sa      -> internal/component/ike/cmd
//   - ze-clear:interface-counters -> internal/component/iface/cmd
//
// Removing an owner removes its clear subcommand; the generic verb shell here
// keeps working.
package clear

import (
	_ "github.com/ze-software/ze/internal/component/cmd/clear/yang" // init() registers the clear YANG verb tree
)
