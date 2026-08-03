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

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/lg"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// lgService adapts *lg.LGServer to the Service interface (it already satisfies
// Reconfigurable + Shutdown; only Name is added).
type lgService struct {
	*lg.LGServer
}

func (lgService) Name() string { return "looking-glass" }

// buildLGService builds and starts the looking-glass HTTP server from deps. It
// returns a nil Service (not an error) when lg is not configured or fails to
// start -- preserving the prior best-effort, non-fatal behavior of
// startLGServer. Every entry in deps.LGAddrs becomes a bound listener; Shutdown
// closes all of them.
func buildLGService(deps ServiceDeps) (Service, error) {
	if len(deps.LGAddrs) == 0 || deps.Dispatch == nil {
		// Not configured: a skip, not a failure. buildServices treats a nil
		// service as "feature absent" and moves on.
		return nil, nil //nolint:nilnil // not-configured is an intentional skip
	}

	dispatch := deps.Dispatch
	resolvers := deps.Resolvers
	cfg := lg.LGConfig{
		ListenAddrs: deps.LGAddrs,
		TLS:         deps.LGTLS,
		// The unified dispatcher is passed through directly; the lg server
		// renders each typed response at its edge with a zero-value caller.
		Dispatch: dispatch,
		DecorateASN: func(asn string) string {
			if resolvers == nil || resolvers.Cymru == nil {
				return ""
			}
			name, _ := resolvers.Cymru.LookupASNName(context.Background(), parseASNForDecorator(asn))
			return name
		},
	}

	// When TLS is enabled, load or generate cert from blob storage. The SAN
	// hint is derived from the first endpoint; GenerateWebCertWithAddr already
	// fans out to all interface IPs when the host is 0.0.0.0. Build failures are
	// returned as errors; buildServices logs them and leaves lg unstarted
	// (best-effort, matching the prior non-fatal behavior).
	if deps.LGTLS {
		if !storage.IsBlobStorage(deps.Store) {
			return nil, errors.New("looking glass TLS requires blob storage (run ze init first)")
		}
		certStore := &blobCertStore{store: deps.Store}
		certPEM, keyPEM, err := selfcert.LoadOrGenerateCert(certStore, deps.LGAddrs[0])
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
	return lgService{srv}, nil
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

// parseASNForDecorator converts an ASN string to uint32 for the Cymru resolver.
// Returns 0 on parse failure (Cymru handles ASN 0 gracefully). Lives here (not
// always-on) because the looking glass is its only caller.
func parseASNForDecorator(asn string) uint32 {
	var n uint64
	for _, c := range asn {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + uint64(c-'0')
		if n > 4294967295 {
			return 0
		}
	}
	return uint32(n)
}
