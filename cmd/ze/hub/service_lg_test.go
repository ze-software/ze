//go:build ze_lg

package hub

// VALIDATES: the ze_lg-gated looking-glass factory -- buildLGService's
// not-configured skip path, its TLS choices, and that the service it builds
// names AS numbers through the dispatcher with no resolver in its deps.
// PREVENTS: a no-op build being treated as a failure, and a private path to a
// Team Cymru resolver beside the resolve component's command.

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/lg"
	"github.com/ze-software/ze/internal/component/plugin"
	resolvecmd "github.com/ze-software/ze/internal/component/resolve/cmd"
)

func lgTestDispatch() plugin.CommandDispatcher {
	return func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}
}

// lgTestListeners returns one loopback looking-glass listener on a port the
// kernel chooses, whose server entry says tls about its protocol.
func lgTestListeners(tls zeconfig.LGServerTLS) []lgListener {
	return []lgListener{{addr: "127.0.0.1:0", tls: tls}}
}

// TestLGUnknownASNameMatchesTheResolveCommand compares the two copies of the
// word `resolve cymru asn-name` answers for an AS Team Cymru does not name.
// The looking glass cannot import the resolve component, so it holds a copy;
// this is the check that names the copy's source.
//
// VALIDATES: lg.ASNameUnknown is resolvecmd.ASNameUnknown.
// PREVENTS: the looking glass drawing "(unknown)" as an organization name
// after the command's word changes.
func TestLGUnknownASNameMatchesTheResolveCommand(t *testing.T) {
	if lg.ASNameUnknown != resolvecmd.ASNameUnknown {
		t.Fatalf("lg.ASNameUnknown = %q, resolvecmd.ASNameUnknown = %q: they MUST be equal",
			lg.ASNameUnknown, resolvecmd.ASNameUnknown)
	}
}

// TestBuildLGService_NamesASThroughTheDispatcher builds the looking glass the
// way the hub does, with no resolver in its deps, and reads the peers page.
//
// VALIDATES: the built service asks `resolve cymru asn-name <asn>` of the
// dispatcher it was given, and draws the name the command answers.
// PREVENTS: the looking glass depending on resolvers.Cymru (the old
// DecorateASN closure), which named nothing when the deps carried none.
func TestBuildLGService_NamesASThroughTheDispatcher(t *testing.T) {
	const summary = `{"peers":[{"address":"192.0.2.1","remote-as":"64500","state":"established"}]}`
	var mu sync.Mutex
	var asked []string
	dispatch := func(_ context.Context, _ plugin.CallerIdentity, command string) (*plugin.Response, error) {
		mu.Lock()
		asked = append(asked, command)
		mu.Unlock()
		if command == "resolve cymru asn-name 64500" {
			// The resolve component's answer shape (handleCymruASNName).
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"name": "Example Transit"}), nil
		}
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(summary)), nil
	}

	svc, err := buildLGService(&serviceDeps{
		Dispatch:    dispatch,
		LGListeners: lgTestListeners(zeconfig.LGServerTLSOff),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if svc == nil {
		t.Fatal("want a running looking glass, got nil")
	}
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })

	lg, ok := svc.(lgService)
	if !ok {
		t.Fatalf("service = %T, want lgService", svc)
	}
	addresses := lg.Addresses()
	if len(addresses) == 0 {
		t.Fatal("the looking glass bound no address")
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addresses[0]+"/lg/peers", http.NoBody)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /lg/peers: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if !strings.Contains(string(body), "Example Transit") {
		t.Errorf("the peers page does not draw the name the command answered:\n%s", body)
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(strings.Join(asked, "|"), "resolve cymru asn-name 64500") {
		t.Errorf("the dispatcher was not asked for the name: %q", asked)
	}
}

func TestBuildLGService_NotConfigured(t *testing.T) {
	// No listen addresses -> the factory skips (nil service, nil error).
	svc, err := buildLGService(&serviceDeps{Dispatch: lgTestDispatch()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != nil {
		t.Fatalf("want nil service when not configured, got %#v", svc)
	}
}

func TestBuildLGService_ExplicitTLSWithoutBlobStorageFails(t *testing.T) {
	// VALIDATES: an operator who WROTE `tls true` on a server entry and has
	// no certificate store gets an error, never a quiet plaintext
	// looking glass. Their instruction cannot be honored, so it is refused
	// rather than approximated (ai/rules/protocol.md).
	// PREVENTS: the default-on fallback below widening into a downgrade of TLS
	// the operator explicitly demanded.
	svc, err := buildLGService(&serviceDeps{
		Dispatch:    lgTestDispatch(),
		LGListeners: lgTestListeners(zeconfig.LGServerTLSRequired),
		Store:       nil,
	})
	if err == nil {
		t.Fatal("explicit TLS without persistent storage must fail, not fall back to plaintext")
	}
	if svc != nil {
		t.Fatal("refused TLS configuration started a listener")
	}
}

func TestBuildLGService_DefaultTLSWithoutBlobStorageServesPlaintext(t *testing.T) {
	// VALIDATES: TLS-on-by-default yields to the prior plaintext behavior when
	// there is no certificate store and the operator asked for nothing. A
	// hardening default must not turn a working looking glass into a missing
	// one; the warning on stderr names `ze init` as the remedy.
	// PREVENTS: every file-config deployment that never ran `ze init` losing its
	// looking glass to a silent build failure, and a warning that drops the
	// address of the entry it is about.
	var svc Service
	var err error
	warned := captureStderrFile(t, func() {
		svc, err = buildLGService(&serviceDeps{
			Dispatch:    lgTestDispatch(),
			LGListeners: lgTestListeners(zeconfig.LGServerTLSInherited),
			Store:       nil,
		})
	})
	if err != nil {
		t.Fatalf("defaulted TLS with no blob storage must still start: %v", err)
	}
	if svc == nil {
		t.Fatal("want a running looking glass, got nil")
	}
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	want := "warning: " + lgDowngradeWarning(lgTestListeners(zeconfig.LGServerTLSInherited)[0].addr).Error()
	if !strings.Contains(warned, want) {
		t.Fatalf("stderr does not carry the downgrade warning %q:\n%s", want, warned)
	}
	for _, part := range []string{"127.0.0.1:0 serves plain HTTP", "\nfix: ", "ze init", "tls false"} {
		if !strings.Contains(warned, part) {
			t.Fatalf("the downgrade warning does not name %q:\n%s", part, warned)
		}
	}
}
