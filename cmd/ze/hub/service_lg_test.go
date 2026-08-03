//go:build ze_lg

package hub

// VALIDATES: the ze_lg-gated looking-glass factory -- parseASNForDecorator's
// numeric parsing/bounds and buildLGService's not-configured skip path.
// PREVENTS: an ASN decorator misparse (overflow/garbage) and a no-op build
// being treated as a failure.

import (
	"context"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

func lgTestDispatch() plugin.CommandDispatcher {
	return func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}
}

func TestParseASNForDecorator(t *testing.T) {
	cases := []struct {
		in   string
		want uint32
	}{
		{"0", 0},
		{"1", 1},
		{"65535", 65535},
		{"4294967295", 4294967295}, // max uint32, last valid
		{"4294967296", 0},          // one past max -> 0
		{"99999999999", 0},         // far past max -> 0
		{"", 0},                    // empty -> 0
		{"abc", 0},                 // non-numeric -> 0
		{"12x", 0},                 // trailing garbage -> 0
	}
	for _, c := range cases {
		if got := parseASNForDecorator(c.in); got != c.want {
			t.Errorf("parseASNForDecorator(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBuildLGService_NotConfigured(t *testing.T) {
	// No listen addresses -> the factory skips (nil service, nil error).
	svc, err := buildLGService(ServiceDeps{Dispatch: lgTestDispatch()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != nil {
		t.Fatalf("want nil service when not configured, got %#v", svc)
	}
}

func TestBuildLGService_ExplicitTLSWithoutBlobStorageFails(t *testing.T) {
	// VALIDATES: an operator who WROTE `tls true` (or set ze.looking-glass.tls)
	// and has no certificate store gets an error, never a quiet plaintext
	// looking glass. Their instruction cannot be honored, so it is refused
	// rather than approximated (ai/rules/protocol.md).
	// PREVENTS: the default-on fallback below widening into a downgrade of TLS
	// the operator explicitly demanded.
	_, err := buildLGService(ServiceDeps{
		Dispatch:      lgTestDispatch(),
		LGAddrs:       []string{"127.0.0.1:0"},
		LGTLS:         true,
		LGTLSExplicit: true,
		Store:         storage.NewFilesystem(),
	})
	if err == nil {
		t.Fatal("explicit TLS with no blob storage must fail, not fall back to plaintext")
	}
	if !strings.Contains(err.Error(), "blob storage") {
		t.Fatalf("error must name the missing certificate store, got %v", err)
	}
}

func TestBuildLGService_DefaultTLSWithoutBlobStorageServesPlaintext(t *testing.T) {
	// VALIDATES: TLS-on-by-default yields to the prior plaintext behavior when
	// there is no certificate store and the operator asked for nothing. A
	// hardening default must not turn a working looking glass into a missing
	// one; the warning on stderr names `ze init` as the remedy.
	// PREVENTS: every file-config deployment that never ran `ze init` losing its
	// looking glass to a silent build failure.
	svc, err := buildLGService(ServiceDeps{
		Dispatch:      lgTestDispatch(),
		LGAddrs:       []string{"127.0.0.1:0"},
		LGTLS:         true,
		LGTLSExplicit: false,
		Store:         storage.NewFilesystem(),
	})
	if err != nil {
		t.Fatalf("defaulted TLS with no blob storage must still start: %v", err)
	}
	if svc == nil {
		t.Fatal("want a running looking glass, got nil")
	}
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
}
