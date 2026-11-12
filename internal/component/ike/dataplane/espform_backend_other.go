//go:build !linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Overview: espform_state.go -- the state a Linux backend answers, and the reporter interface

package dataplane

import (
	"fmt"
	"runtime"
)

// ESPFormFastPathState refuses: no XFRM exists off Linux, so there is no dual-form
// receiver and no fast path to read.
func (b *xfrmBackend) ESPFormFastPathState() (ESPFormFastPathState, error) {
	return ESPFormFastPathState{}, fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}
