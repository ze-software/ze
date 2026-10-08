package yang_test

import (
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/core/ipc/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPluginIPCMethodsKeepTheirSpelling proves every rpc of the plugin IPC
// protocol declares its wire method with ze:method, spelled exactly as the
// module name and the rpc name joined by a colon, which is the spelling the
// engine and the SDK send and match.
//
// VALIDATES: phase 3 of spec-rpc-published-name-does-not-reach-its-handler:
// the 28 IPC rpcs, which no command node reaches, carry an explicit method,
// and AC-7: the stage 1 method an external plugin sends is the declared one.
// PREVENTS: an IPC rpc published under no name, or under a method that is not
// the one on the wire.
func TestPluginIPCMethodsKeepTheirSpelling(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load YANG: %v", err)
	}

	declared := map[string]string{} // "module:rpc" -> ze:method
	for _, module := range []string{"ze-plugin-engine", "ze-plugin-callback"} {
		rpcs := configyang.ExtractRPCs(loader, module)
		if len(rpcs) == 0 {
			t.Fatalf("%s declares no rpc, so this test proves nothing about it", module)
		}
		for _, meta := range rpcs {
			want := module + ":" + meta.Name
			if meta.WireMethod != want {
				t.Errorf("rpc %s in %s declares method %q, want %q", meta.Name, module, meta.WireMethod, want)
			}
			declared[want] = meta.WireMethod
		}
	}

	// The senders spell these as Go constants; each must be a declared method.
	for _, sent := range []string{
		rpc.MethodDeclareRegistration,
		rpc.MethodUpdateRoute,
		rpc.MethodDispatchCommand,
		rpc.MethodSubscribeEvents,
		rpc.MethodUnsubscribeEvents,
		rpc.MethodEmitEvent,
		rpc.MethodRouteMetrics,
	} {
		if declared[sent] != sent {
			t.Errorf("the engine sends %q, which no IPC rpc declares", sent)
		}
	}

	pub, err := configyang.PublishedRPCs(loader)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	protocol := map[string]bool{}
	for _, meta := range pub.Protocol {
		protocol[meta.WireMethod] = true
	}
	for method := range declared {
		if !protocol[method] {
			t.Errorf("%s is not published as a protocol method", method)
		}
	}
}
