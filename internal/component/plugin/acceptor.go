// Design: docs/architecture/api/process-protocol.md — TLS transport for external plugins
// Related: manager/manager.go — Manager.ensureAcceptor, the only caller, builds its acceptor here
// Related: server/subsystem.go — SubsystemManager/SubsystemHandler take an acceptor through SetAcceptor

package plugin

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/clock"
)

// Authority is the daemon's certificate authority as an internal listener uses
// it: it issues a short-lived leaf for one of Ze's own components, and it
// publishes the root a peer validates that leaf against. commonName names the
// component and hosts are the SANs the peer verifies.
//
// The two halves are ONE value because they must agree. A leaf and a root that
// arrive through separate parameters can name different roots, and the peer
// that meets the disagreement is a process away from the caller that caused it.
//
// It is INJECTED rather than imported. internal/component/pki already reaches
// this package (pki/show.go -> plugin/server -> plugin/ipc), so importing pki
// here would close a cycle. *pki.Root is the production implementation and
// cmd/ze/hub is where the two meet. This is the shape
// ManagedServerConfig.TLSMaterialResolver already uses for the same reason.
type Authority interface {
	IssueLeaf(commonName string, hosts []string) (tls.Certificate, error)
	CertificatePEM() []byte
}

// hubLeafCommonName names the hub acceptor in the subject of the certificate it
// serves. It is not a hostname and is never matched against one; the SANs are.
const hubLeafCommonName = "ze-plugin-hub"

// loopbackHost is where the auto-generated hub binds and what every co-located
// plugin dials, so it is both the listen address and a SAN on the served leaf.
const loopbackHost = "127.0.0.1"

// errHubAcceptorNoAuthority refuses an acceptor that has no way to obtain a
// certificate. There is no self-signed fallback: a certificate nothing issued
// is one no peer can validate and no operator can rotate, which is the failure
// the certificate authority exists to replace.
//
// errHubAcceptorNoRoot refuses one whose authority publishes no root. Every
// plugin the acceptor serves takes that root as its only trust anchor, so an
// empty one refuses every connect-back, one process away and with no cause
// named there.
var (
	errHubAcceptorNoAuthority = errors.New("plugin: hub acceptor requires a certificate authority")
	errHubAcceptorNoRoot      = errors.New("plugin: hub acceptor certificate authority published no root")
)

// NewHubAcceptor creates and STARTS a TLS acceptor for external plugin
// connect-back, and is the single implementation of that lifecycle. Every
// orchestrator that forks external plugins takes its acceptor from here:
// Manager.ensureAcceptor builds one, and SubsystemManager.SetAcceptor receives
// one. A second copy is what let the hub path ship with no acceptor at all.
//
// cfg may be nil or carry no server block, in which case a single loopback
// server on an OS-assigned port with a fresh random secret is generated: an
// external plugin authenticates with the per-plugin token the engine hands it
// through ZE_PLUGIN_HUB_TOKEN, so no operator-visible secret is required.
// When cfg does declare servers, the FIRST block is used.
//
// ca supplies the certificate the acceptor serves and the root each plugin
// validates it against. It MUST NOT be nil: an acceptor with no certificate
// authority is an error, never a self-signed fallback. The served leaf is
// REISSUED from ca as it ages, so an acceptor that runs longer than one leaf
// lives keeps presenting a valid certificate (see ServingLeaf).
//
// clk is the clock that decides when the leaf is due. Pass clock.RealClock{}
// outside a test.
//
// Caller obligations: the returned acceptor is already accepting connections.
// The caller owns it and MUST call Stop on it during shutdown, or the listener
// and its accept goroutine leak.
func NewHubAcceptor(cfg *HubConfig, ca Authority, clk clock.Clock) (*ipc.PluginAcceptor, error) {
	if ca == nil {
		return nil, errHubAcceptorNoAuthority
	}

	rootPEM := ca.CertificatePEM()
	if len(rootPEM) == 0 {
		return nil, errHubAcceptorNoRoot
	}

	server, err := hubAcceptorServer(cfg)
	if err != nil {
		return nil, err
	}

	leaf, err := NewServingLeaf(ca, hubLeafCommonName, hubLeafHosts(server), clk)
	if err != nil {
		return nil, fmt.Errorf("hub acceptor: %w", err)
	}

	listeners, err := ipc.StartListeners([]string{server.Address()}, leaf.Certificate)
	if err != nil {
		return nil, fmt.Errorf("start TLS listeners on %s: %w", server.Address(), err)
	}

	acceptor := ipc.NewPluginAcceptor(listeners[0], server.Secret, rootPEM)

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
		Host:   loopbackHost,
		Port:   0,
		Secret: hex.EncodeToString(tokenBytes[:]),
	}, nil
}

// hubLeafHosts returns the SANs the hub's certificate needs: the loopback
// address every co-located plugin dials, plus the configured listen address
// when it names one specific host. An unspecified address (0.0.0.0 or ::) names
// no host a peer can verify, so it adds nothing.
func hubLeafHosts(server HubServerConfig) []string {
	hosts := make([]string, 0, 2)
	hosts = append(hosts, loopbackHost)
	if server.Host == "" || server.Host == loopbackHost {
		return hosts
	}
	if ip := net.ParseIP(server.Host); ip != nil && ip.IsUnspecified() {
		return hosts
	}
	return append(hosts, server.Host)
}
