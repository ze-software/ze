//go:build linux

// Design: docs/research/l2tpv2-ze-integration.md -- exact concrete Go/Linux socket cleanup proof
package kernel

import (
	"context"
	"errors"
	"net"
	"testing"
)

// This real Go socket is closed through its PRIVATE object before carrier
// cleanup sees the resulting error. No raw FD is retained/retried, and a second
// arbitrary opaque net.ErrClosed is not substituted for native ownership proof.
func TestIPv6CarrierReconcilesOnlyItsConcreteClosedLinuxSocket(t *testing.T) {
	var config net.ListenConfig
	packet, err := config.ListenPacket(t.Context(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	owner, ok := packet.(*net.UDPConn)
	if !ok {
		t.Fatalf("actual socket owner=%T", packet)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
	carrier := newIPv6Carrier(ra, owner, "ppp7", 7)
	if err := carrier.Close(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("actual closed Go owner=%v", err)
	}
	retry, ok := any(carrier).(interface{ RetryIPv6CarrierCleanup(context.Context) error })
	if !ok {
		t.Fatal("actual closed Linux socket has no exact-owner reconciliation")
	}
	if err := retry.RetryIPv6CarrierCleanup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if ra.closes != 1 {
		t.Fatal("native reconciliation reclosed the successful RA socket")
	}
	if err := carrier.Close(); !errors.Is(err, net.ErrClosed) {
		t.Fatal("normal Close lost its original exact native error")
	}
}
