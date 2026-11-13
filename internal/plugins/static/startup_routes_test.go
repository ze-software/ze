// Design: docs/architecture/static-routes.md -- startup routes wait for all plugins
// Related: register.go -- registerStaticCallbacks, the production callback wiring

package static

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/rib/routeinstall"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: the real static configuration callbacks complete the five-stage
// SDK handshake without issuing a runtime route-install operation. All-ready
// then installs the configured blackholes with their tag presence intact.
// PREVENTS: external static exhausting its route-install retries during
// configure, closing its SDK and aborting the daemon's startup tier.
func TestStaticStartupRoutesWaitForAllPluginsReady(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn(pluginName, pluginEnd)
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	backend := &mockStaticBackend{}
	rm := newRouteManager(backend)
	rm.setLocRIB(nil, routeinstall.New(ctx, p))
	registerStaticCallbacks(p, rm)

	var runtime atomic.Bool
	var premature atomic.Uint32
	installed := make(chan rpc.RouteInstallEntry, 8)
	removed := make(chan rpc.RouteRemoveEntry, 8)
	declared := make(chan struct{})
	capabilities := make(chan struct{})
	ready := make(chan struct{})
	served := make(chan error, 1)
	run := make(chan error, 1)
	go func() { run <- p.Run(ctx, &sdk.Registration{}) }()
	go func() {
		for {
			select {
			case <-ctx.Done():
				served <- nil
				return
			case request, ok := <-mux.Requests():
				if !ok {
					served <- nil
					return
				}
				var err error
				switch request.Method {
				case rpc.MethodDeclareRegistration:
					err = mux.SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion})
					close(declared)
				case "ze-plugin-engine:declare-capabilities":
					err = mux.SendOK(ctx, request.ID)
					close(capabilities)
				case "ze-plugin-engine:ready":
					err = mux.SendOK(ctx, request.ID)
					close(ready)
				case "ze-plugin-engine:route-install":
					if !runtime.Load() {
						premature.Add(1)
						err = mux.SendError(ctx, request.ID, "route-install is unavailable during configure; call it after ready")
						break
					}
					var input rpc.RouteInstallInput
					if err = json.Unmarshal(request.Params, &input); err == nil {
						for _, entry := range input.Routes {
							installed <- entry
						}
						err = mux.SendResult(ctx, request.ID, &rpc.RouteInstallOutput{Installed: uint32(len(input.Routes))})
					}
				case "ze-plugin-engine:route-remove":
					var input rpc.RouteRemoveInput
					if err = json.Unmarshal(request.Params, &input); err == nil {
						for _, entry := range input.Routes {
							removed <- entry
						}
						err = mux.SendResult(ctx, request.ID, &rpc.RouteRemoveOutput{Removed: uint32(len(input.Routes))})
					}
				default:
					err = fmt.Errorf("unexpected static request %q", request.Method)
				}
				if err != nil {
					served <- err
					return
				}
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		_ = p.Close()
		_ = mux.Close()
		<-run
		<-served
		rm.shutdown()
	})

	wait := func(done <-chan struct{}, stage string) {
		t.Helper()
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatalf("static never completed %s: %v", stage, ctx.Err())
		}
	}
	call := func(method string, input any) {
		t.Helper()
		if _, err := mux.CallRPC(ctx, method, input); err != nil {
			t.Fatalf("%s failed (premature route installs: %d): %v", method, premature.Load(), err)
		}
	}

	wait(declared, "declaration")
	input := rpc.ConfigureInput{Sections: []sdk.ConfigSection{{Root: pluginName, Data: `{"static":{"table":{"default":{"route":{` +
		`"192.0.2.0/24":{"blackhole":{},"tag":"4242"},` +
		`"198.51.100.0/24":{"blackhole":{},"tag":"0"},` +
		`"203.0.113.0/24":{"blackhole":{}}}}}}}`,
	}}}
	call("ze-plugin-callback:configure", input)
	wait(capabilities, "capabilities")
	call("ze-plugin-callback:share-registry", nil)
	wait(ready, "ready")
	if got := premature.Load(); got != 0 {
		t.Fatalf("static issued %d route installs before runtime admission", got)
	}
	if got := len(rm.showRoutes()); got != 0 {
		t.Fatalf("static applied %d routes before all-ready", got)
	}
	runtime.Store(true)
	call("ze-plugin-callback:post-startup", nil)
	want := map[string]struct {
		tag    uint32
		hasTag bool
	}{
		"192.0.2.0/24":    {4242, true},
		"198.51.100.0/24": {0, true},
		"203.0.113.0/24":  {0, false},
	}
	for range 3 {
		select {
		case entry := <-installed:
			expected, ok := want[entry.Prefix]
			if !ok || entry.Tag != expected.tag || entry.HasTag != expected.hasTag ||
				entry.RouteType != uint8(routetype.Blackhole) || entry.Protocol != pluginName {
				t.Fatalf("wrong or duplicate startup route: %+v", entry)
			}
			delete(want, entry.Prefix)
		case <-ctx.Done():
			t.Fatalf("missing useful startup routes %v: %v", want, ctx.Err())
		}
	}
	if got := len(rm.showRoutes()); got != 3 {
		t.Fatalf("static holds %d routes after all-ready, want 3", got)
	}
	rm.shutdown()
	if len(removed) != 3 {
		t.Fatalf("shutdown withdrew %d routes, want 3", len(removed))
	}
}
