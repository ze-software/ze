// Design: docs/architecture/traffic/cos-dynamic.md -- operator cleanup recovery
// Related: ../../test/pluginengine/pluginengine.go -- registered SDK callback route

//go:build ze_l2tp

package cos

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/plugin/server"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/ze"
)

const cleanupCommandForReview = "request class-of-service cleanup"

// The runner, SDK callback loop and event subscriptions are real. Only the
// existing native read/write boundary uses the recorded backend. No future
// cleanup implementation symbol is needed to establish the missing route.
func newRegisteredCoSCleanupReview(t *testing.T) (*sharedCoSTestOwner, *pluginengine.Engine, ze.EventBus) {
	t.Helper()
	f := newSharedCoSTestOwner(t)
	write := f.handler.updateQoS
	read := f.handler.readLink
	priorHandler := dynamicHandler
	t.Cleanup(func() { dynamicHandler = priorHandler })
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	registered := registry.Lookup(Name)
	if registered == nil {
		t.Fatal("the class-of-service plugin is not registered")
	}
	if registered.ConfigureEventBus == nil {
		t.Fatal("the registered class-of-service owner has no event binding")
	}
	if registered.RunEngine == nil {
		t.Fatal("the registered class-of-service owner has no internal runner")
	}
	registered.ConfigureEventBus(bus)
	if dynamicHandler == nil {
		t.Fatal("the registered event binding did not publish its owner")
	}
	dynamicHandler.updateQoS = func(name string, ingress, egress map[uint32]uint32) error {
		if name != sharedCoSTestInterface {
			return fmt.Errorf("cleanup attempted a different native access: %s", name)
		}
		return write(name, ingress, egress)
	}
	dynamicHandler.readLink = func(name string) (*iface.InterfaceInfo, error) {
		if name != sharedCoSTestInterface {
			return nil, fmt.Errorf("cleanup observed a different native access: %s", name)
		}
		return read(name)
	}
	dynamicHandler.captureTarget = captureModeledOriginalCoSTarget(dynamicHandler)
	f.handler = dynamicHandler
	engine := pluginengine.Start(t, pluginengine.TransportBridge, registered.RunEngine)
	return f, engine, bus
}

func registeredCoSCleanupReviewUp(t *testing.T, bus ze.EventBus, session uint16) {
	t.Helper()
	storeMetadataForTest(sharedCoSTestTunnel, session, sharedCoSTestFirst)
	t.Cleanup(func() {
		clearMetadataForTest(sharedCoSTestTunnel, session)
		sessionStore.Delete(fixtureCoSKey(sharedCoSTestTunnel, session))
	})
	if _, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		Birth: fixtureCoSKey(sharedCoSTestTunnel, session).birth, SubscriberID: fixtureCoSID(sharedCoSTestTunnel, session),
		TunnelID: sharedCoSTestTunnel, SessionID: session, AccessInterface: sharedCoSTestInterface,
	}); err != nil {
		t.Fatal(err)
	}
}

func registeredCoSCleanupReviewDown(t *testing.T, bus ze.EventBus, session uint16) {
	t.Helper()
	if _, err := l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{Birth: fixtureCoSKey(sharedCoSTestTunnel, session).birth, SubscriberID: fixtureCoSID(sharedCoSTestTunnel, session),
		TunnelID: sharedCoSTestTunnel, SessionID: session,
	}); err != nil {
		t.Fatal(err)
	}
}

func executeCoSCleanupForReview(t *testing.T, engine *pluginengine.Engine, args []string) error {
	t.Helper()
	// This bounds the test's observation only. It is not evidence that a
	// native backend operation can be canceled by an RPC caller timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := engine.Execute(ctx, cleanupCommandForReview, args)
	return err
}

// The operator cleanup path must be declared once and offered on daemon
// surfaces, while the existing subscriber transaction stays separate.
func TestCoSLegacySharedCleanupCommandIsVisible(t *testing.T) {
	registered := registry.Lookup(Name)
	if registered == nil {
		t.Fatal("the class-of-service plugin is not registered")
	}
	found := false
	for _, declaration := range registered.Commands {
		if declaration.Name != cleanupCommandForReview {
			continue
		}
		if found {
			t.Fatal("the operator cleanup command is declared more than once")
		}
		found = true
		if declaration.Hidden {
			t.Fatal("the operator cleanup command is hidden")
		}
		if len(declaration.Args) != 0 {
			t.Fatalf("cleanup command arguments=%v, want none", declaration.Args)
		}
		listed, err := command.ListedOnDaemonSurfaces(command.CommandRegistration{
			Path: declaration.Name, Kind: command.KindPlugin, Hidden: declaration.Hidden,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !listed {
			t.Fatal("the operator cleanup command is absent from daemon surfaces")
		}
	}
	if !found {
		t.Fatal("the registered owner does not declare the operator cleanup command")
	}
}

// A real command callback must retry an original failed final restoration,
// retain its error until a successful pass, and make an empty pass a no-op.
func TestCoSLegacySharedCleanupCommandRetriesFailedFinalRestore(t *testing.T) {
	f, engine, bus := newRegisteredCoSCleanupReview(t)
	registeredCoSCleanupReviewUp(t, bus, 491)
	f.requireMaps(f.shared)
	f.failWrites = 1
	registeredCoSCleanupReviewDown(t, bus, 491)
	writes := f.writes
	f.failWrites = 1
	err := executeCoSCleanupForReview(t, engine, nil)
	if err == nil {
		t.Fatal("cleanup acknowledged an unconfirmed native restoration")
	}
	if !strings.Contains(err.Error(), "native write outcome is not confirmed") {
		t.Fatalf("cleanup lost the actual retained native cause: %v", err)
	}
	if f.writes != writes+1 {
		t.Fatal("the failed cleanup pass did not retry the original native restore exactly once")
	}
	if _, retained := sessionStore.Load(fixtureCoSKey(sharedCoSTestTunnel, 491)); !retained {
		t.Fatal("the failed cleanup command lost original session custody")
	}
	if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
		t.Fatalf("the healthy cleanup retry failed: %v", err)
	}
	f.requireMaps(f.original)
	if _, retained := sessionStore.Load(fixtureCoSKey(sharedCoSTestTunnel, 491)); retained {
		t.Fatal("confirmed cleanup retained the departed session")
	}
	writes = f.writes
	if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
		t.Fatal(err)
	}
	if f.writes != writes {
		t.Fatal("an empty cleanup pass wrote native maps")
	}
}

// An initial write can fail before a session observation is published. The
// command must still reach its original compensation instead of forgetting it.
func TestCoSLegacySharedCleanupCommandFindsUnpublishedCompensation(t *testing.T) {
	f, engine, bus := newRegisteredCoSCleanupReview(t)
	f.failWrites = 2
	registeredCoSCleanupReviewUp(t, bus, 492)
	f.failWrites = 0
	writes := f.writes
	if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
		t.Fatalf("the unpublished original compensation could not recover: %v", err)
	}
	if f.writes != writes+1 {
		t.Fatal("cleanup did not confirm the unpublished original compensation exactly once")
	}
	f.requireMaps(f.original)
	registeredCoSCleanupReviewUp(t, bus, 492)
	f.requireMaps(f.shared)
	registeredCoSCleanupReviewDown(t, bus, 492)
	f.requireMaps(f.original)
}

// Cleanup must leave a live pending or committed transaction unchanged. The
// original token must remain usable for its ordinary compensation afterward.
func TestCoSLegacySharedCleanupCommandPreservesLiveTransactions(t *testing.T) {
	for _, committed := range []bool{false, true} {
		name := "pending"
		if committed {
			name = "committed"
		}
		t.Run(name, func(t *testing.T) {
			f, engine, bus := newRegisteredCoSCleanupReview(t)
			registeredCoSCleanupReviewUp(t, bus, 493)
			token, err := f.command("prepare", 493, sharedCoSTestDifferent, "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.command("apply", 0, "", token); err != nil {
				t.Fatal(err)
			}
			if committed {
				if _, err := f.command("commit", 0, "", token); err != nil {
					t.Fatal(err)
				}
			}
			different, found := coreCos.Lookup(sharedCoSTestDifferent)
			if !found {
				t.Fatal("the healthy transaction profile disappeared")
			}
			f.requireMaps(different)
			writes := f.writes
			if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
				t.Fatal(err)
			}
			if f.writes != writes {
				t.Fatal("cleanup changed a live transaction's native maps")
			}
			f.requireMaps(different)
			if _, err := f.command("rollback", 0, "", token); err != nil {
				t.Fatalf("cleanup invalidated the live original token: %v", err)
			}
			f.requireMaps(f.shared)
			registeredCoSCleanupReviewDown(t, bus, 493)
		})
	}
}
