// Design: docs/architecture/l2tp/bng-5-pppoe.md -- native socket allocation contract
// Related: socket_linux.go -- injected syscalls; no syscall runs in these tests

//go:build linux

package kernel

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

// TestNativeSocketBindsResolvedDevice proves the exact device, SID and MAC reach
// connect, with atomic close-on-exec and nonblocking descriptor allocation.
func TestNativeSocketBindsResolvedDevice(t *testing.T) {
	closes := 0
	request := factoryRequest()
	owner, err := openTransportWithOps(context.Background(), request, socketOps{
		socket: func(domain, kind, protocol int) (int, error) {
			if domain != unix.AF_PPPOX || kind != unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK || protocol != 0 {
				t.Fatalf("socket arguments: %d %d %d", domain, kind, protocol)
			}
			return 41, nil
		},
		connect: func(fd int, address unix.Sockaddr) error {
			peer, ok := address.(*unix.SockaddrPPPoE)
			if !ok {
				t.Fatalf("unexpected address type: %T", address)
			}
			if fd != 41 || peer.Dev != request.Device || peer.SID != request.SessionID || !bytes.Equal(peer.Remote, request.PeerMAC[:]) {
				t.Fatalf("binding fd=%d address=%+v", fd, peer)
			}
			return nil
		},
		close: func(int) error { closes++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if closes != 0 {
		t.Fatal("successful transport already closed")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if closes != 1 {
		t.Fatalf("owner closed descriptor %d times", closes)
	}
}

// TestNativeSocketFailuresPreserveCleanup proves cancellation and native connect
// failures close the allocated socket and retain both operation/cleanup errors.
func TestNativeSocketFailuresPreserveCleanup(t *testing.T) {
	for _, stage := range []string{"socket", "connect", "cancel-socket", "cancel-connect"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			closes := 0
			failure := errors.New("native operation")
			cleanup := errors.New("native close")
			owner, err := openTransportWithOps(ctx, factoryRequest(), socketOps{
				socket: func(int, int, int) (int, error) {
					if stage == "socket" {
						return -1, failure
					}
					if stage == "cancel-socket" {
						cancel()
					}
					return 41, nil
				},
				connect: func(int, unix.Sockaddr) error {
					if stage == "connect" {
						return failure
					}
					if stage == "cancel-connect" {
						cancel()
					}
					return nil
				},
				close: func(int) error { closes++; return cleanup },
			})
			if owner != nil || err == nil {
				t.Fatalf("failed native open: owner=%v error=%v", owner, err)
			}
			if stage == "socket" {
				if !errors.Is(err, failure) || closes != 0 {
					t.Fatalf("socket failure: closes=%d error=%v", closes, err)
				}
				return
			}
			if closes != 1 || !errors.Is(err, cleanup) {
				t.Fatalf("cleanup: closes=%d error=%v", closes, err)
			}
			if stage == "connect" {
				if !errors.Is(err, failure) {
					t.Fatalf("connect failure lost: %v", err)
				}
				return
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
		})
	}
}
