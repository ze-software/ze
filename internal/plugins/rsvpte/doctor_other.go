//go:build !linux

// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- raw-socket probe (non-Linux stub)
// Related: transport_other.go -- RSVP-TE transport is unsupported off Linux

package rsvpte

// rsvpRawSocketAvailable is a stub on non-Linux: RSVP-TE's raw-socket transport
// only exists on Linux, so there is no raw-socket dependency to warn about here.
func rsvpRawSocketAvailable() bool { return true }
