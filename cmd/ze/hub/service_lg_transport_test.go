//go:build ze_lg

// Design: docs/architecture/pki/tls-listeners.md -- each looking-glass entry chooses HTTPS or HTTP, at start and at reload

package hub

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// lgChainAt handshakes with addr and returns the certificates it presented.
func lgChainAt(t *testing.T, addr string) []*x509.Certificate {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // the test inspects the chain rather than trusting it
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("TLS handshake with %s: %v", addr, err)
	}
	defer conn.Close() //nolint:errcheck // test client teardown
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		t.Fatalf("dialed connection is %T, not TLS", conn)
	}
	return tlsConn.ConnectionState().PeerCertificates
}

// lgRequireHTTPS fails unless addr presents the PKI store leaf over TLS.
func lgRequireHTTPS(t *testing.T, addr, when string) {
	t.Helper()
	chain := lgChainAt(t, addr)
	if chain[0].Subject.CommonName != lgLeafCommonName {
		t.Fatalf("%s: %s presents %q, want the PKI store leaf", when, addr, chain[0].Subject.CommonName)
	}
}

// lgPlainStatus sends one plain-HTTP GET to addr on a new connection and
// returns the status and the Location header. Redirects are not followed.
func lgPlainStatus(t *testing.T, addr, path string) (status int, location string) {
	t.Helper()
	client := &http.Client{
		Timeout:       10 * time.Second,
		Transport:     &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+path, http.NoBody)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET http://%s%s: %v", addr, path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // test response body
	return resp.StatusCode, resp.Header.Get("Location")
}

// lgRequireHTTP fails unless addr serves plain HTTP: the looking glass answers
// `GET /lg` with its redirect to the peers page, and a TLS handshake is refused.
// A TLS listener answers a plain request with 400, so the redirect is what
// tells the two apart.
func lgRequireHTTP(t *testing.T, addr, when string) {
	t.Helper()
	status, location := lgPlainStatus(t, addr, "/lg")
	if status != http.StatusFound || location != "/lg/peers" {
		t.Fatalf("%s: GET http://%s/lg answered %d to %q, want 302 to /lg/peers", when, addr, status, location)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // the test asserts that no handshake completes
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("%s: a TLS handshake with %s completed, want plain HTTP", when, addr)
	}
}

// lgFreeAddr returns a loopback address nothing listens on.
func lgFreeAddr(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("release the reserved port: %v", err)
	}
	return addr
}

// lgOneListener is the config of the one listener a migrator's looking glass
// runs, at the address it is bound to, with the given tls leaf and name.
func lgOneListener(lm *listenerMigrator, entryTLS zeconfig.LGServerTLS, certName string) lgListenerConfig {
	return lgListenerConfig{
		listeners: []lgListener{{addr: lm.lg.Addresses()[0], tls: entryTLS}},
		certName:  certName,
	}
}

func TestBuildLGServiceServesHTTPAndHTTPSAtOnce(t *testing.T) {
	// VALIDATES: one looking glass built from an entry that says `tls false` and
	// an entry that says `tls true` answers HTTP on the first and HTTPS on the
	// second at the same time, and refuses the RFC 8522 API on the HTTP one.
	// PREVENTS: one protocol for the whole looking glass, which is what the
	// block-wide tls leaf gave, and an RFC 8522 answer in the clear.
	loadLGPKIStore(t)

	svc, err := buildLGService(&serviceDeps{
		Dispatch: lgTestDispatch(),
		LGListeners: []lgListener{
			{addr: "127.0.0.1:0", tls: zeconfig.LGServerTLSOff},
			{addr: "127.0.0.1:0", tls: zeconfig.LGServerTLSRequired},
		},
		LGCertificate: "lg-cert",
		Store:         nil,
	})
	if err != nil {
		t.Fatalf("buildLGService: %v", err)
	}
	if svc == nil {
		t.Fatal("want a running looking glass, got nil")
	}
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })

	built, ok := svc.(lgService)
	if !ok {
		t.Fatalf("service = %T, want lgService", svc)
	}
	addrs := built.Addresses()
	if len(addrs) != 2 {
		t.Fatalf("bound %d listeners, want 2", len(addrs))
	}
	lgRequireHTTP(t, addrs[0], "the tls false entry")
	lgRequireHTTPS(t, addrs[1], "the tls true entry")

	// RFC 8522 Section 6: the API is refused over plain HTTP, per connection.
	if status, _ := lgPlainStatus(t, addrs[0], "/.well-known/looking-glass/v1/routers"); status != http.StatusBadRequest {
		t.Fatalf("RFC 8522 over the HTTP entry answered %d, want 400", status)
	}
}

func TestReloadDroppingLGCertificateNameWithoutStore(t *testing.T) {
	// VALIDATES: on a daemon with no store, a reload that drops the certificate
	// name does what a restart with that config does. An entry that left tls
	// unset moves to HTTP on the socket it already had, with a warning. An
	// entry whose operator wrote `tls true` refuses the reload and keeps its
	// chain.
	// PREVENTS: a reload refused over a default nobody wrote, a `tls true`
	// entry served in the clear, and a rebind to change protocol.
	loadLGPKIStore(t)

	t.Run("an unset tls leaf moves to HTTP without a rebind", func(t *testing.T) {
		lm := lgMigratorFor(t, &serviceDeps{
			Dispatch:      lgTestDispatch(),
			LGListeners:   lgTestListeners(zeconfig.LGServerTLSInherited),
			LGCertificate: "lg-cert",
			Store:         nil,
		})
		if lm.lgTransport == nil {
			t.Fatal("a running looking glass must hold the protocol and certificate handle")
		}
		addrs := lm.lg.Addresses()
		lgRequireHTTPS(t, addrs[0], "before the name is dropped")

		logged := captureMigratorLog(lm)
		cfg := lgOneListener(lm, zeconfig.LGServerTLSInherited, "")
		if err := lm.checkLGTransport(cfg); err != nil {
			t.Fatalf("the reload gate must accept an unset tls leaf: %v", err)
		}
		if err := lm.updateLGTransport(cfg); err != nil {
			t.Fatalf("dropping the name on an unset tls leaf must be accepted: %v", err)
		}

		lgRequireHTTP(t, addrs[0], "after the name is dropped")
		if got := lm.lg.Addresses(); !slices.Equal(got, addrs) {
			t.Fatalf("the looking glass moved from %v to %v: a protocol change must not rebind", addrs, got)
		}
		for _, want := range []string{"plain HTTP", addrs[0], "ze init", "tls false"} {
			if !strings.Contains(logged(), want) {
				t.Fatalf("the downgrade must be reported with its fix: %q missing from %q", want, logged())
			}
		}

		// Parity: a start with the reloaded config serves the same protocol.
		restarted, err := buildLGService(&serviceDeps{
			Dispatch:    lgTestDispatch(),
			LGListeners: lgTestListeners(zeconfig.LGServerTLSInherited),
			Store:       nil,
		})
		if err != nil {
			t.Fatalf("a start with the reloaded config: %v", err)
		}
		t.Cleanup(func() { _ = restarted.Shutdown(context.Background()) })
		restartedLG, ok := restarted.(lgService)
		if !ok {
			t.Fatalf("service = %T, want lgService", restarted)
		}
		lgRequireHTTP(t, restartedLG.Addresses()[0], "a start with the reloaded config")
	})

	t.Run("a written tls true refuses the reload and keeps its chain", func(t *testing.T) {
		lm := lgMigratorFor(t, &serviceDeps{
			Dispatch:      lgTestDispatch(),
			LGListeners:   lgTestListeners(zeconfig.LGServerTLSRequired),
			LGCertificate: "lg-cert",
			Store:         nil,
		})
		addr := lm.lg.Addresses()[0]
		cfg := lgOneListener(lm, zeconfig.LGServerTLSRequired, "")

		if err := lm.checkLGTransport(cfg); err == nil {
			t.Fatal("the reload gate must refuse `tls true` with nothing to present")
		}
		err := lm.updateLGTransport(cfg)
		if err == nil {
			t.Fatal("`tls true` with no name and no store must refuse the reload")
		}
		for _, want := range []string{"looking glass", addr, "tls true", "self-signed", "ze init", "tls false"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("the refusal must say what failed and how to fix it: %q missing from %q", want, err)
			}
		}
		lgRequireHTTPS(t, addr, "after the refused reload")

		// Parity: a start with the same config is refused with the same words.
		svc, startErr := buildLGService(&serviceDeps{
			Dispatch:    lgTestDispatch(),
			LGListeners: []lgListener{{addr: addr, tls: zeconfig.LGServerTLSRequired}},
			Store:       nil,
		})
		if startErr == nil {
			_ = svc.Shutdown(context.Background())
			t.Fatal("a start with the refused config must be refused too")
		}
		if startErr.Error() != err.Error() {
			t.Fatalf("start refuses with %q, reload with %q: one config, one answer", startErr, err)
		}
	})
}

func TestReloadNamingCertificateOnDowngradedLGServesHTTPS(t *testing.T) {
	// VALIDATES: a looking glass Ze started on HTTP, because its entry left tls
	// unset and there was no certificate to present, serves HTTPS on the same
	// socket once a reload names a certificate: what a restart with that config
	// serves. A name that does not resolve refuses the reload and changes
	// nothing.
	// PREVENTS: the accepted reload that took no effect until a restart, which
	// is what a looking glass with no rotation handle gave.
	loadLGPKIStore(t)

	lm := lgMigratorFor(t, &serviceDeps{
		Dispatch:    lgTestDispatch(),
		LGListeners: lgTestListeners(zeconfig.LGServerTLSInherited),
		Store:       nil,
	})
	if lm.lgTransport == nil {
		t.Fatal("a looking glass serving HTTP must hold the protocol and certificate handle too")
	}
	addrs := lm.lg.Addresses()
	lgRequireHTTP(t, addrs[0], "before a certificate is named")

	if err := lm.updateLGTransport(lgOneListener(lm, zeconfig.LGServerTLSInherited, "typo-cert")); err == nil {
		t.Fatal("a name the store does not hold must refuse the reload")
	}
	lgRequireHTTP(t, addrs[0], "after the refused reload")

	if err := lm.updateLGTransport(lgOneListener(lm, zeconfig.LGServerTLSInherited, "lg-cert")); err != nil {
		t.Fatalf("naming a certificate must be accepted: %v", err)
	}
	lgRequireHTTPS(t, addrs[0], "after a certificate is named")
	if chain := lgChainAt(t, addrs[0]); len(chain) != 2 {
		t.Fatalf("served chain length = %d, want 2 (leaf + intermediate)", len(chain))
	}
	if got := lm.lg.Addresses(); !slices.Equal(got, addrs) {
		t.Fatalf("the looking glass moved from %v to %v: a protocol change must not rebind", addrs, got)
	}
}

func TestReloadAddsHTTPEntry(t *testing.T) {
	// VALIDATES: a reload whose config adds a server entry with `tls false`
	// binds it and serves HTTP there, beside the HTTPS entry that was running,
	// through the real change set (buildChanges) and the real config reader.
	// The listener undo then removes it again.
	// PREVENTS: a new entry taking the protocol of the entries already bound.
	loadLGPKIStore(t)

	lm := lgMigratorFor(t, &serviceDeps{
		Dispatch:      lgTestDispatch(),
		LGListeners:   lgTestListeners(zeconfig.LGServerTLSRequired),
		LGCertificate: "lg-cert",
		Store:         nil,
	})
	secure := lm.lg.Addresses()[0]
	public := lgFreeAddr(t)

	entry := func(addr string, leaves map[string]any) map[string]any {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatalf("split %s: %v", addr, err)
		}
		out := map[string]any{"ip": host, "port": port}
		for leaf, value := range leaves {
			out[leaf] = value
		}
		return out
	}
	tree := lgEntriesTree(map[string]any{"enabled": "true", "certificate": "lg-cert"}, map[string]any{
		"secure": entry(secure, map[string]any{"tls": "true"}),
		"public": entry(public, map[string]any{"tls": "false"}),
	})

	// The reload's order: the protocols are recorded before any listener
	// moves, so the added entry is bound on HTTP from its first connection
	// (runReloadContext).
	cfg, err := lgListenerConfigFrom(tree)
	if err != nil {
		t.Fatalf("read the reloaded config: %v", err)
	}
	if err := lm.updateLGTransport(cfg); err != nil {
		t.Fatalf("apply the reloaded protocols: %v", err)
	}
	undo, err := lm.reloadListeners(context.Background(), tree)
	if err != nil {
		t.Fatalf("listener migration: %v", err)
	}

	lgRequireHTTP(t, public, "the added tls false entry")
	lgRequireHTTPS(t, secure, "the entry that was running")
	if got := lm.lg.Addresses(); !slices.Contains(got, public) || !slices.Contains(got, secure) {
		t.Fatalf("bound %v, want both %s and %s", got, secure, public)
	}

	if err := undo(); err != nil {
		t.Fatalf("listener undo: %v", err)
	}
	if got := lm.lg.Addresses(); !slices.Equal(got, []string{secure}) {
		t.Fatalf("after the undo the looking glass binds %v, want %v", got, []string{secure})
	}
	lgRequireHTTPS(t, secure, "after the undo")
}

func TestRollbackRestoresLGListenerProtocol(t *testing.T) {
	// VALIDATES: a reload that moved a listener to HTTP and then failed puts
	// the protocol and the certificate of the running config back on it,
	// through the undo the reload registers (restoreOn), with no rebind.
	// PREVENTS: a rolled-back daemon serving in the clear, under a config that
	// names a certificate, until its next restart.
	loadLGPKIStore(t)

	lm := lgMigratorFor(t, &serviceDeps{
		Dispatch:      lgTestDispatch(),
		LGListeners:   lgTestListeners(zeconfig.LGServerTLSInherited),
		LGCertificate: "lg-cert",
		Store:         nil,
	})
	addrs := lm.lg.Addresses()
	running := &listenerCertificateNames{lg: lgOneListener(lm, zeconfig.LGServerTLSInherited, "lg-cert")}

	if err := lm.updateLGTransport(lgOneListener(lm, zeconfig.LGServerTLSInherited, "")); err != nil {
		t.Fatalf("the reload that drops the name: %v", err)
	}
	lgRequireHTTP(t, addrs[0], "after the reload that is about to be rolled back")

	if err := running.restoreOn(lm, true)(); err != nil {
		t.Fatalf("restore the running config's protocol: %v", err)
	}
	lgRequireHTTPS(t, addrs[0], "after the rollback")
	if got := lm.lg.Addresses(); !slices.Equal(got, addrs) {
		t.Fatalf("the rollback moved the looking glass from %v to %v", addrs, got)
	}
}
