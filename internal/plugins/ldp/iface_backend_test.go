// Design: docs/architecture/ldp/mpls-ldp.md -- interface backend before the first interface read
package ldp

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingHandler is a slog.Handler that keeps every record it is given, so a
// test can read the attributes of the line the code logged.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface mandates Record by value
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

// fixAttr returns the "fix" attribute of the first WARN record, or "".
func (h *recordingHandler) fixAttr() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.records {
		if h.records[i].Level != slog.LevelWarn {
			continue
		}
		found := ""
		h.records[i].Attrs(func(a slog.Attr) bool {
			if a.Key != "fix" {
				return true
			}
			found = a.Value.String()
			return false
		})
		return found
	}
	return ""
}

// errorAttr returns the "error" attribute of the first WARN record, or nil.
func (h *recordingHandler) errorAttr() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.records {
		if h.records[i].Level != slog.LevelWarn {
			continue
		}
		var found error
		h.records[i].Attrs(func(a slog.Attr) bool {
			if a.Key != "error" {
				return true
			}
			if e, ok := a.Value.Any().(error); ok {
				found = e
			}
			return false
		})
		return found
	}
	return nil
}

// stubIfaceBackend replaces ensureIfaceBackend for one test with a stub that
// records the call and fails with the returned sentinel.
func stubIfaceBackend(t *testing.T) (called *bool, sentinel error) {
	t.Helper()
	prev := ensureIfaceBackend
	t.Cleanup(func() { ensureIfaceBackend = prev })

	sentinel = errors.New("no backend for this test")
	called = new(bool)
	ensureIfaceBackend = func() error {
		*called = true
		return sentinel
	}
	return called, sentinel
}

// TestInterfacePrefixesEnsuresIfaceBackend pins the order an LDP-only config
// depends on. The iface component loads its backend from an `interface { }`
// block alone, and a config that names its interfaces only under `ldp { }` has
// none, so iface.Addresses failed "iface: no backend loaded" and LDP originated
// no label for its connected prefixes.
//
// PREVENTS: dropping the ensure, or reading the addresses before it. The stub
// records the call and fails, so the test proves interfacePrefixes calls it and
// returns its error, naming the interface, instead of reading the addresses.
func TestInterfacePrefixesEnsuresIfaceBackend(t *testing.T) {
	called, sentinel := stubIfaceBackend(t)

	_, err := interfacePrefixes("eth-does-not-exist")
	if !*called {
		t.Fatal("interfacePrefixes did not ensure an iface backend before reading addresses")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want it to wrap the EnsureBackend failure", err)
	}
	if !strings.Contains(err.Error(), "eth-does-not-exist") {
		t.Errorf("error %q does not name the interface", err)
	}
}

// TestWaitForInterfaceEnsuresIfaceBackend pins the same order for Basic
// Discovery. Without the ensure, iface.Resolve failed "iface: no backend
// loaded" on every retry, the interface never became available, and LDP sent
// no Hello on it (TestLDPInteropFRR, plan/journal/test-against-broken-path.md).
//
// PREVENTS: dropping the ensure, and swallowing the reason. The context is
// already canceled, so waitForInterface makes one attempt and returns; the test
// proves the attempt called the stub and that the warning names its error.
func TestWaitForInterfaceEnsuresIfaceBackend(t *testing.T) {
	called, sentinel := stubIfaceBackend(t)
	rec := &recordingHandler{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if ifi := waitForInterface(ctx, slog.New(rec), "eth-does-not-exist", time.Second); ifi != nil {
		t.Fatalf("waitForInterface returned %v, want nil when no backend loads", ifi)
	}
	if !*called {
		t.Fatal("waitForInterface did not ensure an iface backend before resolving")
	}
	if got := rec.errorAttr(); !errors.Is(got, sentinel) {
		t.Errorf("warning error = %v, want it to carry the EnsureBackend failure", got)
	}
	if got := rec.fixAttr(); got != ifaceBackendFix {
		t.Errorf("warning fix = %q, want the backend config step %q", got, ifaceBackendFix)
	}
}

// TestConnectedPrefixesWarningNamesBackendFix proves the connected-prefix
// warning tells the operator which config loads a backend when none loads.
//
// PREVENTS: a warning that says what LDP loses but not how to fix it.
func TestConnectedPrefixesWarningNamesBackendFix(t *testing.T) {
	_, sentinel := stubIfaceBackend(t)
	rec := &recordingHandler{}

	if got := connectedPrefixes([]string{"eth-does-not-exist"}, slog.New(rec)); len(got) != 0 {
		t.Fatalf("connectedPrefixes = %v, want none when no backend loads", got)
	}
	if got := rec.errorAttr(); !errors.Is(got, sentinel) {
		t.Errorf("warning error = %v, want it to carry the EnsureBackend failure", got)
	}
	if got := rec.fixAttr(); got != ifaceBackendFix {
		t.Errorf("warning fix = %q, want the backend config step %q", got, ifaceBackendFix)
	}
}

// TestIfaceReadFixNamesDeviceCheck proves a read that failed for a reason
// other than the backend points the operator at the device, by its name.
//
// PREVENTS: the backend config step offered for a missing device.
func TestIfaceReadFixNamesDeviceCheck(t *testing.T) {
	got := ifaceReadFix(errors.New("resolve interface eth9: not found"), "eth9")
	if got == ifaceBackendFix {
		t.Fatalf("fix = %q, want the device check, not the backend step", got)
	}
	if !strings.HasSuffix(got, "ip link show eth9") {
		t.Errorf("fix = %q, want it to end with the command ip link show eth9", got)
	}
}
