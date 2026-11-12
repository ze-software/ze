// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- both ESP forms on one inbound state: the complete path re-presents on a policy-checked device
// Related: espform_tun_probe_linux.go -- the probe on Linux

//go:build !linux

package dataplane

import (
	"fmt"
	"runtime"
)

// ESPFormTUNAvailable refuses off Linux: no XFRM exists there, so there is no dual-form
// receiver and no TUN device for it to open. The ze doctor check of the dual-form
// receiver calls it; ErrNotSupported tells it the question does not apply here.
func ESPFormTUNAvailable() error {
	return fmt.Errorf("%w: no TUN device for the ESP form receiver on %s", ErrNotSupported, runtime.GOOS)
}
