// Design: docs/architecture/api/commands.md — clear verb command handlers
//
// Package clear provides the top-level "clear" CLI verb for resetting
// runtime/operational state (counters, ARP entries, session stats)
// without changing configuration. Unlike `del`, which removes config
// objects, `clear` leaves configuration untouched and only zeros
// accumulated kernel or daemon state.
//
// Detail: clear.go — package placeholder; individual clear handlers
// live in their owning plugin's cmd/ directory (e.g. iface/cmd for
// ze-clear:interface-counters).
package clear

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/clear/schema" // init() registers YANG module
)
