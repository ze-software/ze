// Design: docs/research/l2tpv2-ze-integration.md -- acknowledged NCP activation
// Related: ncp.go -- reconcileOpenedNCP

package ppp

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

// Normal closure must also release the initial NCP convergence wait; otherwise
// its timer would later end PPP despite the independent family's Opened state.
func TestNCPInitialCompletionPreservesClosedFamily(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			fixture := newNCPFamilyLifecycle(t)
			if fixture.session.handleFrame(lcpFrame(ncpProto(family), LCPTerminateRequest, 31, nil)) {
				t.Fatal("family termination ended PPP")
			}
			if fixture.session.ncpsComplete(true, true) {
				t.Fatal("unsettled termination permitted initial activation")
			}
			if fixture.session.handleFrame(lcpFrame(ncpProto(family), LCPTerminateAck, 31, nil)) {
				t.Fatal("settled family termination ended PPP")
			}
			if !fixture.session.ncpsComplete(true, true) {
				t.Fatal("settled family closure still blocks initial NCP completion")
			}
			fixture.assertNoSessionDown(t)
		})
	}
}

// A reopened family uses the same assignment barrier as the initial lifetime.
// The probe observes actual Connect calls, including IPv6 service startup.
func TestNCPReopenReconcilesAfterAssignmentAcknowledgment(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			fixture := newNCPFamilyLifecycle(t)
			var publication atomic.Pointer[EventSessionIPAssigned]
			var early atomic.Bool
			var connects atomic.Int32
			fixture.session.dataplane = &ncpReconcileProbe{
				SessionHandle: fixture.session.dataplane,
				connect: func() error {
					assignment := publication.Load()
					if assignment == nil {
						early.Store(true)
					} else {
						select {
						case <-assignment.done:
						default:
							early.Store(true)
						}
					}
					connects.Add(1)
					return nil
				},
			}
			if fixture.session.handleFrame(fixture.configureRequest(family)) {
				t.Fatal("renegotiation ended PPP")
			}
			request := fixture.lastConfigureRequest(t, family)
			finished := make(chan bool, 1)
			fixture.workers.Go(func() {
				finished <- fixture.session.handleFrame(lcpFrame(ncpProto(family), LCPConfigureAck,
					request.Identifier, request.Data))
			})
			var event Event
			select {
			case event = <-fixture.events:
			case <-finished:
				t.Fatal("reopen returned before publishing an assignment")
			case <-time.After(time.Second):
				t.Fatal("reopen did not publish its assignment")
			}
			assignment, ok := event.(EventSessionIPAssigned)
			if !ok {
				t.Fatalf("reopen event = %T, want assignment", event)
			}
			if assignment.Family != family {
				t.Fatalf("assignment family = %s, want %s", assignment.Family, family)
			}
			publication.Store(&assignment)
			if family == AddressFamilyIPv4 && fixture.session.installedIPv4 != fixture.localPrefix(family) {
				t.Fatal("IPv4 assignment was published before retaining its installed resource")
			}
			if connects.Load() != 0 {
				t.Fatal("reopened family reconciled before its assignment acknowledgment")
			}
			assignment.Acknowledge()
			select {
			case terminated := <-finished:
				if terminated {
					t.Fatal("acknowledged reopen ended PPP")
				}
			case <-time.After(time.Second):
				t.Fatal("reopen did not consume its assignment acknowledgment")
			}
			if early.Load() || connects.Load() != 1 {
				t.Fatalf("reconciliation early = %t, calls = %d; want one post-ACK call", early.Load(), connects.Load())
			}
		})
	}
}

// The initial channel is connected before the IPv6 pair exists. Startup must
// reconcile that pair before opening any carrier that can send network data.
func TestIPv6ServiceReconcilesBeforeCarrier(t *testing.T) {
	owner := newTestSessionHandle(&fakeBackend{}, "created-ppp")
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Errorf("session cleanup: %v", err)
		}
	})
	var operations []string
	owner.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) {
		operations = append(operations, "carrier")
		return newServiceCarrier(), nil
	}
	probe := &ncpReconcileProbe{
		SessionHandle: owner,
		addAddress: func(local, peer netip.Prefix) error {
			operations = append(operations, "address")
			return owner.AddAddressP2P(local, peer)
		},
		connect: func() error { operations = append(operations, "connect"); return nil },
	}
	cfg := serviceHandleConfig(owner)
	cfg.Dataplane = probe
	service, err := startIPv6Service(cfg, testServerID(), nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := service.Stop(); err != nil {
			t.Errorf("service cleanup: %v", err)
		}
	}()
	if len(operations) != 3 || operations[0] != "address" || operations[1] != "connect" || operations[2] != "carrier" {
		t.Fatalf("startup operations = %v, want address, connect, carrier", operations)
	}
}

// A failed reconciliation owns the same rollback obligation as a failed
// carrier acquisition, including retaining the owner if withdrawal fails.
func TestIPv6ServiceReconcileFailureRetainsCleanupOwner(t *testing.T) {
	backend := &fakeBackend{removeAddrErr: errors.New("withdrawal refused")}
	owner := newTestSessionHandle(backend, "created-ppp")
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Errorf("session cleanup: %v", err)
		}
	})
	opened := false
	owner.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) {
		opened = true
		return newServiceCarrier(), nil
	}
	failure := errors.New("family reconciliation refused")
	cfg := serviceHandleConfig(owner)
	cfg.Dataplane = &ncpReconcileProbe{SessionHandle: owner, connect: func() error { return failure }}
	service, err := startIPv6Service(cfg, testServerID(), nil, slog.New(slog.DiscardHandler))
	if service != nil {
		t.Cleanup(func() {
			backend.removeAddrErr = nil
			if err := service.Stop(); err != nil {
				t.Errorf("retained service cleanup: %v", err)
			}
		})
	}
	if !errors.Is(err, failure) || !errors.Is(err, backend.removeAddrErr) {
		t.Fatalf("startup failure = %v, want reconciliation and withdrawal causes", err)
	}
	if opened || service == nil || !service.linkLocalInstalled {
		t.Fatalf("carrier opened = %t, retained service = %+v", opened, service)
	}
	backend.removeAddrErr = nil
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if service.linkLocalInstalled {
		t.Fatal("successful cleanup retained the installation witness")
	}
}

type ncpReconcileProbe struct {
	SessionHandle
	connect    func() error
	addAddress func(netip.Prefix, netip.Prefix) error
}

func (probe *ncpReconcileProbe) Connect() error { return probe.connect() }

func (probe *ncpReconcileProbe) AddAddressP2P(local, peer netip.Prefix) error {
	if probe.addAddress != nil {
		return probe.addAddress(local, peer)
	}
	return probe.SessionHandle.AddAddressP2P(local, peer)
}
