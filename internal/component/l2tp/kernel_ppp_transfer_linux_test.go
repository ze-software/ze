// Design: docs/research/l2tpv2-ze-integration.md -- original worker transfer
// Related: kernel_linux.go -- receipt validation precedes fallible preflight

//go:build linux

package l2tp

import (
	"errors"
	"net/netip"
	"os"
	"testing"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// This is deliberately a private fault model. Empty private resource records
// distinguish original worker custody; they are not valid public PPP owners
// and MUST NOT be submitted to the real NewSession constructor.
func originalTransferModel(t *testing.T) (*kernelWorker, kernelSetupSucceeded, *int) {
	t.Helper()
	session := &L2TPSession{localSID: 2}
	resources := &kernelPPPOwners{}
	owner := &pppSessionFDs{
		session: session, ppp: resources,
		pppoxFD: 7, chanFD: 0, unitFD: 9, unitNum: 11,
	}
	calls := new(int)
	worker := &kernelWorker{
		sessions: map[sessionKey]*pppSessionFDs{{1, 2}: owner},
		ops: kernelOps{
			pppConfig: func(actual *kernelPPPOwners, _ pppkernel.InterfaceOperations) (pppkernel.SessionConfig, error) {
				(*calls)++
				if actual != resources {
					t.Fatal("configuration preflight received another original")
				}
				return pppkernel.SessionConfig{ChanFD: 0, UnitFD: 9, UnitNum: 11}, nil
			},
		},
	}
	return worker, kernelSetupSucceeded{localTID: 1, localSID: 2, owner: owner, session: session, fds: *owner}, calls
}

func TestKernelOriginalTransferRejectsStaleReceiptBeforePreflight(t *testing.T) {
	for _, stale := range []string{"owner", "session", "removed", "bridged", "taken"} {
		t.Run(stale, func(t *testing.T) {
			worker, event, calls := originalTransferModel(t)
			switch stale {
			case "owner":
				replacement := *event.owner
				worker.sessions[sessionKey{1, 2}] = &replacement
			case "session":
				event.session = &L2TPSession{localSID: 2}
			case "removed":
				event.session.removed.Store(true)
			case "bridged":
				event.owner.bridged = true
			case "taken":
				event.owner.ppp = nil
			default:
				t.Fatal("unknown stale-receipt model")
			}
			if _, current, err := worker.preparePPPTransfer(event, nil); current || err != nil {
				t.Fatalf("stale receipt became a failure or transfer: current=%v error=%v", current, err)
			}
			if *calls != 0 {
				t.Fatalf("stale receipt invoked fallible preflight %d times", *calls)
			}
		})
	}
}

func TestKernelOriginalTransferPreflightFailureRetainsCustody(t *testing.T) {
	worker, event, _ := originalTransferModel(t)
	resources := event.owner.ppp
	failure := errors.New("original backend cannot provide creation-bound counters")
	worker.ops.pppConfig = func(actual *kernelPPPOwners, _ pppkernel.InterfaceOperations) (pppkernel.SessionConfig, error) {
		if actual != resources {
			t.Fatal("preflight did not check the original pair")
		}
		return pppkernel.SessionConfig{}, failure
	}
	if _, current, err := worker.preparePPPTransfer(event, nil); !current || !errors.Is(err, failure) {
		t.Fatalf("current preflight result: current=%v error=%v", current, err)
	}
	if event.owner.ppp != resources || event.owner.chanFD != 0 || event.owner.unitFD != 9 {
		t.Fatal("failed preflight transferred or altered original custody")
	}
	if event.session.pppConstruction != nil {
		t.Fatal("worker preflight published an actor construction receipt")
	}
}

func TestKernelOriginalTransferConsumesExactPairOnce(t *testing.T) {
	worker, event, calls := originalTransferModel(t)
	transfer, current, err := worker.preparePPPTransfer(event, nil)
	if err != nil || !current {
		t.Fatalf("original preparation: current=%v error=%v", current, err)
	}
	if event.owner.ppp != transfer.resources || event.owner.chanFD != 0 {
		t.Fatal("non-consuming preparation changed original custody")
	}
	config, taken := worker.takePPPConfig(event, transfer)
	if !taken || config.ChanFD != 0 || config.UnitFD != 9 || config.UnitNum != 11 {
		t.Fatalf("exact configuration was not transferred: taken=%v config=%+v", taken, config)
	}
	if event.owner.ppp != nil || event.owner.chanFD != -1 || event.owner.unitFD != -1 || event.owner.pppoxFD != 7 {
		t.Fatal("transfer did not consume only the pair while retaining transport custody")
	}
	if _, taken := worker.takePPPConfig(event, transfer); taken {
		t.Fatal("original pair transferred twice")
	}
	if *calls != 1 {
		t.Fatalf("configuration preflight calls=%d", *calls)
	}
}

func TestKernelOriginalTransferRechecksPairAfterPreparation(t *testing.T) {
	for _, replacement := range []string{"owner", "resources", "removed"} {
		t.Run(replacement, func(t *testing.T) {
			worker, event, _ := originalTransferModel(t)
			transfer, current, err := worker.preparePPPTransfer(event, nil)
			if err != nil || !current {
				t.Fatalf("original preparation: current=%v error=%v", current, err)
			}
			switch replacement {
			case "owner":
				next := *event.owner
				worker.sessions[sessionKey{1, 2}] = &next
			case "resources":
				event.owner.ppp = &kernelPPPOwners{}
			case "removed":
				event.session.removed.Store(true)
			default:
				t.Fatal("unknown replacement model")
			}
			if _, taken := worker.takePPPConfig(event, transfer); taken {
				t.Fatal("prepared receipt transferred a changed original")
			}
			if event.owner.chanFD != 0 || event.owner.unitFD != 9 {
				t.Fatal("rejected transfer consumed diagnostic labels")
			}
		})
	}
}

func TestKernelOriginalRolePreflightRequiresCreatorOnlyForLocalUnit(t *testing.T) {
	if plan, err := prepareKernelPPPSetup(kernelSetupEvent{}, nil); err == nil || plan != nil {
		t.Fatalf("local unit accepted absent creation capability: plan-present=%v error=%v", plan != nil, err)
	}
	// The existing channel interface is a borrowed transport supplied by the
	// accepted relay. Merely preparing this role must not invoke its methods.
	ev := kernelSetupEvent{channel: &transferPreparationChannel{}}
	if plan, err := prepareKernelPPPSetup(ev, nil); err != nil || plan == nil {
		t.Fatalf("bridge role required an unused local creator: plan-present=%v error=%v", plan != nil, err)
	}
	// Do not execute this plan: native channel creation is a separate Linux
	// acceptance question requiring actual kernel-created original owners.
}

func TestKernelOriginalCreatorRefusalPrecedesSubscriberAllocation(t *testing.T) {
	session := &L2TPSession{localSID: 2}
	failures := make(chan kernelSetupFailed, 1)
	allocations := 0
	worker := newKernelWorker(kernelOps{
		pppSetup: func(event kernelSetupEvent) (kernelPPPSetup, error) {
			return prepareKernelPPPSetup(event, nil)
		},
		tunnelCreate: func(uint16, uint16, int, netip.AddrPort) (int, error) {
			allocations++
			return -1, errors.New("must not allocate a tunnel")
		},
		sessionCreate: func(sessionCreateParams) error {
			allocations++
			return errors.New("must not allocate a kernel session")
		},
	}, failures, nil, discardLoggerForTest())
	worker.setupSession(kernelSetupEvent{localTID: 1, localSID: 2, session: session})
	if allocations != 0 || len(worker.tunnels) != 0 || len(worker.sessions) != 0 || session.nativeOwner.Load() != nil {
		t.Fatal("creation capability refusal mutated original native custody or allocated subscriber resources")
	}
	select {
	case failure := <-failures:
		if failure.session != session || failure.err == nil {
			t.Fatal("capability refusal lost its original failure receipt")
		}
	default:
		t.Fatal("capability refusal was not reported")
	}
}

type transferPreparationChannel struct{}

func (*transferPreparationChannel) WithDescriptor(func(int) error) error {
	panic("role preflight must not borrow an attached descriptor")
}

func TestKernelOriginalSetupPartialTransportZeroRetainsCleanupCause(t *testing.T) {
	creation := errors.New("transport returned a partial allocation")
	cleanup := errors.New("transport close failed")
	closes := 0
	owned, err := pppSetupWith(kernelSetupEvent{}, nil, kernelPPPSetupOperations{
		create: func(int, netip.AddrPort, uint16, uint16, uint16, uint16) (*os.File, int, error) {
			return nil, 0, creation
		},
		close: func(owners *kernelPPPOwners, original *os.File, labels [3]int) [3]error {
			if original != nil {
				t.Fatal("logical fixture fabricated a native file")
			}
			closes++
			if owners != nil || labels != [3]int{-1, -1, 0} {
				t.Fatalf("partial transport cleanup lost its exact original labels: owners=%v labels=%v", owners, labels)
			}
			return [3]error{nil, nil, cleanup}
		},
	})
	if !errors.Is(err, creation) || !errors.Is(err, cleanup) {
		t.Fatalf("partial allocation lost its creation/cleanup cause: %v", err)
	}
	if closes != 1 || owned.pppoxFD != -1 {
		t.Fatalf("partial transport was not consumed once: closes=%d fd=%d", closes, owned.pppoxFD)
	}
}

func TestKernelOriginalSetupPanicReleasesAlreadyAllocatedTransport(t *testing.T) {
	failure := errors.New("PPP attachment panicked")
	closes := 0
	owned, err := pppSetupWith(kernelSetupEvent{}, nil, kernelPPPSetupOperations{
		create: func(int, netip.AddrPort, uint16, uint16, uint16, uint16) (*os.File, int, error) {
			return nil, 0, nil
		},
		setup: func(*pppkernel.UnitCreator, *os.File, string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
			panic(failure)
		},
		close: func(owners *kernelPPPOwners, original *os.File, labels [3]int) [3]error {
			if original != nil {
				t.Fatal("logical fixture fabricated a native file")
			}
			closes++
			if owners != nil || labels != [3]int{-1, -1, 0} {
				t.Fatalf("panic cleanup lost the allocated transport: owners=%v labels=%v", owners, labels)
			}
			return [3]error{}
		},
	})
	if !errors.Is(err, failure) || closes != 1 || owned.pppoxFD != -1 {
		t.Fatalf("panic did not release the original transport once: closes=%d fd=%d error=%v", closes, owned.pppoxFD, err)
	}
}

func TestKernelOriginalCleanupPanicRetainsCause(t *testing.T) {
	failure := errors.New("original close panic")
	if err := closeKernelPPPOperation(func() error { panic(failure) }); !errors.Is(err, failure) {
		t.Fatalf("close panic lost original cause: %v", err)
	}
}
