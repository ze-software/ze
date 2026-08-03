// Design: ai/rules/plugins.md -- ze_bgp-absent decoder seam state for hub tests
//
//go:build !ze_bgp

package hub

// Without ze_bgp there is no BGP CLI package to fill the registry's hex-packet
// decoder seam, so withBGPDecode has nothing to intercept with and "show bgp
// decode" must fall through to the dispatcher like any other unknown command.
// The withBGPDecode tests read this to assert that behavior instead of the
// decode behavior.

// bgpDecodeLinked reports whether this build has a BGP hex-packet decoder.
const bgpDecodeLinked = false
