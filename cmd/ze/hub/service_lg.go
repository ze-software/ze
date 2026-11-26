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
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/lg"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// lgService adapts *lg.LGServer to the Service interface (it already satisfies
// Reconfigurable + Shutdown; Name is added), and carries what a reload needs to
// give a TLS listener its self-signed pair (selfSignedSource).
type lgService struct {
	*lg.LGServer
	// certStore is the store the self-signed pair lives in. Nil when the daemon
	// runs without one: a listener then serves TLS on a named certificate only,
	// and planLGTransports decides what each entry does with no name.
	certStore selfcert.CertStore
	// sanHint is the first address the looking glass was built with.
	sanHint string
}

func (lgService) Name() string { return "looking-glass" }

// The reload migrator installs the certificate a reloaded config describes,
// and the protocol of each listener, through this seam (listener_migrate.go,
// updateLGTransport).
var _ lgTransportUpdatable = lgService{}

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

// buildLGService is the looking-glass factory: startLGService reporting to the
// daemon's log and terminal.
func buildLGService(deps *serviceDeps) (Service, error) {
	return startLGService(deps, slogutil.Logger("lg.server"), os.Stderr)
}

// startLGService builds and starts the looking-glass HTTP server from deps. It
// never stops ze: a nil Service with no error is a looking glass that is not
// configured, and an error is a looking glass that could not start, which
// buildServices logs before it moves on. Every servable entry in
// deps.LGListeners becomes a bound listener, on HTTPS or HTTP as
// planLGEndpoints decides for it; Shutdown closes all of them.
//
// An entry that cannot be served stays down and is reported on log and console
// under the code `ze doctor` reports it with (reportLGServerDown), and the
// other entries serve. When no entry can be served the looking glass does not
// start, and the error joins the operator error of each entry.
func startLGService(deps *serviceDeps, log *slog.Logger, console io.Writer) (Service, error) {
	// Not configured: a skip, not a failure. buildServices treats a nil service
	// as "feature absent" and moves on.
	if len(deps.LGListeners) == 0 {
		return nil, nil //nolint:nilnil // not-configured is an intentional skip
	}
	if deps.Dispatch == nil {
		return nil, nil //nolint:nilnil // no dispatcher to serve from is the same skip
	}

	// A typed nil store would read as a store, so certStore stays an untyped
	// nil when the daemon has none (listenerTLSMaterial refuses on nil).
	var certStore selfcert.CertStore
	if deps.Store != nil {
		certStore = &blobCertStore{store: deps.Store}
	}

	// Each entry chooses HTTPS or HTTP. A daemon without a store cannot keep a
	// self-signed certificate, so with no named one an entry that left tls
	// unset serves HTTP with the warning below, and an entry whose operator
	// wrote `tls true` stays down. A named certificate that cannot be presented
	// leaves every entry that asks for TLS down. A reload decides with the same
	// function, and refuses what a start leaves down (listener_migrate.go,
	// checkLGTransport).
	plan := planLGEndpoints(deps.LGListeners, deps.LGCertificate, lgCertificateProblem(deps.LGCertificate), certStore != nil)
	var down []error
	for _, unservable := range plan.unservable {
		reportLGServerDown(log, console, unservable)
		down = append(down, unservable.problem)
	}
	for _, addr := range plan.downgraded {
		fmt.Fprintln(console, "warning: "+lgDowngradeWarning(addr).Error()) //nolint:errcheck // a terminal notice; the log line carries the same report
	}
	if len(plan.bindings) == 0 {
		return nil, errors.Join(down...)
	}

	listen := make([]lg.ListenAddr, 0, len(plan.bindings))
	for _, binding := range plan.bindings {
		listen = append(listen, lg.ListenAddr{Addr: binding.addr, TLS: binding.tls})
	}
	sanHint := plan.bindings[0].addr

	cfg := lg.LGConfig{
		ListenAddrs: listen,
		Token:       deps.LGToken,
		// The names an operator gave to next-hop ranges. A reload replaces
		// them through lgService.UpdateNextHopRanges (listener_migrate.go).
		NextHopRanges: deps.LGNextHopRanges,
		// The unified dispatcher is passed through directly; the lg server
		// renders each typed response at its edge with a zero-value caller.
		// It also names AS numbers, by asking the resolve component's
		// `resolve cymru asn-name` command (internal/component/lg/as_names.go),
		// so the looking glass holds no resolver of its own.
		Dispatch: deps.Dispatch,
	}

	// When a listener serves TLS, resolve the material it presents through the
	// selector the web listener shares. Build failures are returned as errors;
	// buildServices logs them and leaves lg unstarted, and ze keeps running.
	if plan.servesTLS() {
		certPEM, keyPEM, err := listenerTLSMaterial(deps.LGCertificate, certStore, sanHint)
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

	// Addresses answers the bound addresses in the order of the plan.
	for i, addr := range srv.Addresses() {
		scheme := "http"
		if plan.bindings[i].tls {
			scheme = "https"
		}
		fmt.Fprintf(console, "looking glass listening on %s://%s/\n", scheme, addr) //nolint:errcheck // a terminal notice; nothing acts on its delivery
	}
	return lgService{LGServer: srv, certStore: certStore, sanHint: sanHint}, nil
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
