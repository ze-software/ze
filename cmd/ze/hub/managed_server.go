// Design: docs/architecture/fleet-config.md -- hub managed-config server startup wiring

package hub

import (
	"context"
	"errors"

	zeconfig "github.com/ze-software/ze/internal/component/config"

	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
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
		if problem, found := errors.AsType[*opererr.Error](err); found {
			return nil, problem
		}
		return nil, opererr.Wrap(err, "the managed listener configuration is invalid", "the hub cannot prepare this candidate for reload", "edit bus hub")
	}
	if srv == nil {
		return nil, nil // No managed store was opened by this daemon.
	}
	listens, certificate := managedListeners(&cfg)
	return srv.PrepareManagedListenersWithHistoryAndAlarms(listens, certificate, cfg.History, cfg.Alarms)
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
		LoadAuthority: func() (zePlugin.Authority, error) {
			return zepki.LoadOrGenerateRoot(store)
		},
		// The plugin acceptor binds the first server block (hubAcceptorServer),
		// so a managed address colliding with it is the collision the local
		// hub-managed-listener check reports, not an unbound listener.
		Acceptor: acceptor,
		History:  hubConfig.History,
		Alarms:   hubConfig.Alarms,
	})
	// refuse-by-default: guards runYANGConfig -- a construction failure leaves no server to start, publish or pass to the fleet commands
	if err != nil {
		problem, found := errors.AsType[*opererr.Error](err)
		if !found {
			problem = opererr.Wrap(err, "the managed config server could not be constructed", "the hub cannot serve device configurations", "ze show doctor")
		}
		managedServerStartupFailed(problem)
		return nil
	}
	if startErr := srv.Start(ctx); startErr != nil {
		managedServerStartupFailed(opererr.Wrap(startErr, "the managed config server could not load its registry", "storage must be readable before the hub starts serving devices", "ze show data check"))
		return nil
	}

	managedServerLog().Info("managed config server started", "listeners", len(listeners))
	return srv
}

// managedServerStartupFailed reports startup failure even though Start could
// not publish the server's runtime snapshot.
func managedServerStartupFailed(problem *opererr.Error) {
	pluginserver.RecordManagedStartupFailure(problem)
	managedServerLog().Error(problem.Problem(), "code", pluginserver.CodeDoctorManagedServerUnavailable,
		"error", problem.Unwrap(), "why", problem.Why(), "fix", problem.Fix())
}
