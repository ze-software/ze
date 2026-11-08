package sysctl

import "testing"

// TestClearSourceKeepsAnotherSourcesDefault proves that withdrawing one
// source's default hands the key back to a source that still claims it.
//
// VALIDATES: fib-kernel and the interface component both declare
// net.ipv6.conf.all.forwarding=1; the interface component then clears its
// defaults (no unit forwards any more) and the key stays 1 under fib-kernel.
// PREVENTS: the later source stealing the claim, so its withdrawal turns IPv6
// forwarding off under a FIB writer that still needs it.
func TestClearSourceKeepsAnotherSourcesDefault(t *testing.T) {
	s, fb := newTestStore()
	const key = "net.ipv6.conf.all.forwarding"
	fb.values[key] = "0"

	if _, err := s.setDefault(key, "1", "fib-kernel"); err != nil {
		t.Fatalf("fib-kernel default: %v", err)
	}
	if _, err := s.setDefault(key, "1", "interface-ipv6-forwarding"); err != nil {
		t.Fatalf("interface default: %v", err)
	}
	s.clearSourceDefaults("interface-ipv6-forwarding")

	if got := fb.values[key]; got != "1" {
		t.Errorf("kernel %s = %q after the interface component withdrew, want 1 (fib-kernel still claims it)", key, got)
	}
	e := s.entries[key]
	if e.defaultSource != "fib-kernel" {
		t.Errorf("default source = %q, want fib-kernel", e.defaultSource)
	}

	s.clearSourceDefaults("fib-kernel")
	if got := fb.values[key]; got != "0" {
		t.Errorf("kernel %s = %q after every source withdrew, want the original 0", key, got)
	}
}

// TestClearSourceRemovesAShadowedClaim proves that a source clearing its
// default also drops the claim a later source shadowed, so it does not come
// back when the later source withdraws.
//
// VALIDATES: interface claims 1, fib-kernel claims 1, interface clears, then
// fib-kernel clears: the key returns to its original 0.
// PREVENTS: a stale shadowed claim resurrecting a value nobody asks for.
func TestClearSourceRemovesAShadowedClaim(t *testing.T) {
	s, fb := newTestStore()
	const key = "net.ipv6.conf.all.forwarding"
	fb.values[key] = "0"

	if _, err := s.setDefault(key, "1", "interface-ipv6-forwarding"); err != nil {
		t.Fatalf("interface default: %v", err)
	}
	if _, err := s.setDefault(key, "1", "fib-kernel"); err != nil {
		t.Fatalf("fib-kernel default: %v", err)
	}
	s.clearSourceDefaults("interface-ipv6-forwarding")
	if got := fb.values[key]; got != "1" {
		t.Errorf("kernel %s = %q, want 1 while fib-kernel claims it", key, got)
	}
	s.clearSourceDefaults("fib-kernel")
	if got := fb.values[key]; got != "0" {
		t.Errorf("kernel %s = %q after every source withdrew, want the original 0", key, got)
	}
}
