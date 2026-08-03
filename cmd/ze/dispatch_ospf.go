// Design: ai/rules/plugins.md -- ze_ospf dispatch-root CLI gating
//
// OSPF CLI + transport registration for the ze dispatch composition root, gated
// on ze_ospf. This is the SECOND composition root (the first is the generated
// internal/component/plugin/all/all.go, gated via all_ze_ospf.go). Both must
// drop the ospf blank imports when ze_ospf is off, or the package stays linked
// through this root (the two-composition-root reality, spec-feature-gate-8).

//go:build ze_core && ze_ospf

package main

import (
	_ "github.com/ze-software/ze/internal/plugins/ospf/cli"
	_ "github.com/ze-software/ze/internal/plugins/ospf/transport"
)
