// Design: docs/features/interfaces.md -- terminal PPP close history and backend cleanup
// These are non-native lifecycle models. No fake descriptor grants authority.

//go:build linux

package ifacenetlink

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestNativePPPModelTerminalCloseErrorFinishesBackend requires backend cleanup
// after both an empty writer set's historical error and a newly observed terminal
// writer error. The original error survives; it never masquerades as unresolved
// custody or prevents final counter cleanup on repeated Close.
func TestNativePPPModelTerminalCloseErrorFinishesBackend(t *testing.T) {
	for _, entry := range []struct {
		name           string
		terminalWriter bool
	}{
		{"empty writer set with close history", false},
		{"terminal writer with close error", true},
	} {
		t.Run(entry.name, func(t *testing.T) {
			history := errors.New("model original socket terminal close error")
			backend := &netlinkBackend{}
			if entry.terminalWriter {
				// Already closed is a model state, not a fake native file. No
				// syscall may use this model or a fabricated descriptor.
				writer := &nativePPPWriter{parent: &backend.nativePPPs,
					owner: &nativePPPModelRefuser{}, closed: true, closeErr: history}
				backend.nativePPPs.writers = map[*nativePPPWriter]struct{}{writer: {}}
			} else {
				backend.nativePPPs.closeErr = history
			}
			for range 2 {
				err := backend.Close()
				if !errors.Is(err, history) {
					t.Fatalf("terminal close history was lost: %v", err)
				}
				if errors.Is(err, iface.ErrNativePPPPending) {
					t.Fatal("terminal close history became unresolved native custody")
				}
				if !backend.counters.closed {
					t.Fatal("terminal PPP close error prevented final backend counter cleanup")
				}
				if !backend.nativePPPs.closed.Load() {
					t.Fatal("backend Close left new PPP admission open")
				}
				if len(backend.nativePPPs.writers) != 0 {
					t.Fatal("terminal writer remained in backend custody")
				}
			}
		})
	}
}

// TestNativePPPModelPendingWithHistoryKeepsBackend pairs the terminal cases
// with real model pending custody. Both independent causes survive and the
// backend's original counter owner stays alive; no retirement is invented.
func TestNativePPPModelPendingWithHistoryKeepsBackend(t *testing.T) {
	history := errors.New("model previous terminal close error")
	pending := errors.New("model unresolved original ACK outcome")
	backend := &netlinkBackend{}
	backend.nativePPPs.closeErr = history
	owner := &nativePPPModelRefuser{}
	writer := &nativePPPWriter{parent: &backend.nativePPPs, owner: owner}
	writer.pending = &nativePPPTransaction{writer: writer, poison: pending}
	backend.nativePPPs.writers = map[*nativePPPWriter]struct{}{writer: {}}
	err := backend.Close()
	for _, cause := range []error{history, pending, iface.ErrNativePPPPending} {
		if !errors.Is(err, cause) {
			t.Fatalf("original independent cause %v was lost in %v", cause, err)
		}
	}
	if backend.counters.closed {
		t.Fatal("unresolved original PPP custody lost the original backend counter owner")
	}
	if len(backend.nativePPPs.writers) != 1 {
		t.Fatal("unresolved original PPP writer was discarded")
	}
	if owner.calls != 0 {
		t.Fatal("backend close acquired a new original descriptor instead of preserving custody")
	}
}
