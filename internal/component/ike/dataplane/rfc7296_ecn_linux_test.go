// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend

//go:build linux

package dataplane

import (
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
)

// VALIDATES: the netlink state ze fills for the kernel carries no general flags field, so
// XFRM_STATE_NOECN cannot be set from the XFRM backend even by mistake.
//
// PREVENTS: a netlink bump that adds a flags field, after which the backend could disable
// the kernel's ECN propagation without any other code change being visible.
//
// This is the linux-only companion of TestEcnNoConfigPathDisablesECN. It carries no RFC
// tag on purpose: the vendored bindings collapse XfrmState to struct{} off Linux, so this
// check does not run in the merge gate and must not be counted as evidence that does.
func TestEcnXfrmStateHasNoFlagsField(t *testing.T) {
	for _, name := range ecnFieldNames(t, netlink.XfrmState{}) {
		if name == "flags" || strings.Contains(name, "ecn") {
			t.Errorf("netlink.XfrmState now exposes %q, so XFRM_STATE_NOECN became reachable; "+
				"re-check that InstallSA never sets it", name)
		}
	}
}
