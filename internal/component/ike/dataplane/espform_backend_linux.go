// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Overview: espform_state.go -- the state answered here, and the reporter interface it satisfies
// Related: espform_linux.go -- the dual-form receiver whose fast path this reads
// Related: espform_fastpath_linux.go -- the fast path's attach lifecycle, which fills the state

//go:build linux

package dataplane

// The XFRM backend reports its dual-form receiver's fast path, so the engine's doctor
// check reads it through dataplane.Get() without naming this package's types. The
// MOBIKE backend embeds *xfrmBackend, so it answers the same.
var (
	_ ESPFormFastPathReporter = (*xfrmBackend)(nil)
	_ ESPFormFastPathReporter = (*xfrmMobikeBackend)(nil)
	_ ESPFormFirewallHolder   = (*xfrmBackend)(nil)
	_ ESPFormFirewallHolder   = (*xfrmMobikeBackend)(nil)
)

// HoldESPFormFastPath shuts the gate of the backend's fast path for the length of a
// firewall commit, and answers the release. A backend built without a receiver (a
// bare &xfrmBackend{} literal) has no fast path, so its release does nothing. A fast
// path that is idle still counts the hold, so a start during the commit stays shut.
func (b *xfrmBackend) HoldESPFormFastPath() (release func()) {
	return b.espForms.holdFastPath()
}

// ESPFormFastPathState reads the fast path of the backend's dual-form receiver: its
// status, the cause when it is not working as designed, the links it is attached to and
// refused, and its rewrite counters. A backend built without a receiver (a bare
// &xfrmBackend{} literal) watches no SPI, so its fast path is idle. It answers an error
// when the program's counters cannot be read.
func (b *xfrmBackend) ESPFormFastPathState() (ESPFormFastPathState, error) {
	return b.espForms.fastPathState()
}
