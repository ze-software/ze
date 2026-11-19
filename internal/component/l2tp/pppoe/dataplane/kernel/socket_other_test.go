// Design: docs/architecture/l2tp/bng-5-pppoe.md -- native platform refusal
// Related: socket_other.go -- explicit Linux requirement

//go:build !linux

package kernel

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// TestNonLinuxOpenRefuses proves no interface backend or alternate provider is
// consulted when the native PPPoX platform is unavailable.
func TestNonLinuxOpenRefuses(t *testing.T) {
	request := factoryRequest()
	owner := &kernelAccessOwner{name: request.AccessName,
		info:    dataplane.AccessInfo{Device: request.Device, ControlDevice: request.ControlDevice, Namespace: request.Namespace},
		backend: &prepareBackend{}}
	opened, err := owner.Open(t.Context(), request)
	if opened != nil || err == nil {
		t.Fatalf("non-Linux open: session=%v error=%v", opened, err)
	}
	if !strings.Contains(err.Error(), "require Linux") {
		t.Fatalf("platform refusal lost: %v", err)
	}
}
