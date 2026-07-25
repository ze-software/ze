// Design: ai/rules/feature-gate-registration.md -- ze_bfd partition of the analysis-tree blank imports
//
//go:build ze_bfd

package cli

// The BFD command package registers its RPC handlers from init() but is not
// reachable through plugin/all from this package (import cycles -- see
// plugin_imports.go rpcDirs), so the analysis tree blank-imports it directly.
// tree.go is always-on, so the import gets a source build tag instead, exactly
// like the BGP partition in tree_bgp.go (spec-feature-gate-12).
import (
	_ "github.com/ze-software/ze/internal/component/bfd/cmd"
)
