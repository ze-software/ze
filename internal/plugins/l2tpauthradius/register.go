// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS auth plugin lifecycle
// Related: l2tpauthradius.go -- atomic logger, Name constant
// Related: handler.go -- RADIUS auth handler

package l2tpauthradius

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	schema "codeberg.org/thomas-mangin/ze/internal/plugins/l2tpauthradius/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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
		YANG:                    schema.ZeL2TPAuthRadiusConfYANG,
		ConfigRoots:             []string{"l2tp"},
		InProcessConfigVerifier: verifyRadiusAuthConfig,
		RunEngine:               runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				bindRADIUSMetrics(r)
			}
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				acctInstance.SubscribeEventBus(e)
				eventBusMu.Lock()
				storedBus = e
				eventBusMu.Unlock()
			}
		},
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

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		return verifyRadiusAuthConfig(sections)
	})

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
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		if pending == nil {
			return nil
		}
		client, err := radius.NewClient(radius.ClientConfig{
			Servers:       pending.Servers,
			Timeout:       pending.Timeout,
			Retries:       pending.Retries,
			SourceAddress: pending.SourceAddress,
			Logger:        logger(),
		})
		if err != nil {
			return fmt.Errorf("%s: create client: %w", Name, err)
		}
		var primaryAddr string
		if len(pending.Servers) > 0 {
			primaryAddr = pending.Servers[0].Address
		}
		oldClient := authInstance.swapClient(client, pending.NASIdentifier, primaryAddr, pending.SourceAddress)
		acctInstance.setClient(client, pending.NASIdentifier, pending.AcctInterval, primaryAddr, pending.SourceAddress)
		if oldClient != nil {
			oldClient.Close() //nolint:errcheck // best-effort on replaced client
		}
		logger().Info("l2tp-auth-radius: configured",
			"servers", len(pending.Servers), "timeout", pending.Timeout)

		// Start or restart CoA/DM listener if configured.
		eventBusMu.Lock()
		bus := storedBus
		eventBusMu.Unlock()
		if activeCoA != nil {
			if closeErr := activeCoA.Close(); closeErr != nil {
				logger().Warn("l2tp-auth-radius: CoA listener close failed", "error", closeErr)
			}
			activeCoA = nil
		}
		if pending.CoAPort > 0 && len(pending.Servers) > 0 {
			allowed := serverIPs(pending.Servers)
			secrets := serverSecrets(pending.Servers)
			cl, coaErr := newCoAListener(pending.CoAPort, secrets, pending.Servers[0].SharedKey, bus, allowed)
			if coaErr != nil {
				logger().Warn("l2tp-auth-radius: CoA listener failed to start", "error", coaErr)
			} else {
				activeCoA = cl
				logger().Info("l2tp-auth-radius: CoA listener started", "port", pending.CoAPort)
			}
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

func closeCoAListener() {
	if activeCoA != nil {
		if err := activeCoA.Close(); err != nil {
			logger().Warn("l2tp-auth-radius: CoA listener close failed", "error", err)
		}
		activeCoA = nil
	}
}

// serverIPs extracts the IP addresses from the configured RADIUS servers
// for CoA source address filtering.
func serverIPs(servers []radius.Server) []net.IP {
	var ips []net.IP
	for _, srv := range servers {
		host, _, err := net.SplitHostPort(srv.Address)
		if err != nil {
			host = srv.Address
		}
		if ip := net.ParseIP(host); ip != nil {
			ips = append(ips, ip)
		} else {
			var resolver net.Resolver
			addrs, resolveErr := resolver.LookupIPAddr(context.Background(), host)
			if resolveErr == nil {
				for _, a := range addrs {
					ips = append(ips, a.IP)
				}
			}
		}
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
