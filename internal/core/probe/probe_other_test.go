//go:build !linux

package probe

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

// TestProbeCapabilityAbsentOffLinux proves the non-Linux stub reports the DF
// capability absent as a named error, for both families and both DF modes,
// and hands back no conn beside it. A stub that opened the socket with the
// bit silently clear would report a fragment size as a path MTU.
func TestProbeCapabilityAbsentOffLinux(t *testing.T) {
	for _, family := range []Family{FamilyIPv4, FamilyIPv6} {
		for _, df := range []DFMode{DFHonorCache, DFBypassCache} {
			conn, err := OpenICMP(context.Background(), family, netip.Addr{}, df)
			if !errors.Is(err, ErrDFUnsupported) {
				t.Errorf("OpenICMP(%v, %v) err = %v, want ErrDFUnsupported", family, df, err)
			}
			if conn != nil {
				t.Errorf("OpenICMP(%v, %v) returned a conn beside the refusal", family, df)
			}
		}
	}
}

// TestErrorQueueAbsentOffLinux proves the non-Linux drain names the absence
// of a socket error queue and visits nothing: an empty queue and a missing
// capability must never read the same.
func TestErrorQueueAbsentOffLinux(t *testing.T) {
	visited := 0
	err := drainErrorQueue(nil, FamilyIPv4, func(QueuedError) { visited++ })
	if !errors.Is(err, ErrErrQueueUnsupported) {
		t.Errorf("drainErrorQueue err = %v, want ErrErrQueueUnsupported", err)
	}
	if visited != 0 {
		t.Errorf("drainErrorQueue visited %d entries off Linux, want 0", visited)
	}
}

// TestKernelPathMTUAbsentOffLinux proves the non-Linux estimate reports the
// capability absent by name and hands back no value beside it.
func TestKernelPathMTUAbsentOffLinux(t *testing.T) {
	mtu, err := KernelPathMTU(context.Background(), netip.MustParseAddr("192.0.2.1"))
	if !errors.Is(err, ErrPathMTUUnsupported) {
		t.Errorf("KernelPathMTU err = %v, want ErrPathMTUUnsupported", err)
	}
	if mtu != 0 {
		t.Errorf("KernelPathMTU returned %d beside the refusal", mtu)
	}
}
