// Design: ai/rules/feature-gate-registration.md -- ze_isis dispatch-root CLI gating
//
// IS-IS CLI command registration for the ze dispatch composition root, gated on
// ze_isis. This is the SECOND composition root (the first is the generated
// internal/component/plugin/all/all.go, gated via all_ze_isis.go). Both must
// drop the isis blank imports when ze_isis is off, or the package stays linked
// through this root (the two-composition-root reality, spec-feature-gate-8).

//go:build ze_core && ze_isis

package main

import (
	_ "github.com/ze-software/ze/internal/plugins/isis/cli"
)
