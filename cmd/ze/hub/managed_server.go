// Design: docs/architecture/fleet-config.md -- hub managed-config server startup wiring

package hub

import (
	"context"
	"maps"

	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/slogutil"
)

var managedServerLog = slogutil.LazyLogger("hub.managed-server")

// startManagedServer starts the dedicated managed-config server when this hub declares
// one or more server blocks with client entries and storage is blob-backed. It returns
// nil when managed config is not configured (or fails to start, which is logged at
// Error). The server:
//   - authenticates each managed client with its per-client secret,
//   - answers config-fetch/config-ack/ping,
//   - reads each client's config from the blob at file/active/client-<name>.conf,
//   - pushes config-changed when that blob is written (via the storage write observer).
//
// The server's goroutines self-terminate when ctx is canceled (its closeOnDone worker
// closes the listeners). It is independent of the outbound managed client
// (RunManagedClient): a hub can serve clients, be a managed client, or both.
func startManagedServer(ctx context.Context, store storage.Storage, hubConfig *zePlugin.HubConfig) *pluginserver.ManagedServer {
	if hubConfig == nil || !storage.IsBlobStorage(store) {
		return nil
	}

	var addrs []string
	var certificate string
	secrets := map[string]string{}
	for _, blk := range hubConfig.Servers {
		if len(blk.Clients) == 0 {
			continue // Not a managed-client-serving block.
		}
		addrs = append(addrs, blk.Address())
		maps.Copy(secrets, blk.Clients)
		// One certificate across every serving block. The extraction refuses two
		// blocks that name different ones (internal/component/config/loader_extract.go),
		// so the last non-empty name is the only name.
		if blk.Certificate != "" {
			certificate = blk.Certificate
		}
	}
	if len(addrs) == 0 {
		return nil // No managed clients configured on any server block.
	}

	// The daemon's certificate authority issues the listener's leaf when no
	// certificate is named. LoadOrGenerateRoot reads the root the hub already
	// generated at startup, so this is a load and never a second root.
	caRoot, caErr := zepki.LoadOrGenerateRoot(store)
	if caErr != nil {
		managedServerLog().Error("managed config server disabled: local certificate authority", "error", caErr)
		return nil
	}

	srv, err := pluginserver.NewManagedServer(pluginserver.ManagedServerConfig{
		Addrs:         addrs,
		ClientSecrets: secrets,
		ReadConfig: func(name string) ([]byte, error) {
			return store.ReadFile(pluginserver.ClientConfigKey(name))
		},
		Metrics: registry.GetMetricsRegistry(),
		// The resolver and the authority are both injected because pki imports
		// plugin/server. A named certificate wins and fails closed when it does
		// not resolve; with no name the authority issues a leaf whose root the
		// operator exports into each client's pki ca block.
		Certificate:         certificate,
		TLSMaterialResolver: zepki.ServerTLSMaterial,
		Authority:           caRoot,
	})
	if err != nil {
		managedServerLog().Error("managed config server disabled", "error", err)
		return nil
	}
	if startErr := srv.Start(ctx); startErr != nil {
		managedServerLog().Error("managed config server failed to start", "error", startErr)
		return nil
	}

	// Push config-changed when a client's config blob is written. NotifyConfigChanged
	// only enqueues (non-blocking), so the storage write path is never stalled.
	storage.SetWriteObserver(store, func(key string) {
		if name, ok := pluginserver.ClientNameFromConfigKey(key); ok {
			srv.NotifyConfigChanged(name)
		}
	})

	managedServerLog().Info("managed config server started", "addrs", addrs, "clients", len(secrets))
	return srv
}
