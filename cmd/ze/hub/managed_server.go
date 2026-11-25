// Design: docs/architecture/fleet-config.md -- hub managed-config server startup wiring

package hub

import (
	"context"
	"fmt"

	zeconfig "github.com/ze-software/ze/internal/component/config"

	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/slogutil"
)

var managedServerLog = slogutil.LazyLogger("hub.managed-server")

// managedListeners reads the serving blocks and their common certificate for
// both startup and reload. Extraction validates the shared certificate name.
func managedListeners(hub *zePlugin.HubConfig) ([]pluginserver.ManagedListener, string) {
	var listeners []pluginserver.ManagedListener
	var certificate string
	for _, block := range hub.Servers {
		if len(block.Clients) == 0 {
			continue
		}
		listeners = append(listeners, pluginserver.ManagedListener{
			Block: block.Name, Addr: block.Address(), Clients: block.Clients,
			ClientLabels: block.ClientLabels, Certificate: block.Certificate,
		})
		if block.Certificate != "" {
			certificate = block.Certificate
		}
	}
	return listeners, certificate
}

// prepareManagedReload extracts the candidate's serving declarations before any
// consumer applies them. The caller has already installed the candidate PKI store.
func prepareManagedReload(tree *zeconfig.Tree, srv *pluginserver.ManagedServer) (*pluginserver.ManagedListenerPlan, error) {
	if tree == nil {
		return nil, nil // A reload with no parsed configuration supplies no listener declarations.
	}
	cfg, err := zeconfig.ExtractHubConfig(tree)
	if err != nil {
		return nil, fmt.Errorf("managed listener configuration: %w", err)
	}
	if srv == nil {
		return nil, nil // No managed store was opened by this daemon.
	}
	listens, certificate := managedListeners(&cfg)
	return srv.PrepareManagedListeners(listens, certificate)
}

// startManagedServer starts the registry and its workers on the daemon's store.
// A hub with no managed clients starts with no bound sockets. An accepted reload
// can add its first serving block.
// The caller MUST call Stop before releasing store. Cancellation closes the
// listeners, but Stop also joins requests and notifications already reading it.
// It is independent of the outbound managed client
// (RunManagedClient): a hub can serve clients, be a managed client, or both.
func startManagedServer(ctx context.Context, store storage.Storage, hubConfig *zePlugin.HubConfig) *pluginserver.ManagedServer {
	if hubConfig == nil || store == nil {
		return nil
	}

	listeners, certificate := managedListeners(hubConfig)
	var acceptor zePlugin.HubServerConfig
	if len(hubConfig.Servers) != 0 {
		acceptor = hubConfig.Servers[0]
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
		Listeners: listeners,
		ReadConfig: func(name string) ([]byte, error) {
			return store.ReadFile(pluginserver.ClientConfigKey(name))
		},
		// The same store keeps the copy each holding client sends of its
		// configuration, beside that client's configuration.
		Store:   store,
		Metrics: registry.GetMetricsRegistry(),
		// The resolver and the authority are both injected because pki imports
		// plugin/server. A named certificate wins and fails closed when it does
		// not resolve; with no name the authority issues a leaf whose root the
		// operator exports into each client's pki ca block.
		Certificate:         certificate,
		TLSMaterialResolver: zepki.ServerTLSMaterial,
		Authority:           caRoot,
		// The plugin acceptor binds the first server block (hubAcceptorServer),
		// so a managed address colliding with it is the collision the local
		// hub-managed-listener check reports, not an unbound listener.
		Acceptor: acceptor,
	})
	// refuse-by-default: guards runYANGConfig -- a construction failure leaves no server to start, publish or pass to the fleet commands
	if err != nil {
		managedServerLog().Error("managed config server disabled", "error", err)
		return nil
	}
	// Subscribe before Start reads the stored records. The callback schedules work
	// only, so it can also run while the configuration writer holds its lock.
	store.SetWriteObserver(func(key string) {
		srv.ObserveDeviceRecordKey(key)
		if name, ok := pluginserver.ClientNameFromConfigKey(key); ok {
			srv.MarkManagedConfigChanged(name)
			srv.NotifyConfigChanged(name)
		}
	})
	if startErr := srv.Start(ctx); startErr != nil {
		store.SetWriteObserver(nil)
		managedServerLog().Error("managed config server failed to start", "error", startErr)
		return nil
	}

	managedServerLog().Info("managed config server started", "listeners", len(listeners))
	return srv
}
