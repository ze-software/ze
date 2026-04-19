// Design: plan/design-rib-unified.md -- Phase 3e (process-wide Loc-RIB)
// Related: manager.go -- the RIB type returned by Default

package locrib

import (
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

var (
	defaultOnce sync.Once
	defaultRIB  *RIB
)

// Default returns the process-wide Loc-RIB shared by in-process plugins
// (BGP, sysrib, OSPF, static, ...). Lazy-initialized on first call; all
// subsequent callers see the same *RIB.
//
// Returns nil in forked plugin subprocesses. The detection reads the
// per-plugin auth token the engine sets only when spawning a subprocess;
// returning nil there avoids installing a private singleton that no other
// plugin could reach, which would silently starve Loc-RIB-backed code paths
// (e.g. sysrib's OnChange subscription).
//
// Callers wire it unconditionally:
//
//	r.SetLocRIB(locrib.Default())   // nil-safe; skips mirroring in forked mode
func Default() *RIB {
	defaultOnce.Do(func() {
		if env.Get("ze.plugin.hub.token") != "" {
			return
		}
		defaultRIB = NewRIB()
	})
	return defaultRIB
}
