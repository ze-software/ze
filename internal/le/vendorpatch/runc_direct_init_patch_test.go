package vendorpatch

import "testing"

// runcDirectInitSeries is the patch series against the vendored
// opencontainers/runc, in set order: one file for each hunk of the container
// runtime's diff (docs/architecture/container/runtime.md, "The diff").
// TestEveryVendorPatchIsApplied cannot notice a patch file that is gone, so
// each name here pins that its patch is present.
var runcDirectInitSeries = []string{
	"runc-01-direct-init.patch",
	"runc-02-start-without-bootstrap.patch",
	"runc-03-refuse-setns.patch",
	"runc-04-cgroup-v2-manager.patch",
	"runc-05-criu-constraint.patch",
	"runc-06-seccomp-seam.patch",
}

// TestRuncDirectInitPatchApplied keeps a re-vendor or a deletion from dropping
// any patch of the runc series. Without the whole series a container either
// does not start (the Go init waits for a C stage a ze binary does not hold)
// or starts with less confinement than the design states, so each file is
// checked by name: it MUST exist and every hunk of it MUST be in vendor/.
func TestRuncDirectInitPatchApplied(t *testing.T) {
	for _, name := range runcDirectInitSeries {
		t.Run(name, func(t *testing.T) {
			assertVendorPatchApplied(t, Dir+"/"+name, vendorPatchRecovery)
		})
	}
}
