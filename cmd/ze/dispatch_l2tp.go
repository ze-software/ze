// Design: ai/rules/plugins.md -- ze_l2tp dispatch-root CLI gating
//
// L2TP CLI command registration (`ze l2tp ...`) for the ze dispatch
// composition root, gated on ze_l2tp. The other roots are the generated
// all_ze_l2tp.go group file and the hub's register_l2tp.go seam filler. All
// must drop their l2tp imports when ze_l2tp is off, or the package stays
// linked through the surviving root.

//go:build ze_core && ze_l2tp

package main

import (
	_ "github.com/ze-software/ze/internal/component/l2tp/cli"
)
