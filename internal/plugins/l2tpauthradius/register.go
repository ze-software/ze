// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS auth plugin lifecycle
// Related: l2tpauthradius.go -- atomic logger, Name constant
// Related: handler.go -- RADIUS auth handler

package l2tpauthradius

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	schema "codeberg.org/thomas-mangin/ze/internal/plugins/l2tpauthradius/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	authInstance = newRADIUSAuth()
	acctInstance = newRADIUSAcct()
)

func init() {
	l2tp.RegisterAuthHandler(func(req ppp.EventAuthRequest, respond l2tp.AuthRespondFunc) l2tp.AuthResult {
		return authInstance.handle(req, respond)
	})

	reg := registry.Registration{
		Name:        Name,
		Description: "RADIUS authentication and accounting for L2TP PPP sessions",
		Features:    "yang",
		YANG:        schema.ZeL2TPAuthRadiusConfYANG,
		ConfigRoots: []string{"l2tp"},
		RunEngine:   runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				acctInstance.SubscribeEventBus(e)
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

func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() {
		if err := p.Close(); err != nil {
			logger().Debug("plugin close error", "error", err)
		}
	}()

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != "l2tp" {
				continue
			}
			if _, err := parseConfigFromJSON(sec.Data); err != nil && !errors.Is(err, errNoRADIUSConfig) {
				return err
			}
		}
		return nil
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
			Servers: pending.Servers,
			Timeout: pending.Timeout,
			Retries: pending.Retries,
			Logger:  logger(),
		})
		if err != nil {
			return fmt.Errorf("%s: create client: %w", Name, err)
		}
		oldClient := authInstance.swapClient(client, pending.NASIdentifier)
		acctInstance.setClient(client, pending.NASIdentifier, pending.AcctInterval)
		if oldClient != nil {
			oldClient.Close() //nolint:errcheck // best-effort on replaced client
		}
		logger().Info("l2tp-auth-radius: configured",
			"servers", len(pending.Servers), "timeout", pending.Timeout)
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
		return 1
	}
	return 0
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
