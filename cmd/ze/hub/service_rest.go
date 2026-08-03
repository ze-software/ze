// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
// Related: api_infra.go -- the always-on seam (restBuild hook / apiShared) this installs
//
// ze_rest-gated REST/HTTP API server construction. This file (with
// register_rest.go) is the ONLY place always-on-buildable code reaches
// internal/component/api/rest, and it is compiled only under //go:build ze_rest.
// With ze_rest off the hook is nil, no REST server is built, and the rest server
// package + its YANG schema (api/rest/yang) are linked nowhere -- so the linker
// drops them and a rest{} config block is rejected as unknown.
//
// The shared engine/session manager is built once, always-on (buildAPIShared in
// api.go); this file only constructs the REST server from it. See
// plan/spec-feature-gate-6-api.md.

//go:build ze_rest

package hub

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/api/rest"
)

// restBuildImpl is the ze_rest implementation of the restBuild seam hook. It
// builds and starts the REST server from the resolved config and the shared
// engine/sessions, returning it as a Reconfigurable handle (zero handle when
// REST is not enabled in config).
func restBuildImpl(in *apiBuildInputs, sh *apiShared) (apiServerHandle, error) {
	cfg := in.Config
	if !cfg.RESTOn || len(cfg.REST) == 0 {
		return apiServerHandle{}, nil
	}

	addrs := make([]string, 0, len(cfg.REST))
	for _, ep := range cfg.REST {
		addrs = append(addrs, ep.Listen())
	}

	// Generate the OpenAPI spec lazily so it captures all plugin commands
	// (plugins may still be registering during startup). REST-specific.
	var (
		specOnce sync.Once
		specData []byte
	)
	lazySpec := func() []byte {
		specOnce.Do(func() {
			cmds := sh.Engine.ListCommands(&api.ListCommandsRequest{})
			var err error
			specData, err = api.OpenAPISchema(cmds)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: API OpenAPI generation failed: %v\n", err)
				specData = []byte(`{"openapi":"3.1.0","info":{"title":"Ze API","version":"1.0.0"},"paths":{}}`)
			}
		})
		return specData
	}

	srv, err := rest.NewRESTServer(rest.RESTConfig{
		ListenAddrs:   addrs,
		Token:         cfg.Token,
		Authenticator: sh.Authenticator,
		Authorizer:    in.Authorizer,
		CORSOrigin:    cfg.RESTCORSOrigin,
		AuditRecorder: in.Recorder,
	}, sh.Engine, sh.Sessions, lazySpec)
	if err != nil {
		return apiServerHandle{}, fmt.Errorf("create REST API: %w", err)
	}
	errCh, startErr := srv.Start(in.Server.Context())
	if startErr != nil {
		return apiServerHandle{}, fmt.Errorf("start REST API: %w", startErr)
	}
	go logRESTServerErrors(errCh)
	for _, addr := range srv.Addresses() {
		fmt.Fprintf(os.Stderr, "REST API server starting on http://%s/\n", addr)
	}
	return apiServerHandle{
		Server: srv,
		Shutdown: func(ctx context.Context) {
			if shErr := srv.Shutdown(ctx); shErr != nil {
				fmt.Fprintf(os.Stderr, "warning: REST API shutdown: %v\n", shErr)
			}
		},
	}, nil
}

// logRESTServerErrors logs runtime serving failures after startup already
// bound every requested REST listener successfully.
func logRESTServerErrors(errCh <-chan error) {
	for err := range errCh {
		fmt.Fprintf(os.Stderr, "warning: REST API server: %v\n", err)
	}
}
