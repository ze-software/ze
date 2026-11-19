// Design: docs/features/interfaces.md -- terminal settlement history after Close
// Related: native_ppp_settlement_history_integration_linux_test.go -- actual ACK/NACK
// These are non-native lifecycle models. A task-owned pipe tests file Close;
// no modeled owner or descriptor reaches a Linux mutation or ACK receive.

//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// Goal: final physical Close preserves a completed settlement's causes without
// retaining custody. Method: model terminal history on a still-open task-owned
// pipe, then close through both writer-first and backend-first entry points.
func TestNativePPPModelSettlementHistorySurvivesPhysicalClose(t *testing.T) {
	for _, entry := range []struct {
		name         string
		backendFirst bool
	}{
		{"writer close first", false},
		{"backend close first", true},
	} {
		t.Run(entry.name, func(t *testing.T) {
			read, write, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range []*os.File{read, write} {
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						if !errors.Is(err, os.ErrClosed) {
							t.Errorf("close task-owned lifecycle pipe: %v", err)
						}
					}
				})
			}
			// Namespace restoration is cause-only history. It cannot confer a
			// pending transaction or turn completed physical release into custody.
			namespaceCause := errors.New("model original namespace restoration failure")
			history := errors.Join(context.Canceled, namespaceCause)
			backend := &netlinkBackend{}
			owner := &nativePPPModelRefuser{cause: errors.New("must not borrow after settlement")}
			writer := &nativePPPWriter{parent: &backend.nativePPPs, owner: owner,
				file: read, closeErr: history}
			backend.nativePPPs.writers = map[*nativePPPWriter]struct{}{writer: {}}
			completed := backend.closed.channel()
			if !writer.ClosePending() {
				t.Fatal("model still-open original file did not owe Close")
			}
			closeOriginal := writer.Close
			if entry.backendFirst {
				closeOriginal = backend.Close
			}
			closeErr := closeOriginal()
			if _, err := read.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Errorf("model original file did not complete Close: %v", err)
			}
			if writer.ClosePending() {
				t.Error("terminal cause-only history retained custody after successful Close")
			}
			for _, cause := range []error{context.Canceled, namespaceCause} {
				if !errors.Is(closeErr, cause) {
					t.Errorf("physical Close overwrote completed settlement cause %v: %v", cause, closeErr)
				}
			}
			for _, cleanup := range []func() error{
				func() error { return writer.Settle(context.Background()) },
				writer.Close,
				backend.Close,
				backend.Close,
			} {
				cleanupErr := cleanup()
				for _, cause := range []error{context.Canceled, namespaceCause} {
					if !errors.Is(cleanupErr, cause) {
						t.Errorf("repeated terminal cleanup lost cause %v: %v", cause, cleanupErr)
					}
				}
				if errors.Is(cleanupErr, iface.ErrNativePPPPending) {
					t.Error("terminal settlement history invented unresolved custody")
				}
			}
			if !backend.counters.closed {
				t.Error("terminal settlement history prevented final counter cleanup")
			}
			if len(backend.nativePPPs.writers) != 0 {
				t.Error("successful physical Close retained its terminal writer")
			}
			select {
			case <-completed:
			default:
				t.Error("terminal settlement history prevented backend completion")
			}
			if owner.calls != 0 {
				t.Error("terminal cleanup borrowed an original descriptor")
			}
		})
	}
}
