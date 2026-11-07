//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- shared /dev/ppp setup.
// Related: devppp_linux.go -- PPPIOCNEWUNIT and (planned) rtnetlink kind ppp.

package ppp

import (
	"errors"
	"testing"
)

// TestDevPPPSetupNamesUnit proves that DevPPPSetup honours a configured name
// by creating the kernel device under that exact name. Child-1 Phase 1 wires
// the plumbing so the client passes cfg.Name through to DevPPPSetup. Phase 1
// stubs the named path with ErrNamedUnitNotImplemented, so this test MUST
// stay red until Phase 2 replaces PPPIOCNEWUNIT with an RTM_NEWLINK of kind
// "ppp" carrying IFLA_PPP_DEV_FD (ppp_generic.c ppp_nl_newlink, Linux
// v6.18 drivers/net/ppp/ppp_generic.c:3367). Its red is the signal that
// Phase 2 is owed, per
// plan/release/fixes/spec-home-cpe-1-logical-interface-name.md.
func TestDevPPPSetupNamesUnit(t *testing.T) {
	_, _, _, err := DevPPPSetup(-1, "pppoe0")
	if err == nil {
		t.Fatal("named DevPPPSetup should still refuse: kernel integration is Phase 2")
	}
	if !errors.Is(err, ErrNamedUnitNotImplemented) {
		t.Fatalf("named path returned %v; expected ErrNamedUnitNotImplemented", err)
	}
	// Phase 2 replaces the body above with: a real /dev/ppp open, an
	// rtnetlink LinkAdd of kind "ppp" with IFLA_PPP_DEV_FD on the opened
	// file, PPPIOCGUNIT on the unit fd to read the auto-assigned unit
	// number back, then an assertion that /sys/class/net/pppoe0 exists and
	// a cleanup that deletes it. A-1, A-4 then flip from unvalidated to
	// confirmed.
}
