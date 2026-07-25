// Design: ai/rules/feature-gate-registration.md -- ze_bgp dispatch-root CLI gating
//
// BGP CLI command registration for the ze dispatch composition root, gated on
// ze_bgp. This is the SECOND composition root (the first is the generated
// internal/component/plugin/all/all.go, gated via all_ze_bgp.go). Both must
// drop the bgp blank imports when ze_bgp is off, or the packages stay linked
// through this root (the two-composition-root reality, spec-feature-gate-8).
//
// bgp/config is linked HERE rather than from internal/component/bgp/plugin: its
// init() registers the reactor factory the plugin calls at OnConfigure, plus the
// always-on infra seams (config resolution, peer validation, GR marker). It
// cannot be blank-imported from bgp/plugin because bgp/config's own tests import
// plugin/all, which imports bgp/plugin -- an import cycle in test. Being a
// package main file, this root can never be imported back, so it is the one
// place the edge is safe (spec-feature-gate-10-bgp).

//go:build ze_core && ze_bgp

package main

import (
	_ "github.com/ze-software/ze/internal/component/bgp/cli"
	_ "github.com/ze-software/ze/internal/component/bgp/config"
)
