// Design: ai/rules/feature-gate-registration.md -- ze_bgp partition of the analysis-tree blank imports
//
//go:build ze_bgp

package cli

// The BGP command packages register their RPC handlers from init() but are not
// reachable through plugin/all (import cycles -- see plugin_imports.go rpcDirs),
// so the analysis tree blank-imports them directly. tree.go is always-on, and
// this package is imported at the CLI dispatch root (cmd/ze/ze_core_dispatch.go)
// rather than through the generated composition root, so these five imports are
// the one place in the always-on tree that cannot be gated by blank-import
// partitioning. They get a source build tag instead -- one of the three
// non-test files budgeted for it in spec-feature-gate-10-bgp.
import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/handler"
)
