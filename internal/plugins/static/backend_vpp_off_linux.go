// Design: ai/rules/feature-gate-registration.md -- ze_vpp-off static backend stub

//go:build linux && !ze_vpp

package static

// newVPPStaticBackend is the ze_vpp-off counterpart of the GoVPP-backed
// constructor in backend_vpp_linux.go. Returning nil is the existing "fall
// back to kernel" signal newStaticBackend already handles for a missing VPP
// connector, so a vpp-less build routes static routes to the kernel FIB
// exactly like a vpp-enabled build without an active VPP connection.
func newVPPStaticBackend() routeBackend { return nil }
