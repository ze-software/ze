// Design: ai/rules/feature-gate-registration.md -- ze_exabgp dispatch-root CLI gating
//
// ExaBGP bridge CLI command registration (`ze exabgp ...`) for the ze dispatch
// composition root, gated on ze_exabgp. The other root is the generated
// internal/component/plugin/all/all_ze_exabgp.go (the in-process bridge plugin
// + its schema). Both must drop the exabgp blank imports when ze_exabgp is
// off, or the package stays linked through the surviving root. The always-on
// `ze config migrate` path is untouched: it lives in component/config and
// imports only the internal/exabgp/{topics,migration} library leaves.

//go:build ze_core && ze_exabgp

package main

import (
	_ "github.com/ze-software/ze/internal/plugins/exabgp"
)
