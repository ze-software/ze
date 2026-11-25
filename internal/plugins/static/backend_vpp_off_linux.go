// Design: ai/rules/plugins.md -- ze_vpp-off static backend refusal

//go:build linux && !ze_vpp

package static

import "errors"

var errStaticVPPNotBuilt = errors.New("this build was compiled without VPP support (missing ze_vpp build tag)")

// newVPPStaticBackend is the ze_vpp-off counterpart of the GoVPP-backed
// constructor in backend_vpp_linux.go. When the configured data plane is "vpp"
// in a build without ze_vpp, it refuses with doctorCodeVPPChannel and NEVER
// falls back to the Linux kernel backend.
func newVPPStaticBackend() (routeBackend, error) {
	return nil, staticVPPChannelError(errStaticVPPNotBuilt)
}
