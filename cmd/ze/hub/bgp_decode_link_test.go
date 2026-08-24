// Design: ai/rules/plugins.md -- ze_bgp decoder seam link for hub tests
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
// Without ze_bgp there is no BGP CLI package to fill the seam, so
// registry.GetPacketDecoder() answers nil and the withBGPDecode tests assert
// the fall-through to the dispatcher instead. They read the seam itself rather
// than a build-tag mirror of it, and build_tag_bgp_absent_test.go asserts the
// nil in a build that runs.

import (
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/cli"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestBuildTag_BGP_PresentFillsTheDecoderSeam is the positive half of
// build_tag_bgp_absent_test.go's seam assertion: that file proves the seam is
// nil without ze_bgp, and this one proves it is filled with it.
//
// VALIDATES: a ze_bgp build links internal/component/bgp/cli and its init()
// reaches registry.SetPacketDecoder.
// PREVENTS: the decode tests passing vacuously. They read the seam and return
// early when it is nil, which is the honest answer in a BGP-less build and a
// silent pass in this one. Without this test a broken registration would empty
// every one of them and no assertion anywhere would notice.
func TestBuildTag_BGP_PresentFillsTheDecoderSeam(t *testing.T) {
	if pluginreg.GetPacketDecoder() == nil {
		t.Fatal("ze_bgp build: hex-packet decoder seam is nil (bgp/cli init did not reach SetPacketDecoder)")
	}
}
