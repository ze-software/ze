// Design: ai/rules/plugins.md -- ze_bgp dispatch-root CLI gating
//
// BGP CLI command registration for the ze dispatch composition root, gated on
// ze_bgp. This is the SECOND composition root (the first is the generated
// internal/component/plugin/all/all.go, gated via all_ze_bgp.go). Both must
// drop the bgp blank imports when ze_bgp is off, or the packages stay linked
// through this root (the two-composition-root reality, spec-feature-gate-8).
//
// This root carries the CLI registration only, so it also carries ze_core, the
// tag that selects the CLI dispatch personality. The bgp/config blank import
// that fills the always-on infra seams lives in infra_bgp.go under ze_bgp
// alone, because every personality reaches those seams.

//go:build ze_core && ze_bgp

package main

import (
	_ "github.com/ze-software/ze/internal/component/bgp/cli"
)
