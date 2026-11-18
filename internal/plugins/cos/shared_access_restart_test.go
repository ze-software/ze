// Design: docs/architecture/traffic/cos-dynamic.md -- original map owner handoff
// Related: ../../test/pluginengine/pluginengine.go -- real SDK lifecycle

//go:build ze_l2tp

package cos

import (
	"testing"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/plugin/server"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/test/pluginengine"
)

// A registered internal runner must preserve the original map owner through
// stop and replacement. Both generations finish the real SDK startup, and each
// child scope closes and joins that runner before the next observation. The
// native maps start empty so the old static-snapshot defect cannot hide this
// independent lifecycle regression. Backend calls alone are modeled.
func TestCoSLegacySharedRegisteredRestartPreservesOriginalOwner(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.original = coreCos.Profile{}
	f.installed = coreCos.Profile{}
	priorHandler := dynamicHandler
	t.Cleanup(func() { dynamicHandler = priorHandler })
	const session = uint16(481)
	key := fixtureCoSKey(sharedCoSTestTunnel, session)
	storeMetadataForTest(sharedCoSTestTunnel, session, sharedCoSTestFirst)
	t.Cleanup(func() {
		clearMetadataForTest(sharedCoSTestTunnel, session)
		sessionStore.Delete(key)
	})
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	registration := registry.Lookup(Name)
	if registration == nil {
		t.Fatal("the class-of-service plugin is not registered")
	}
	if registration.ConfigureEventBus == nil {
		t.Fatal("the registered class-of-service owner has no event binding")
	}
	if registration.RunEngine == nil {
		t.Fatal("the registered class-of-service owner has no internal runner")
	}
	start := func(t *testing.T) {
		t.Helper()
		registration.ConfigureEventBus(bus)
		if dynamicHandler == nil {
			t.Fatal("the registered event binding did not publish its owner")
		}
		// Keep the registered owner and its actual lifecycle. Replace only
		// the existing native boundary with this test's recorded backend.
		dynamicHandler.updateQoS = f.handler.updateQoS
		dynamicHandler.readLink = f.handler.readLink
		dynamicHandler.captureTarget = captureModeledOriginalCoSTarget(dynamicHandler)
		pluginengine.Start(t, pluginengine.TransportBridge, registration.RunEngine)
	}
	if !t.Run("original runner", func(t *testing.T) {
		start(t)
		if _, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
			Birth: fixtureCoSKey(sharedCoSTestTunnel, session).birth, SubscriberID: fixtureCoSID(sharedCoSTestTunnel, session),
			TunnelID: sharedCoSTestTunnel, SessionID: session, AccessInterface: sharedCoSTestInterface,
		}); err != nil {
			t.Fatal(err)
		}
		if f.writes != 1 {
			t.Fatalf("the original registered owner made %d writes, want one install", f.writes)
		}
	}) {
		return
	}
	f.requireMaps(f.shared)
	if f.writes != 1 {
		t.Fatal("stopping the policy owner changed a live subscriber's maps")
	}
	// This is the actual server bus, not testBus's no-op unsubscriber. A
	// stopped generation must not accept a later policy notification.
	if _, err := l2tpevents.SessionCoSChange.Emit(bus, &l2tpevents.SessionCoSChangePayload{Birth: fixtureCoSKey(sharedCoSTestTunnel, session).birth, SubscriberID: fixtureCoSID(sharedCoSTestTunnel, session),
		TunnelID: sharedCoSTestTunnel, SessionID: session,
		AccessInterface: sharedCoSTestInterface, ProfileName: sharedCoSTestDifferent,
	}); err != nil {
		t.Fatal(err)
	}
	if f.writes != 1 {
		t.Fatal("the stopped registered generation still changed the native maps")
	}
	f.requireMaps(f.shared)
	if !t.Run("replacement runner", func(t *testing.T) {
		start(t)
		if _, err := l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{Birth: fixtureCoSKey(sharedCoSTestTunnel, session).birth, SubscriberID: fixtureCoSID(sharedCoSTestTunnel, session),
			TunnelID: sharedCoSTestTunnel, SessionID: session,
		}); err != nil {
			t.Fatal(err)
		}
		if f.writes != 2 {
			t.Fatalf("the replacement made %d total writes, want install and original restore", f.writes)
		}
		if _, retained := sessionStore.Load(key); retained {
			t.Fatal("the replacement left the departed original user retained")
		}
	}) {
		return
	}
	f.requireMaps(f.original)
}
