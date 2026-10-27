// Design: docs/architecture/dns/server-harness.md -- the LAN resolver, the harness's recursive consumer
// RFC: rfc/short/rfc1035.md -- DNS message structure and transport

// Package dnsresolver is the LAN DNS resolver: `service dns-resolver`
// answers recursive queries from the prefixes the operator allows, on the
// listeners the operator names, through dnsserver.Recursive.
package dnsresolver

import (
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/dnsserver"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	resolveryang "github.com/ze-software/ze/internal/plugins/dnsresolver/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// listenerService is the name config.DiscoverListenerServices derives from
// the schema path service/dns-resolver/listener.
const listenerService = "service-dns-resolver-listener"

var loggerPtr atomic.Pointer[slog.Logger]

// activeResolver is the running plugin's resolver, which the show and clear
// handlers read; nil while the plugin is not running.
var activeResolver atomic.Pointer[resolver]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	reg := registry.Registration{
		Name:                    "dnsresolver",
		Description:             "LAN DNS resolver: answers recursive queries from allowed prefixes by forwarding to upstream resolvers (RFC 1035, RFC 2181)",
		Features:                "yang",
		YANG:                    resolveryang.ZeDNSResolverConfYANG,
		ConfigRoots:             []string{configRootService},
		InProcessConfigVerifier: verifyResolverConfig,
		RunEngine:               runResolverPlugin,
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	reg.ConfigureEngineLogger = func(loggerName string) {
		if l := slogutil.Logger(loggerName); l != nil {
			loggerPtr.Store(l)
		}
	}
	reg.ConfigureMetrics = func(r metrics.Registry) {
		// The reply write happens inside the shared harness, which owns its
		// write-failure counter; without this it counts against a no-op
		// registry.
		dnsserver.SetMetricsRegistry(r)
		setMetricsRegistry(r)
	}

	// dnsserver.Manager binds UDP and TCP on every endpoint, while the
	// zt:listener shape says TCP alone, so ze doctor and the conflict check
	// are told both.
	config.RegisterListenerProtocols(listenerService, config.ProtocolUDP, config.ProtocolTCP)

	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{
		WireMethod: "ze-dnsresolver:show-dns-resolver",
		Handler:    handleShowDNSResolver,
	})
	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{
		WireMethod: "ze-dnsresolver:clear-dns-resolver-cache",
		Handler:    handleClearDNSResolverCache,
	})

	if err := registry.Register(reg); err != nil {
		_ = textbuf.New().Str("dnsresolver: registration failed: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // exiting; stderr is the last channel
		os.Exit(1)
	}
}

// verifyResolverConfig is the offline verifier: it parses and validates the
// committed config without binding anything. It is the parse the reload's
// verify runs (parseResolverSections, through sdk.ConfigTx), so the two
// cannot disagree.
func verifyResolverConfig(sections []sdk.ConfigSection) error {
	_, _, err := parseResolverSections(sections)
	return err
}

// runResolverPlugin is the engine entry point: it registers the config
// transaction, which binds the listeners each committed config names, and
// serves until the plugin is stopped.
func runResolverPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("dnsresolver plugin starting")

	p := sdk.NewWithConn("dnsresolver", conn)
	defer func() { _ = p.Close() }()

	res := newResolver(time.Now)
	res.start()
	defer res.stop()
	activeResolver.Store(res)
	defer activeResolver.Store(nil)

	srv := newResolverServer(log, res)
	inst := &resolverInstaller{srv: srv, res: res, log: log}
	registerResolverConfigTx(p, sdk.NewConfigTx(defaultConfig(), parseResolverSections, inst.install), inst)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRootService},
		VerifyBudget: 2,
		ApplyBudget:  5,
	}); err != nil {
		log.Error("dnsresolver plugin failed", "error", err)
		srv.stop()
		return 1
	}

	srv.stop()
	log.Info("dnsresolver plugin stopped")
	return 0
}
