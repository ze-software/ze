// Design: docs/architecture/api/process-protocol.md -- owned interface devices
// Related: owned_devices.go -- the engine operations and cleanup keyed on compiled registration identity

package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	containerIdentityOwner  = "container"
	containerIdentityPrefix = "zti"
)

var containerGrantOnce sync.Once

func ensureContainerIdentityGrant() {
	containerGrantOnce.Do(func() {
		if iface.RefuseUngrantedDeviceOwner(containerIdentityOwner) != nil {
			iface.MustGrantOwnedDeviceNames(containerIdentityOwner, containerIdentityPrefix)
		}
	})
}

// TestOwnedDevicesAuthorizedByCompiledIdentity proves that the owned interface
// device handlers authorize callers by compiled registration identity
// (proc.RegistrationName()), not the operator's configured endpoint alias
// (proc.Name()).
//
// VALIDATES: Q-T2 compile-time grants for owned interface devices — an aliased
// compiled "container" plugin (in-daemon and separate-process) passes the grant
// check and registers its caller under "container", while a foreign external
// program named "container" is refused on all three handlers.
// PREVENTS: an aliased compiled container plugin losing its device grant, or a
// foreign external program named "container" creating bridges or links.
func TestOwnedDevicesAuthorizedByCompiledIdentity(t *testing.T) {
	ensureContainerIdentityGrant()
	if err := iface.CloseBackend(); err != nil {
		t.Fatalf("reset backend: %v", err)
	}

	bridgeParams, err := json.Marshal(rpc.OwnedBridgesReplaceInput{})
	if err != nil {
		t.Fatalf("marshal bridge input: %v", err)
	}
	removeParams, err := json.Marshal(rpc.OwnedLinkRemoveInput{Key: "wire"})
	if err != nil {
		t.Fatalf("marshal remove input: %v", err)
	}
	createParams, err := json.Marshal(rpc.OwnedLinkCreateInput{
		Key: "wire", Bridge: containerIdentityPrefix + "-net", Slot: 1,
		PID: 1, NamespaceInode: 1, Name: "eth0", MTU: 1500,
	})
	if err != nil {
		t.Fatalf("marshal create input: %v", err)
	}

	for _, tc := range []struct {
		name string
		cfg  plugin.PluginConfig
	}{
		{
			name: "aliased in-daemon compiled container",
			cfg: plugin.PluginConfig{
				Name:           "my-container",
				CompiledPlugin: containerIdentityOwner,
				Internal:       true,
			},
		},
		{
			name: "aliased separate-process compiled container",
			cfg: plugin.PluginConfig{
				Name:            "my-container-fork",
				CompiledPlugin:  containerIdentityOwner,
				SeparateProcess: true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proc := process.NewProcess(tc.cfg)
			t.Cleanup(func() { ownedDevicesCleanup(nil, proc) })

			_, err := ownedBridgesReplaceHandler(nil, t.Context(), proc, bridgeParams)
			if err == nil {
				t.Fatal("expected backend-absent error with no interface backend loaded")
			}
			if strings.Contains(err.Error(), "may not ask for owned interface devices") {
				t.Fatalf("aliased compiled container was refused by owner grant: %v", err)
			}

			ownedDeviceCallers.mu.Lock()
			recorded := ownedDeviceCallers.byOwner[containerIdentityOwner]
			ownedDeviceCallers.mu.Unlock()
			if recorded != proc {
				t.Fatalf("ownedDeviceCallers[%q] = %p, want %p", containerIdentityOwner, recorded, proc)
			}

			ownedDevicesCleanup(nil, proc)
			ownedDeviceCallers.mu.Lock()
			_, stillKnown := ownedDeviceCallers.byOwner[containerIdentityOwner]
			ownedDeviceCallers.mu.Unlock()
			if stillKnown {
				t.Fatalf("ownedDevicesCleanup left %q recorded in ownedDeviceCallers", containerIdentityOwner)
			}
		})
	}

	t.Run("foreign external named container is refused on every handler", func(t *testing.T) {
		foreign := process.NewProcess(plugin.PluginConfig{
			Name: containerIdentityOwner,
			Run:  "/usr/bin/fake",
		})
		for _, call := range []struct {
			name string
			run  func() (any, error)
		}{
			{"replace bridges", func() (any, error) { return ownedBridgesReplaceHandler(nil, t.Context(), foreign, bridgeParams) }},
			{"create link", func() (any, error) { return ownedLinkCreateHandler(nil, t.Context(), foreign, createParams) }},
			{"remove link", func() (any, error) { return ownedLinkRemoveHandler(nil, t.Context(), foreign, removeParams) }},
		} {
			_, err := call.run()
			if err == nil {
				t.Fatalf("%s: foreign external named %q was allowed", call.name, containerIdentityOwner)
			}
			var failure *opererr.Error
			if !errors.As(err, &failure) {
				t.Fatalf("%s: refusal is not an operator error: %T %v", call.name, err, err)
			}
			if !strings.Contains(failure.Problem(), "Plugin "+containerIdentityOwner+" may not ask for owned interface devices") {
				t.Fatalf("%s: unexpected refusal problem: %q", call.name, failure.Problem())
			}
		}
	})
}
