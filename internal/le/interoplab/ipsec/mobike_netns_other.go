//go:build !linux

// Design: docs/architecture/testing/qemu-integration.md -- native MOBIKE needs Linux namespaces and XFRM.
package ipsec

import (
	"context"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// RunMOBIKENetns refuses hosts that cannot run the real Linux dataplane proof.
// Use the runtime-kernel QEMU action on other platforms.
func RunMOBIKENetns(_ context.Context, _, _, _, _ string) (Report, int) {
	return Report{SuiteReport: interoplab.SuiteReport{
		SetupError: "native MOBIKE requires Linux; use qemu ipsec-mobike-test with Ze's runtime kernel",
		Code:       1,
	}}, 1
}
