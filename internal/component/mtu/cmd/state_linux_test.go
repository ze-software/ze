// Design: docs/architecture/diagnostics/path-mtu.md -- the local state read on a live Linux box
// Related: state.go -- readFragmentationCounters, readTCPMTUProbing, readRouteMTUExpires
//
// VALIDATES: the three local reads answer on the running kernel through the
// production paths: the proc mount, and the sysctl component's exported read.
// PREVENTS: a read that works on a fixture and fails on /proc.

//go:build linux

package cmd

import "testing"

// TestLocalStateReadsTheRunningKernel reads the fragmentation counters,
// tcp_mtu_probing and mtu_expires from the box the test runs on. It asserts
// only what every Linux kernel guarantees: the reads succeed and the sysctl
// values are in their documented ranges.
func TestLocalStateReadsTheRunningKernel(t *testing.T) {
	if _, err := readFragmentationCounters(procMountPoint); err != nil {
		t.Errorf("readFragmentationCounters(%s): %v", procMountPoint, err)
	}
	probing, err := readTCPMTUProbing()
	if err != nil {
		t.Fatalf("readTCPMTUProbing: %v", err)
	}
	if probing == tcpMTUProbingUnspecified {
		t.Error("readTCPMTUProbing answered Unspecified with no error")
	}
	expires, err := readRouteMTUExpires()
	if err != nil {
		t.Fatalf("readRouteMTUExpires: %v", err)
	}
	if expires == 0 {
		t.Error("readRouteMTUExpires answered 0: the kernel default is 600")
	}
}
