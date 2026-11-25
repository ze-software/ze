// Design: docs/features/interfaces.md -- passive PPP transport retirement
// These are NON-NATIVE state models. Their owners do not certify a Linux unit.
// A task-owned pipe tests socket-file Close only; no modeled fd reaches the netlink socket.

//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
)

type nativePPPRetirementModelOwner struct {
	nativePPPModelRefuser
	waits      int
	retirement error
	retired    bool
}

func (owner *nativePPPRetirementModelOwner) WaitNativePPPUnitRetired(ctx context.Context) error {
	owner.waits++
	if ctx.Err() != context.Canceled {
		return errors.New("model observation was not already canceled")
	}
	if owner.retired {
		return owner.retirement
	}
	return ctx.Err()
}

func nativePPPRetirementModel(t *testing.T, owner iface.NativePPPUnitOwner) (*netlinkBackend, *nativePPPWriter, []error) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = read.Close(); _ = write.Close() })
	backend := &netlinkBackend{}
	writer := &nativePPPWriter{parent: &backend.nativePPPs, owner: owner, file: read}
	causes := []error{errors.New("model original uncertain send"), unix.ESRCH,
		errors.New("model original ACK wait"), errors.New("model original poisoned ACK")}
	writer.pending = &nativePPPTransaction{writer: writer, entered: true,
		cause: causes[0], nativeErr: causes[1], waitErrs: []error{causes[2]}, poison: causes[3]}
	backend.nativePPPs.writers = map[*nativePPPWriter]struct{}{writer: {}}
	return backend, writer, causes
}

func nativePPPRetirementModelRequireCauses(t *testing.T, err error, causes []error) {
	t.Helper()
	for _, cause := range causes {
		if !errors.Is(err, cause) {
			t.Fatalf("lost original cause %v in %v", cause, err)
		}
	}
	var rejected interface{ NativePPPWriteRejected() bool }
	if errors.As(err, &rejected) {
		t.Fatal("retirement invented a matched native NACK")
	}
}

// Pending and failed original retirement preserve the original socket/backend.
// Only the same owner's positive completed result ends uncertain transport.
func TestNativePPPModelRetirementPreservesCustodyAndCauses(t *testing.T) {
	owner := &nativePPPRetirementModelOwner{}
	backend, writer, causes := nativePPPRetirementModel(t, owner)
	for range 2 {
		for _, call := range []func() error{func() error { return writer.Settle(context.Background()) }, writer.Close, backend.Close} {
			err := call()
			nativePPPRetirementModelRequireCauses(t, err, causes)
			if !errors.Is(err, iface.ErrNativePPPPending) {
				t.Fatalf("negative retirement ended custody: %v", err)
			}
			if writer.closed || writer.pending == nil || backend.counters.closed {
				t.Fatal("negative retirement destroyed custody")
			}
		}
	}
	owner.retired = true
	err := writer.Settle(context.Background())
	nativePPPRetirementModelRequireCauses(t, err, causes)
	if err == nil || errors.Is(err, iface.ErrNativePPPPending) {
		t.Fatalf("terminal unknown outcome was not reported distinctly: %v", err)
	}
	if writer.pending != nil || !writer.closed {
		t.Fatal("completed retirement retained socket custody")
	}
	raw, rawErr := writer.file.SyscallConn()
	if rawErr != nil || raw == nil {
		t.Fatalf("original socket raw wrapper unavailable: %v", rawErr)
	}
	controlEntered := false
	controlErr := raw.Control(func(uintptr) { controlEntered = true })
	if controlErr == nil || controlEntered {
		t.Fatalf("original socket file remained usable: callback entered=%v, error=%v", controlEntered, controlErr)
	}
	if _, err := writer.file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("original socket file not actually closed: %v", err)
	}
	for _, call := range []func() error{func() error { return writer.Settle(context.Background()) }, writer.Close, backend.Close, backend.Close} {
		err := call()
		nativePPPRetirementModelRequireCauses(t, err, causes)
		if errors.Is(err, iface.ErrNativePPPPending) {
			t.Fatalf("historical error resurrected custody: %v", err)
		}
	}
	if !backend.counters.closed || len(backend.nativePPPs.writers) != 0 {
		t.Fatal("terminal history prevented final backend cleanup")
	}
	if owner.calls != 0 {
		t.Fatal("retirement borrowed or resent an original descriptor")
	}
}

func TestNativePPPModelFailedRetirementCannotSettle(t *testing.T) {
	closeCause := errors.New("model original unit close failed")
	owner := &nativePPPRetirementModelOwner{retired: true, retirement: closeCause}
	backend, writer, causes := nativePPPRetirementModel(t, owner)
	for range 2 {
		for _, call := range []func() error{func() error { return writer.Settle(context.Background()) }, writer.Close, backend.Close} {
			err := call()
			nativePPPRetirementModelRequireCauses(t, err, append(causes, closeCause))
			if !errors.Is(err, iface.ErrNativePPPPending) {
				t.Fatalf("failed original close granted retirement: %v", err)
			}
			if writer.closed || writer.pending == nil || backend.counters.closed {
				t.Fatal("failed original close destroyed custody")
			}
		}
	}
	if owner.calls != 0 {
		t.Fatal("failed retirement borrowed or resent an original descriptor")
	}
}

func TestNativePPPModelRetirementAtTurnLimitAndSocketCloseFailure(t *testing.T) {
	owner := &nativePPPRetirementModelOwner{retired: true}
	backend, writer, causes := nativePPPRetirementModel(t, owner)
	writer.pending.poison = nil
	writer.pending.turns = nativePPPSettlementLimit
	if err := writer.file.Close(); err != nil {
		t.Fatal(err)
	}
	err := writer.Settle(context.Background())
	nativePPPRetirementModelRequireCauses(t, err, causes[:3])
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("terminal settlement lost actual socket Close failure: %v", err)
	}
	if errors.Is(err, iface.ErrNativePPPPending) || writer.pending != nil {
		t.Fatal("turn limit prevented genuine retirement")
	}
	if got := err.Error(); !strings.Contains(got, "settlement turn limit reached") {
		t.Fatalf("lost 64-turn history: %v", err)
	}
	if !writer.ClosePending() {
		t.Fatal("failed socket Close discarded custody after unit retirement")
	}
	err = backend.Close()
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("backend cleanup lost original socket Close failure: %v", err)
	}
	if backend.counters.closed {
		t.Fatal("failed socket Close released unrelated backend resources")
	}
	if _, retained := backend.nativePPPs.writers[writer]; !retained {
		t.Fatal("failed socket Close deregistered its original writer")
	}
}

// Unavailability is produced at known authority/capability branches. Ordinary
// route/alias validation and context/refusal errors must not gain that facet.
func TestNativePPPModelUnavailableFacet(t *testing.T) {
	refusal := errors.New("model ordinary descriptor refusal")
	owner := &nativePPPModelRefuser{cause: refusal}
	var absent *nativePPPWriter
	var typedNil *nativePPPModelRefuser
	closedBackend := &netlinkBackend{}
	closedBackend.nativePPPs.closed.Store(true)
	for _, entry := range []struct {
		name string
		call func() error
	}{
		{"nil writer", func() error { return absent.SetAdminUp(context.Background()) }},
		{"uninitialized writer", func() error { return (&nativePPPWriter{}).SetAdminUp(context.Background()) }},
		{"closed writer", func() error { return (&nativePPPWriter{owner: owner, closed: true}).SetAdminUp(context.Background()) }},
		{"closed backend", func() error {
			return (&nativePPPWriter{owner: owner, parent: &closedBackend.nativePPPs}).SetAdminUp(context.Background())
		}},
		{"missing feature", func() error {
			return (&nativePPPWriter{owner: owner, parent: &nativePPPResources{}}).SetAdminUp(context.Background())
		}},
		{"closed original borrow", func() error {
			return (&nativePPPWriter{owner: &nativePPPModelRefuser{cause: os.ErrClosed}, parent: &nativePPPResources{}, features: nativePPPSetlink}).SetAdminUp(context.Background())
		}},
		{"missing bind owner", func() error { _, err := (&netlinkBackend{}).BindNativePPPUnit(nil, 37, 2); return err }},
		{"typed nil bind owner", func() error { _, err := (&netlinkBackend{}).BindNativePPPUnit(typedNil, 37, 2); return err }},
		{"closed bind backend", func() error { _, err := closedBackend.BindNativePPPUnit(owner, 37, 2); return err }},
	} {
		t.Run(entry.name, func(t *testing.T) {
			err := entry.call()
			if !errors.Is(err, iface.ErrNativePPPUnavailable) {
				t.Fatalf("known missing authority/capability lacks facet: %v", err)
			}
			if errors.Is(err, iface.ErrNativePPPPending) {
				t.Fatalf("unsubmitted unavailable operation became pending: %v", err)
			}
		})
	}
	writer := &nativePPPWriter{owner: owner, parent: &nativePPPResources{}, features: nativePPPSetlink}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, entry := range []struct {
		name string
		call func() error
	}{
		{"route validation", func() error {
			return writer.AddRoute(context.Background(), netip.Prefix{}, netip.Addr{}, 1, rtproto.Iface)
		}},
		{"alias validation", func() error { return writer.SetAlias(context.Background(), "bad\x00alias") }},
		{"context cancellation", func() error { return writer.SetAdminUp(canceled) }},
		{"ordinary borrow refusal", func() error { return writer.SetAdminUp(context.Background()) }},
		{"settled native route NACK model", func() error { return &nativePPPRejected{cause: unix.ESRCH} }},
	} {
		t.Run(entry.name, func(t *testing.T) {
			err := entry.call()
			if err == nil || errors.Is(err, iface.ErrNativePPPUnavailable) || errors.Is(err, iface.ErrNativePPPPending) {
				t.Fatalf("ordinary failure gained fatal authority classification: %v", err)
			}
		})
	}
}

func TestNativePPPModelClosePendingSeparatesCustodyFromHistory(t *testing.T) {
	var absent *nativePPPWriter
	if absent.ClosePending() || (&nativePPPWriter{}).ClosePending() {
		t.Fatal("unavailable writer invented custody")
	}
	_, writer, _ := nativePPPRetirementModel(t, &nativePPPRetirementModelOwner{retired: true})
	if !writer.ClosePending() {
		t.Fatal("pending original transaction lost custody")
	}
	if err := writer.Close(); err == nil {
		t.Fatal("unknown retirement outcome lost history")
	}
	if writer.ClosePending() {
		t.Fatal("terminal unknown-outcome history became native custody")
	}
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = read.Close(); _ = write.Close() })
	owner := &nativePPPModelRefuser{}
	writer = &nativePPPWriter{parent: &nativePPPResources{}, owner: owner, file: read}
	if !writer.ClosePending() {
		t.Fatal("unused live socket did not owe Close")
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("actual socket Close error was lost: %v", err)
	}
	if !writer.ClosePending() {
		t.Fatal("failed socket Close discarded custody without physical completion proof")
	}
}
