// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: espform_fastpath_probe_linux.go -- the probe on Linux

//go:build !linux

package dataplane

import (
	"fmt"
	"runtime"
)

// ESPFormFastPathReady refuses off Linux: no XFRM exists there, so there is no
// dual-form receiver and no fast path to load. ErrNotSupported tells the engine's
// readiness record the question does not apply here.
func ESPFormFastPathReady() (ESPFormFastPathFault, error) {
	return ESPFormFastPathFaultUnspecified, fmt.Errorf("%w: no ESP form fast path on %s", ErrNotSupported, runtime.GOOS)
}
