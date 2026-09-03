// Design: ai/rules/plugins.md -- ze_bgp infra-seam linking from the dispatch root
//
// Links internal/component/bgp/config so its init() fills the always-on seams,
// gated on ze_bgp alone. The seams are the reactor factory
// (registry.RegisterReactorFactory), the inline plugin extractor
// (zeconfig.RegisterPluginExtractor) and four in internal/component/config/infra:
// config resolution (SetBGPTreeResolver), peer validation
// (SetBGPPeerValidator), roleless-peer reporting
// (SetBGPRolelessPeerReporter) and the RFC 4724 GR marker writer
// (SetGRMarkerWriter). With ze_bgp off all six stay nil and every caller takes
// its no-BGP branch.
//
// bgp/config is linked from a package main file rather than from
// internal/component/bgp/plugin because bgp/config's own tests import
// plugin/all, which imports bgp/plugin -- an import cycle in test. A package
// main root can never be imported back, so it is the one place the edge is safe
// (spec-feature-gate-10-bgp).
//
// The gate is ze_bgp and NOT ze_core && ze_bgp, which is what dispatch_bgp.go
// carries. ze_core selects the CLI dispatch personality, so it is the right
// gate for a CLI command registration and the wrong gate for a seam every
// personality reaches. The ze-test harness binary is built with ze_test plus
// the feature tags and no ze_core (internal/le/functional/binaries.go,
// buildCommands), and it drives the config editor in-process: under the wider
// gate it held the BGP filter plugins that declare a peer obligation and not
// the package that enforces one, so infra.ValidateBGPPeers answered nil and
// test/editor/lifecycle/commit-blocked-missing-leak-filter.et could never pass.

//go:build ze_bgp

package main

import (
	_ "github.com/ze-software/ze/internal/component/bgp/config"
)
