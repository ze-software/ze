// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
//
// Looking-glass (lg) service factory: the feature-gate pilot. This file (with
// register_lg.go) is the ONLY place always-on-buildable code reaches the
// internal/component/lg server package, and it is compiled only under
// //go:build ze_lg. With ze_lg off the factory is not registered, the hub
// builds no lg service, and the lg package is linked nowhere -- so it is
// dropped from the binary.
//
// The lg YANG schema is gated separately by the generator (all_ze_lg.go).
// See plan/spec-feature-gate-1-lg.md.

//go:build ze_lg

package hub

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/lg"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// lgService adapts *lg.LGServer to the Service interface (it already satisfies
// Reconfigurable + Shutdown; Name is added), and carries what a reload needs to
// return the listener to its self-signed pair (selfSignedSource).
type lgService struct {
	*lg.LGServer
	// certStore is the store the self-signed pair lives in. Nil when the daemon
	// runs without one: a looking glass that started on a named certificate
	// needs none, and a reload that drops the name is then refused.
	certStore selfcert.CertStore
	// sanHint is the first address the looking glass was built with.
	sanHint string
}

func (lgService) Name() string { return "looking-glass" }

// The reload migrator installs the certificate a reloaded config describes
// through this seam (listener_migrate.go, updateLGCertificate).
var _ tlsUpdatable = lgService{}

// selfSignedSource answers the store and the SAN hint the looking glass's
// self-signed pair comes from (tlsUpdatable). The hint names a SAN only when
// the store holds no pair yet.
func (s lgService) selfSignedSource() (certStore selfcert.CertStore, listenAddr string) {
	return s.certStore, s.sanHint
}

// The reload migrator replaces the named next-hop ranges through this seam. A
// change to the method on *lg.LGServer that broke it would otherwise leave
// the registration hook (register_lg.go) as the only place that noticed.
var _ nextHopRangesUpdatable = lgService{}

// buildLGService builds and starts the looking-glass HTTP server from deps. It
// returns a nil Service (not an error) when lg is not configured or fails to
// start -- preserving the prior best-effort, non-fatal behavior of
// startLGServer. Every entry in deps.LGAddrs becomes a bound listener; Shutdown
// closes all of them.
func buildLGService(deps *serviceDeps) (Service, error) {
	if len(deps.LGAddrs) == 0 || deps.Dispatch == nil {
		// Not configured: a skip, not a failure. buildServices treats a nil
		// service as "feature absent" and moves on.
		return nil, nil //nolint:nilnil // not-configured is an intentional skip
	}

	dispatch := deps.Dispatch

	// A storeless stdin daemon cannot persist a self-signed looking-glass
	// certificate. Named certificates remain usable without persistence.
	useTLS := deps.LGTLS
	selfSignedTLS := useTLS && deps.LGCertificate == ""
	if selfSignedTLS && deps.Store == nil {
		if deps.LGTLSExplicit {
			return nil, errors.New("looking glass TLS requires persistent storage (run ze init first)")
		}
		fmt.Fprintln(os.Stderr,
			"warning: looking glass serving plaintext: TLS is on by default but needs persistent storage for certificates")
		fmt.Fprintln(os.Stderr,
			"  run `ze init` to enable TLS, or set looking-glass tls false to silence this warning")
		useTLS = false
	}

	cfg := lg.LGConfig{
		ListenAddrs: deps.LGAddrs,
		TLS:         useTLS,
		Token:       deps.LGToken,
		// The names an operator gave to next-hop ranges. A reload replaces
		// them through lgService.UpdateNextHopRanges (listener_migrate.go).
		NextHopRanges: deps.LGNextHopRanges,
		// The unified dispatcher is passed through directly; the lg server
		// renders each typed response at its edge with a zero-value caller.
		// It also names AS numbers, by asking the resolve component's
		// `resolve cymru asn-name` command (internal/component/lg/as_names.go),
		// so the looking glass holds no resolver of its own.
		Dispatch: dispatch,
	}

	// When TLS is enabled, resolve the material the listener serves through the
	// selector the web listener shares. Build failures are returned as errors;
	// buildServices logs them and leaves lg unstarted (best-effort, matching the
	// prior non-fatal behavior).
	if useTLS {
		certStore := &blobCertStore{store: deps.Store}
		certPEM, keyPEM, err := listenerTLSMaterial(deps.LGCertificate, certStore, deps.LGAddrs[0])
		if err != nil {
			return nil, fmt.Errorf("looking glass TLS cert: %w", err)
		}
		cfg.CertPEM = certPEM
		cfg.KeyPEM = keyPEM
	}

	srv, err := lg.NewLGServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("looking glass: %w", err)
	}

	// Component startup goroutine (one-time, same pattern as startWebServer).
	serveLG(srv)

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	if waitErr := srv.WaitReady(readyCtx); waitErr != nil {
		_ = srv.Shutdown(context.Background())
		return nil, fmt.Errorf("looking glass failed to start: %w", waitErr)
	}

	scheme := "http"
	if cfg.TLS {
		scheme = "https"
	}
	for _, addr := range srv.Addresses() {
		fmt.Fprintf(os.Stderr, "looking glass listening on %s://%s/\n", scheme, addr)
	}
	service := lgService{LGServer: srv, sanHint: deps.LGAddrs[0]}
	// A typed nil store would read as a store, so the field stays an untyped
	// nil when the daemon has none (listenerTLSMaterial refuses on nil).
	if deps.Store != nil {
		service.certStore = &blobCertStore{store: deps.Store}
	}
	return service, nil
}

// serveLG runs the LG server's ListenAndServe in a background goroutine.
// This is a one-time component startup, not a per-event goroutine.
func serveLG(srv *lg.LGServer) {
	go serveLGBlocking(srv)
}

func serveLGBlocking(srv *lg.LGServer) {
	if serveErr := srv.ListenAndServe(context.Background()); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		slogutil.Logger("lg.server").Error("looking glass server error", "error", serveErr)
	}
}
