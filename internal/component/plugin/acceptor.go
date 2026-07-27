// Design: docs/architecture/api/process-protocol.md — TLS transport for external plugins
// Related: manager/manager.go — Manager.ensureAcceptor (engine/YANG-daemon path) builds its acceptor here
// Related: ../hub/hub.go — Orchestrator.ensureAcceptor (hub orchestrator path) builds its acceptor here
// Related: server/subsystem.go — SubsystemManager/SubsystemHandler receive the acceptor built here

package plugin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

// NewHubAcceptor creates and STARTS a TLS acceptor for external plugin
// connect-back, and is the single implementation of that lifecycle. Both
// orchestrators that fork external plugins use it: the engine's PluginManager
// (bgp/YANG daemon config) and the hub Orchestrator's SubsystemManager (pure
// `plugin { external ... }` config). A second copy is what let the hub path
// ship with no acceptor at all.
//
// cfg may be nil or carry no server block, in which case a single loopback
// server on an OS-assigned port with a fresh random secret is generated: an
// external plugin authenticates with the per-plugin token the engine hands it
// through ZE_PLUGIN_HUB_TOKEN, so no operator-visible secret is required.
// When cfg does declare servers, the FIRST block is used.
//
// Caller obligations: the returned acceptor is already accepting connections.
// The caller owns it and MUST call Stop on it during shutdown, or the listener
// and its accept goroutine leak.
func NewHubAcceptor(cfg *HubConfig) (*ipc.PluginAcceptor, error) {
	server, err := hubAcceptorServer(cfg)
	if err != nil {
		return nil, err
	}

	cert, err := ipc.GenerateSelfSignedCert()
	if err != nil {
		return nil, fmt.Errorf("generate TLS cert: %w", err)
	}

	listeners, err := ipc.StartListeners([]string{server.Address()}, cert)
	if err != nil {
		return nil, fmt.Errorf("start TLS listeners on %s: %w", server.Address(), err)
	}

	acceptor := ipc.NewPluginAcceptor(listeners[0], server.Secret, ipc.CertFingerprint(cert))

	// Wire per-client secrets if the server block has any.
	if len(server.Clients) > 0 {
		clients := server.Clients // capture for closure
		acceptor.SetSecretLookup(func(name string) (string, bool) {
			s, ok := clients[name]
			return s, ok
		})
	}

	acceptor.Start()

	// Close extra listeners (acceptor owns the first one).
	for _, ln := range listeners[1:] {
		ln.Close() //nolint:errcheck,gosec // extra listeners not used yet
	}

	return acceptor, nil
}

// hubAcceptorServer picks the server block the acceptor listens on: the first
// configured one, or an auto-generated loopback block with a random secret when
// the config declares none.
func hubAcceptorServer(cfg *HubConfig) (HubServerConfig, error) {
	if cfg != nil && len(cfg.Servers) > 0 {
		return cfg.Servers[0], nil
	}

	var tokenBytes [32]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return HubServerConfig{}, fmt.Errorf("generate hub token: %w", err)
	}
	return HubServerConfig{
		Name:   "auto",
		Host:   "127.0.0.1",
		Port:   0,
		Secret: hex.EncodeToString(tokenBytes[:]),
	}, nil
}
