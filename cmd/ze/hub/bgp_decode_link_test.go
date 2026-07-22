// Design: ai/rules/feature-gate-registration.md -- ze_bgp decoder seam link for hub tests
//
//go:build ze_bgp

package hub

// The web tool page decodes "show bgp decode <hex>" in-process through the leaf
// registry seam (registry.SetPacketDecoder), which the gated BGP CLI package
// fills from its own init(). In the shipped binary that package is linked by
// cmd/ze/dispatch_bgp.go; the cmd/ze/hub package on its own never imports it --
// that is the whole point of the seam. This test-only blank import makes the
// hub test binary mirror the shipped binary so the withBGPDecode tests exercise
// a filled seam rather than a nil one.
//
// bgpDecodeLinked lets the same tests assert the opposite in a !ze_bgp build:
// there the seam stays nil and the command must fall through to the dispatcher.

import _ "codeberg.org/thomas-mangin/ze/internal/component/bgp/cli"

// bgpDecodeLinked reports whether this build has a BGP hex-packet decoder.
const bgpDecodeLinked = true
