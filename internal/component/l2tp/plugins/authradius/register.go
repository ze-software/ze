// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS auth plugin lifecycle
// Related: l2tpauthradius.go -- atomic logger, Name constant
// Related: handler.go -- RADIUS auth handler
// Related: doctor.go -- RADIUS server reachability doctor check

package l2tpauthradius

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/plugins/authradius/yang"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	authInstance = newRADIUSAuth()
	acctInstance = newRADIUSAcct()
	eventBusMu   sync.Mutex
	storedBus    ze.EventBus
	activeCoA    *coaListener
)

// configRootL2TP is the YANG container this plugin reads.
const configRootL2TP = "l2tp"

func init() {
	reg := registry.Registration{
		Name:                    Name,
		Description:             "RADIUS authentication and accounting for L2TP PPP sessions",
		Features:                "yang",
		YANG:                    yang.ZeL2TPAuthRadiusConfYANG,
		ConfigRoots:             []string{configRootL2TP},
		InProcessConfigVerifier: verifyRadiusAuthConfig,
		RunEngine:               runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			bindRADIUSMetrics(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			acctInstance.subscribeEventBus(eb)
			eventBusMu.Lock()
			storedBus = eb
			eventBusMu.Unlock()
		},
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "l2tp-auth-radius-servers",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        710,
			Dependencies: []string{"radius-server"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-radius-unreachable"},
			Check:        checkRADIUSServers,
		}},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: registration failed: %v\n", Name, err)
		os.Exit(1)
	}
}

func verifyRadiusAuthConfig(sections []sdk.ConfigSection) error {
	for _, sec := range sections {
		if sec.Root != configRootL2TP {
			continue
		}
		if _, err := parseConfigFromJSON(sec.Data); err != nil && !errors.Is(err, errNoRADIUSConfig) {
			return err
		}
	}
	return nil
}

func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() {
		if err := p.Close(); err != nil {
			logger().Debug("plugin close error", "error", err)
		}
	}()

	p.OnConfigVerify(verifyRadiusAuthConfig)

	var pending *radiusConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != configRootL2TP {
				continue
			}
			cfg, err := parseConfigFromJSON(sec.Data)
			if errors.Is(err, errNoRADIUSConfig) {
				continue
			}
			if err != nil {
				return err
			}
			pending = cfg
		}
		if pending != nil {
			if applyErr := activateRadiusConfig(pending); applyErr != nil {
				return applyErr
			}
			pending = nil
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		if pending == nil {
			return nil
		}
		if err := activateRadiusConfig(pending); err != nil {
			return err
		}
		pending = nil
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		pending = nil
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootL2TP},
		VerifyBudget: 1,
		ApplyBudget:  1,
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		acctInstance.Stop()
		closeCoAListener()
		return 1
	}
	acctInstance.Stop()
	closeCoAListener()
	return 0
}

func activateRadiusConfig(cfg *radiusConfig) error {
	client, err := radius.NewClient(radius.ClientConfig{
		Servers:       cfg.Servers,
		Timeout:       cfg.Timeout,
		Retries:       cfg.Retries,
		SourceAddress: cfg.SourceAddress,
		Logger:        logger(),
	})
	if err != nil {
		return fmt.Errorf("%s: create client: %w", Name, err)
	}
	var primaryAddr string
	if len(cfg.Servers) > 0 {
		primaryAddr = cfg.Servers[0].Address
	}
	oldClient := authInstance.swapClient(client, cfg.NASIdentifier, primaryAddr, cfg.SourceAddress, cfg.NASPortIDFormat)
	authInstance.setExclusions(cfg.Exclusions)
	// Claim the auth slot HERE, not in init(). The slot holds one handler
	// (internal/component/l2tp/subscriber/handler_registry.go), and this plugin
	// and l2tp-auth-local both sit in the same binary, so an init()-time claim
	// made the owner a function of link order rather than of configuration. An
	// unconfigured RADIUS plugin then answered every CHAP Response with "no
	// RADIUS client", and the operator's local users were never consulted: a
	// PPPoE or L2TP subscriber authenticated against a local credential could
	// not come up at all. Claiming it once RADIUS has a client makes the
	// precedence "RADIUS when configured, local otherwise".
	l2tp.RegisterAuthHandler(func(req ppp.EventAuthRequest, respond l2tp.AuthRespondFunc) l2tp.AuthResult {
		return authInstance.handle(req, respond)
	})
	acctInstance.setClient(client, cfg.NASIdentifier, cfg.AcctInterval, primaryAddr, cfg.SourceAddress, cfg.NASPortIDFormat)
	acctInstance.setExclusions(cfg.Exclusions)
	if oldClient != nil {
		oldClient.Close() //nolint:errcheck // best-effort on replaced client
	}
	logger().Info("l2tp-auth-radius: configured",
		"servers", len(cfg.Servers), "timeout", cfg.Timeout)

	eventBusMu.Lock()
	bus := storedBus
	eventBusMu.Unlock()
	if activeCoA != nil {
		if closeErr := activeCoA.Close(); closeErr != nil {
			logger().Warn("l2tp-auth-radius: CoA listener close failed", "error", closeErr)
		}
		activeCoA = nil
	}
	if cfg.CoAPort > 0 && len(cfg.Servers) > 0 {
		startCoAListener(cfg, bus)
	}
	return nil
}

// startCoAListener starts the RFC 5176 CoA/Disconnect listener and records it in
// activeCoA. The caller holds the same lock every other writer of activeCoA
// holds.
func startCoAListener(cfg *radiusConfig, bus ze.EventBus) {
	// RFC 5176 Section 6.1: "A Dynamic Authorization Server MUST silently discard
	// Disconnect-Request or CoA-Request packets from untrusted sources." The
	// trusted sources are the configured RADIUS servers. When not one of their
	// addresses resolves there is no trusted client, so isAllowedSource (coa.go)
	// would answer no to every packet. Binding the port anyway would give the
	// operator a listener that can never serve anybody, with nothing above Debug
	// to say why, so the reason is named here instead.
	allowed := serverIPs(cfg.Servers)
	if len(allowed) == 0 {
		logger().Warn("l2tp-auth-radius: CoA listener not started: no RADIUS server address resolved, so no dynamic authorization client is trusted",
			"servers", len(cfg.Servers), "port", cfg.CoAPort)
		return
	}

	cl, coaErr := newCoAListener(coaListenerConfig{
		Port:                        cfg.CoAPort,
		Secrets:                     serverSecrets(cfg.Servers),
		DefaultSecret:               cfg.Servers[0].SharedKey,
		Bus:                         bus,
		AllowedSources:              allowed,
		RequireMessageAuthenticator: cfg.RequireMessageAuthenticator,
	})
	if coaErr != nil {
		logger().Warn("l2tp-auth-radius: CoA listener failed to start", "error", coaErr)
		return
	}
	activeCoA = cl
	logger().Info("l2tp-auth-radius: CoA listener started",
		"port", cfg.CoAPort,
		"trusted-sources", len(allowed),
		"require-message-authenticator", cfg.RequireMessageAuthenticator)
}

func closeCoAListener() {
	if activeCoA != nil {
		if err := activeCoA.Close(); err != nil {
			logger().Warn("l2tp-auth-radius: CoA listener close failed", "error", err)
		}
		activeCoA = nil
	}
}

// coaResolveTimeout is the TOTAL wall-clock budget for resolving every hostname
// RADIUS server address when building the CoA source-address allow list. A
// single deadline is shared across all server lookups (not per-server), so the
// worst case is bounded regardless of how many hostname servers are configured.
// It must stay under the plugin's declared ApplyBudget (1s, register.go Run) so
// a dead resolver can never push OnConfigApply past the transaction deadline
// (see startup-resilience FIX 2). A var (not const) so tests can shrink it.
var coaResolveTimeout = 750 * time.Millisecond

// lookupIPAddr resolves a hostname to IP addresses. It is a test seam over
// net.Resolver.LookupIPAddr so unit tests can exercise the bounded-timeout and
// skip-on-failure behavior without real DNS. Production code leaves it at the
// real resolver.
var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	var r net.Resolver
	return r.LookupIPAddr(ctx, host)
}

// serverIPs extracts the IP addresses from the configured RADIUS servers
// for CoA source address filtering. IP-literal addresses are used directly;
// hostname addresses are resolved under a single shared deadline
// (coaResolveTimeout) covering all of them, so total apply-path DNS work stays
// under the plugin's ApplyBudget even with several dead-resolver hostnames.
func serverIPs(servers []radius.Server) []net.IP {
	ctx, cancel := context.WithTimeout(context.Background(), coaResolveTimeout)
	defer cancel()
	var ips []net.IP
	for _, srv := range servers {
		host, _, err := net.SplitHostPort(srv.Address)
		if err != nil {
			host = srv.Address
		}
		if ip := net.ParseIP(host); ip != nil {
			ips = append(ips, ip)
			continue
		}
		ips = append(ips, resolveCoAHost(ctx, host)...)
	}
	return ips
}

// resolveCoAHost resolves a hostname RADIUS server address to IPs for the CoA
// source-address allow list under the caller's shared deadline. An unresolved
// (or timed-out) hostname is logged and skipped, so the allow list degrades to
// the resolvable subset and apply is never blocked on a dead resolver.
//
// That degradation is fail-closed at every size INCLUDING zero, which is a
// property of isAllowedSource (coa.go) rather than of this function. Until
// 2026-08-31 this comment claimed fail-closed while the guard read an empty
// list as "allow every source", so an outage that left no hostname resolvable
// opened the CoA port instead of shutting it. The claim is true now because
// the guard was corrected, not because the wording was.
func resolveCoAHost(ctx context.Context, host string) []net.IP {
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		logger().Warn("l2tp-auth-radius: CoA server hostname unresolved, skipping from source filter",
			"host", host, "error", err)
		return nil
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
	}
	return ips
}

// serverSecrets builds a map of server IP -> shared secret for per-source
// CoA/DM authenticator verification.
func serverSecrets(servers []radius.Server) map[string][]byte {
	secrets := make(map[string][]byte, len(servers))
	for _, srv := range servers {
		host, _, err := net.SplitHostPort(srv.Address)
		if err != nil {
			host = srv.Address
		}
		if ip := net.ParseIP(host); ip != nil {
			secrets[ip.String()] = srv.SharedKey
		}
	}
	return secrets
}

// parseConfigFromJSON parses YANG-delivered JSON config.
// JSON shape: {"auth":{"radius":{"server":[{...}]}}}.
func parseConfigFromJSON(data string) (*radiusConfig, error) {
	if data == "" {
		return nil, errNoRADIUSConfig
	}

	var tree map[string]any
	if err := json.Unmarshal([]byte(data), &tree); err != nil {
		return nil, fmt.Errorf("%s: invalid config JSON: %w", Name, err)
	}

	return parseConfigFromTree(tree)
}
