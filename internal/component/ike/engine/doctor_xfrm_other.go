//go:build !linux

// Design: plan/spec-ipsec-dataplane-inspection.md -- kernel XFRM probe (non-Linux)
// Related: doctor_xfrm.go -- the check and its seam

package engine

import (
	"fmt"
	"runtime"
)

// probeXFRM always reports a failure off Linux. XFRM is a Linux dataplane, and
// xfrmBackend (internal/component/ike/dataplane/xfrm_other.go) returns
// ErrNotSupported from every method on this platform, so an IPsec config here
// carries no traffic. Reporting that is the honest answer, not a false pass.
func probeXFRM() error {
	return fmt.Errorf("xfrm is not available on %s", runtime.GOOS)
}
