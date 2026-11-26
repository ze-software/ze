//go:build ze_lg

// Design: docs/architecture/core-design.md -- a looking-glass entry that cannot be served stays down, and the others serve

package hub

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// lgStartRecorded starts a looking glass from deps and returns the service,
// the log it wrote, what it printed and the start error.
func lgStartRecorded(t *testing.T, deps *serviceDeps) (svc Service, logged, console string, startErr error) {
	t.Helper()
	var logText, consoleText strings.Builder
	svc, startErr = startLGService(deps, slog.New(slog.NewTextHandler(&logText, nil)), &consoleText)
	if svc != nil {
		t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	}
	return svc, logText.String(), consoleText.String(), startErr
}

// lgRequireReported fails unless text holds each of want.
func lgRequireReported(t *testing.T, where, text string, want ...string) {
	t.Helper()
	for _, piece := range want {
		if !strings.Contains(text, piece) {
			t.Fatalf("%s does not hold %q: %s", where, piece, text)
		}
	}
}

func TestStartLGServiceLeavesTLSTrueEntryDown(t *testing.T) {
	// VALIDATES: a looking glass with one `tls false` entry and one `tls true`
	// entry, on a daemon with no store and no named certificate, starts: the
	// `tls false` entry answers HTTP, the `tls true` entry is not bound, and
	// the log and the terminal name it under doctor-lg-tls-unservable with the
	// fix.
	// PREVENTS: one entry that cannot be served taking the looking glass down,
	// and that entry served over HTTP.
	loadLGPKIStore(t)

	svc, logged, console, err := lgStartRecorded(t, &serviceDeps{
		Dispatch: lgTestDispatch(),
		LGListeners: []lgListener{
			{addr: "127.0.0.1:0", tls: zeconfig.LGServerTLSOff},
			{addr: lgFreeAddr(t), tls: zeconfig.LGServerTLSRequired},
		},
		Store: nil,
	})
	if err != nil {
		t.Fatalf("a looking glass with one servable entry must start: %v", err)
	}
	if svc == nil {
		t.Fatal("want a running looking glass, got nil")
	}
	addrs := svc.Addresses()
	if len(addrs) != 1 {
		t.Fatalf("bound %d listeners, want only the tls false entry: %v", len(addrs), addrs)
	}
	lgRequireHTTP(t, addrs[0], "the tls false entry")

	lgRequireReported(t, "the log", logged, "level=ERROR", "code="+codeDoctorLGTLSUnservable, "tls true", "ze init", "is not started", "keep running")
	lgRequireReported(t, "the terminal", console, "error: ", "is not started", "fix: ", "ze init")
}

func TestStartLGServiceUnresolvedNameLeavesTLSEntriesDown(t *testing.T) {
	// VALIDATES: a looking glass whose named certificate does not resolve
	// starts with its `tls false` entry on HTTP and every entry that asks for
	// TLS down, on a daemon that has a store: the self-signed pair is never
	// served in place of the named certificate. Each down entry is reported
	// with the name and the fix.
	// PREVENTS: a daemon that exits over the looking-glass certificate name,
	// and a self-signed or plain-HTTP listener where the config names a
	// certificate.
	loadLGPKIStore(t)
	unset, written := lgFreeAddr(t), lgFreeAddr(t)

	svc, logged, console, err := lgStartRecorded(t, &serviceDeps{
		Dispatch: lgTestDispatch(),
		LGListeners: []lgListener{
			{addr: "127.0.0.1:0", tls: zeconfig.LGServerTLSOff},
			{addr: unset, tls: zeconfig.LGServerTLSInherited},
			{addr: written, tls: zeconfig.LGServerTLSRequired},
		},
		LGCertificate: "typo-cert",
		Store:         lgBlobStorage(t),
	})
	if err != nil {
		t.Fatalf("a looking glass with one servable entry must start: %v", err)
	}
	if svc == nil {
		t.Fatal("want a running looking glass, got nil")
	}
	addrs := svc.Addresses()
	if len(addrs) != 1 {
		t.Fatalf("bound %d listeners, want only the tls false entry: %v", len(addrs), addrs)
	}
	lgRequireHTTP(t, addrs[0], "the tls false entry")

	lgRequireReported(t, "the log", logged, "code="+codeDoctorLGTLSUnservable, unset, written, "typo-cert", "edit pki certificate typo-cert", "keep running")
	lgRequireReported(t, "the terminal", console, unset, written, "typo-cert", "fix: ")
	// The downgrade warning, not the refusal: the refusal itself says that ze
	// never serves plain HTTP in place of a named certificate.
	if strings.Contains(console, "serves plain HTTP") {
		t.Fatalf("an entry that left tls unset was downgraded to HTTP in place of the named certificate: %s", console)
	}
}

func TestStartLGServiceWithNoServableEntryStartsNothing(t *testing.T) {
	// VALIDATES: a looking glass none of whose entries can be served builds no
	// service, reports the entry, and answers the factory an error, which
	// buildServices logs before it builds the next service
	// (TestServiceRegistry_BuildErrorSkipped): ze keeps running.
	// PREVENTS: an empty looking-glass server that binds nothing and reads as
	// running.
	loadLGPKIStore(t)
	addr := lgFreeAddr(t)

	svc, logged, _, err := lgStartRecorded(t, &serviceDeps{
		Dispatch:    lgTestDispatch(),
		LGListeners: []lgListener{{addr: addr, tls: zeconfig.LGServerTLSRequired}},
		Store:       nil,
	})
	if svc != nil {
		t.Fatal("a looking glass with no servable entry must build no service")
	}
	if err == nil {
		t.Fatal("the factory must answer why the looking glass did not start")
	}
	lgRequireReported(t, "the factory error", err.Error(), addr, "tls true", "ze init")
	lgRequireReported(t, "the log", logged, "code="+codeDoctorLGTLSUnservable, addr)
}
