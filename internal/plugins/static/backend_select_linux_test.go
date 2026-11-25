// VALIDATES: AC-7 (G) on Linux -- newStaticBackendForDataPlane("kernel")
// constructs the Linux kernel route backend.
// PREVENTS: kernel named-table route selection breaking on Linux builds with
// or without ze_vpp.

//go:build linux

package static

import "testing"

func TestStaticKernelDataPlaneConstructsNetlinkBackend(t *testing.T) {
	be, err := newStaticBackendForDataPlane(dataPlaneKernel)
	if err != nil {
		t.Fatalf("newStaticBackendForDataPlane(kernel): %v", err)
	}
	t.Cleanup(func() { _ = be.close() })
	if _, ok := be.(*netlinkStaticBackend); !ok {
		t.Fatalf("backend type = %T, want *netlinkStaticBackend", be)
	}
}
