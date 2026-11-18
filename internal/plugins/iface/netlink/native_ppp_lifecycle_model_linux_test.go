// Design: docs/features/interfaces.md -- original PPP write refusal and custody models
// These are non-native models. No fake descriptor or feature mask is sent to Linux.

//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

type nativePPPModelRefuser struct {
	calls int
	cause error
}

func (owner *nativePPPModelRefuser) WithDescriptor(func(int) error) error {
	owner.calls++
	return owner.cause
}

// TestNativePPPModelFeatureGuardPrecedesBorrow checks the real method's guard
// and a control that reaches the refusal owner. The model never grants a lease.
func TestNativePPPModelFeatureGuardPrecedesBorrow(t *testing.T) {
	refusal := errors.New("model original file refuses its descriptor")
	owner := &nativePPPModelRefuser{cause: refusal}
	writer := &nativePPPWriter{parent: &nativePPPResources{}, owner: owner,
		index: 37, features: nativePPPAddr4}
	local := netip.MustParsePrefix("198.51.100.1/24")
	peer := netip.MustParsePrefix("198.51.100.2/24")
	if err := writer.RemoveAddressP2P(context.Background(), local, peer); err == nil {
		t.Fatal("family bit alone authorized exact pair deletion")
	}
	if owner.calls != 0 {
		t.Fatal("missing EXACT_ADDR_PAIR borrowed an original descriptor")
	}
	writer.features |= nativePPPExactAddressPair
	if err := writer.RemoveAddressP2P(context.Background(), local, peer); !errors.Is(err, refusal) {
		t.Fatalf("complete feature control did not reach original owner refusal: %v", err)
	}
	if owner.calls != 1 {
		t.Fatalf("complete feature control borrowed %d times, want once", owner.calls)
	}
	if writer.pending != nil {
		t.Fatal("unsubmitted owner refusal retained a native transaction")
	}
}

// TestNativePPPModelUncertainCustody preserves independent original send,
// native errno, wait and parser causes. Mutation/settlement/Close never asks for
// another file, and backend Close does not destroy its other original readers.
func TestNativePPPModelUncertainCustody(t *testing.T) {
	owner := &nativePPPModelRefuser{cause: errors.New("must never acquire another descriptor")}
	backend := &netlinkBackend{}
	writer := &nativePPPWriter{parent: &backend.nativePPPs, owner: owner,
		index: 37, features: nativePPPSetlink}
	sendCause := errors.New("model uncertain original send")
	waitCause := errors.New("model unknown original ACK read")
	parseCause := errors.New("model invalid original ACK trailing message")
	writer.pending = &nativePPPTransaction{writer: writer, entered: true,
		cause: sendCause, nativeErr: unix.ESRCH, waitErrs: []error{waitCause}, poison: parseCause}
	backend.nativePPPs.writers = map[*nativePPPWriter]struct{}{writer: {}}
	for _, operation := range []struct {
		name string
		call func() error
	}{
		{"mutation", func() error { return writer.SetMTU(context.Background(), 1431) }},
		{"settlement", func() error { return writer.Settle(context.Background()) }},
		{"writer close", writer.Close},
		{"backend close", backend.Close},
	} {
		t.Run(operation.name, func(t *testing.T) {
			err := operation.call()
			for _, cause := range []error{iface.ErrNativePPPPending, sendCause, unix.ESRCH, waitCause, parseCause} {
				if !errors.Is(err, cause) {
					t.Fatalf("original cause %v was not retained in %v", cause, err)
				}
			}
			var rejected interface{ NativePPPWriteRejected() bool }
			if errors.As(err, &rejected) {
				t.Fatal("uncertain native errno became a genuine matched NACK")
			}
		})
	}
	if owner.calls != 0 {
		t.Fatal("uncertain native outcome borrowed or resent a descriptor")
	}
	if writer.pending == nil {
		t.Fatal("uncertain native transaction lost its original custody")
	}
	if backend.counters.closed {
		t.Fatal("failed backend close destroyed original raw-counter custody")
	}
	if len(backend.nativePPPs.writers) != 1 {
		t.Fatal("failed backend close discarded its unresolved original writer")
	}
}

// TestNativePPPModelAdmissionLabelsRefuseBeforeNativeIO checks boundary shapes
// that cannot carry an actual original owner. Its control is another refusal,
// never a fake positive native capability receipt.
func TestNativePPPModelAdmissionLabelsRefuseBeforeNativeIO(t *testing.T) {
	var typedNil *nativePPPModelRefuser
	owner := &nativePPPModelRefuser{cause: errors.New("no model native lease")}
	for _, entry := range []struct {
		name  string
		owner iface.NativePPPUnitOwner
		index int
		unit  int32
	}{
		{"nil owner", nil, 37, 2},
		{"typed nil owner", typedNil, 37, 2},
		{"zero index", owner, 0, 2},
		{"negative index", owner, -1, 2},
		{"negative unit", owner, 37, -1},
	} {
		t.Run(entry.name, func(t *testing.T) {
			backend := &netlinkBackend{}
			writer, err := backend.BindNativePPPUnit(entry.owner, entry.index, entry.unit)
			if err == nil {
				t.Fatal("invalid original owner boundary was admitted")
			}
			if writer != nil {
				t.Fatal("invalid original owner boundary produced a writer")
			}
		})
	}
	if owner.calls != 0 {
		t.Fatal("invalid labels borrowed a fake model file")
	}
}
