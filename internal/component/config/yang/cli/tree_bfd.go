// Design: ai/rules/plugins.md -- ze_bfd partition of the analysis-tree blank imports
//
//go:build ze_bfd

package cli

// The BFD command package registers its RPC handlers from init(). plugin/all
// already carries it (all_ze_bfd.go, behind the same ze_bfd tag), which tree.go
// imports, so this blank import duplicates that one; it is kept behind a
// source build tag because tree.go is always-on (spec-feature-gate-12).
import (
	_ "github.com/ze-software/ze/internal/component/bfd/cmd"
)
