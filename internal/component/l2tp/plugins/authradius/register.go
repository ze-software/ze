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

func init() {
	l2tp.RegisterAuthHandler(func(req ppp.EventAuthRequest, respond l2tp.AuthRespondFunc) l2tp.AuthResult {
		return authInstance.handle(req, respond)
	})

	reg := registry.Registration{
		Name:                    Name,
		Description:             "RADIUS authentication and accounting for L2TP PPP sessions",
		Features:                "yang",
		YANG:                    yang.ZeL2TPAuthRadiusConfYANG,
		ConfigRoots:             []string{"l2tp"},
		InProcessConfigVerifier: verifyRadiusAuthConfig,
		RunEngine:               runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			bindRADIUSMetrics(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			acctInstance.SubscribeEventBus(eb)
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
		if sec.Root != "l2tp" {
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
			if sec.Root != "l2tp" {
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
		WantsConfig:  []string{"l2tp"},
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
	acctInstance.setClient(client, cfg.NASIdentifier, cfg.AcctInterval, primaryAddr, cfg.SourceAddress, cfg.NASPortIDFormat)
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
		allowed := serverIPs(cfg.Servers)
		secrets := serverSecrets(cfg.Servers)
		cl, coaErr := newCoAListener(cfg.CoAPort, secrets, cfg.Servers[0].SharedKey, bus, allowed)
		if coaErr != nil {
			logger().Warn("l2tp-auth-radius: CoA listener failed to start", "error", coaErr)
		} else {
			activeCoA = cl
			logger().Info("l2tp-auth-radius: CoA listener started", "port", cfg.CoAPort)
		}
	}
	return nil
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
// (or timed-out) hostname is logged and skipped: the allow list degrades to the
// resolvable subset (fail-closed for CoA source filtering) and apply is never
// blocked on a dead resolver.
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
