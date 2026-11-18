// Design: docs/research/l2tpv2-ze-integration.md -- retained original transport custody
// Related: kernel_linux.go -- actual setup callback and partial rollback

//go:build linux

package l2tp

import (
	"errors"
	"net/netip"
	"os"
	"testing"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// TestKernelSetupRetainsExactOriginalTransport drives actual setup with an
// owned pipe and private failure ports. A false diagnostic label cannot replace
// the original file at setup/channel/rollback, including partial-create/panic.
// The pipe is a lifecycle witness, never a counterfeit native PPP grant.
func TestKernelSetupRetainsExactOriginalTransport(t *testing.T) {
	for _, stage := range []string{"partial-create", "setup-error", "setup-panic", "bridge-channel"} {
		t.Run(stage, func(t *testing.T) {
			original, peer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := peer.Close(); err != nil {
					t.Error(err)
				}
			})
			closed := false
			t.Cleanup(func() {
				if !closed {
					if err := original.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			failure := errors.New("original setup boundary")
			calls, closes := 0, 0
			ops := kernelPPPSetupOperations{
				create: func(int, netip.AddrPort, uint16, uint16, uint16, uint16) (*os.File, int, error) {
					if stage == "partial-create" {
						return original, 991, failure
					}
					return original, 991, nil
				},
				setup: func(_ *pppkernel.UnitCreator, actual *os.File, name string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
					calls++
					if actual != original {
						t.Fatalf("setup substituted transport: %p", actual)
					}
					if name != "" {
						t.Fatalf("setup changed name: %q", name)
					}
					if stage == "setup-panic" {
						panic(failure)
					}
					return nil, nil, failure
				},
				channel: func(actual *os.File) (*pppkernel.CreatedChannel, error) {
					calls++
					if actual != original {
						t.Fatalf("bridge attachment substituted transport: %p", actual)
					}
					return nil, failure
				},
				close: func(owners *kernelPPPOwners, actual *os.File, labels [3]int) [3]error {
					closes++
					if actual != original {
						t.Fatalf("rollback substituted transport: %p", actual)
					}
					if labels != [3]int{-1, -1, 991} {
						t.Fatalf("diagnostic labels changed: %v", labels)
					}
					closed = true
					return closeOriginalKernelPPP(owners, actual, (*os.File).Close)
				},
			}
			event := kernelSetupEvent{}
			if stage == "bridge-channel" {
				event.channel = &transferPreparationChannel{}
			}
			owned, err := pppSetupWith(event, nil, ops)
			if !errors.Is(err, failure) {
				t.Fatalf("setup lost cause: %v", err)
			}
			wantCalls := 1
			if stage == "partial-create" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("setup calls=%d want=%d", calls, wantCalls)
			}
			if closes != 1 {
				t.Fatalf("transport closes=%d", closes)
			}
			if owned.transport != nil {
				t.Fatal("failed setup retained active transport custody")
			}
			if owned.retiredTransport != original {
				t.Fatal("failed setup lost original close receipt")
			}
			if _, err := original.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("original remains open: %v", err)
			}
		})
	}
}

// TestKernelPairTransferRetainsOriginalTransport drives the actual worker
// preparation/take pair and confirms only the channel/unit custody transfers.
func TestKernelPairTransferRetainsOriginalTransport(t *testing.T) {
	worker, event, _ := originalTransferModel(t)
	original, peer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(original.Close(), peer.Close()); err != nil {
			t.Error(err)
		}
	})
	event.owner.transport = original
	transfer, current, err := worker.preparePPPTransfer(event, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !current {
		t.Fatal("original receipt was not current")
	}
	if _, ok := worker.takePPPConfig(event, transfer); !ok {
		t.Fatal("original pair did not transfer")
	}
	if event.owner.transport != original {
		t.Fatal("pair transfer lost the worker transport")
	}
	if _, err := original.Stat(); err != nil {
		t.Fatalf("pair transfer closed transport: %v", err)
	}
}
